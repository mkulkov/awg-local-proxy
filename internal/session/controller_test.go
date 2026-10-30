package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
)

type fakeBackend struct {
	connect    func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error)
	status     func(context.Context) (coreclient.Status, error)
	closed     atomic.Bool
	onClose    func()
	closeError func() error
}

func (f *fakeBackend) Hello(context.Context) (coreclient.Hello, error) {
	return coreclient.Hello{}, nil
}
func (f *fakeBackend) Connect(context.Context) (coreclient.Status, error) {
	return coreclient.Status{}, nil
}
func (f *fakeBackend) ConnectProfile(ctx context.Context, r coreclient.ConnectRequest) (coreclient.Status, error) {
	return f.connect(ctx, r)
}
func (f *fakeBackend) Status(ctx context.Context) (coreclient.Status, error) {
	if f.status != nil {
		return f.status(ctx)
	}
	return connected(), nil
}
func (f *fakeBackend) CheckTunnel(ctx context.Context) (coreclient.Status, error) {
	return f.Status(ctx)
}
func (f *fakeBackend) Disconnect(context.Context) (coreclient.Status, error) {
	return coreclient.Status{State: coreclient.Stopped}, nil
}
func (f *fakeBackend) Close() error {
	if f.closeError != nil {
		if err := f.closeError(); err != nil {
			return err
		}
	}
	if f.closed.CompareAndSwap(false, true) && f.onClose != nil {
		f.onClose()
	}
	return nil
}

func TestUnconfirmedCloseBlocksRetryAndNewStart(t *testing.T) {
	var made atomic.Int32
	var permit atomic.Bool
	factory := func(context.Context) (Backend, error) {
		n := made.Add(1)
		f := &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) {
			return coreclient.Status{}, &coreclient.TransportError{}
		}}
		if n == 1 {
			f.closeError = func() error {
				if !permit.Load() {
					return errors.New("synthetic process still alive")
				}
				return nil
			}
		}
		return f, nil
	}
	c := New(factory, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
	defer c.Close()
	c.wait = func(context.Context, time.Duration) error { return nil }
	_, _ = c.Start(selection(true))
	waitState(t, c, coreclient.Error)
	if made.Load() != 1 {
		t.Fatal("retried before confirming previous child termination")
	}
	if _, err := c.Start(selection(true)); err == nil || made.Load() != 1 {
		t.Fatal("new Start bypassed failed Close")
	}
	permit.Store(true)
	if _, err := c.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCancelledLateFactoryRetainsUnconfirmedChild(t *testing.T) {
	entered := make(chan struct{})
	var made atomic.Int32
	var permit atomic.Bool
	f := &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) {
		t.Error("cancelled factory child used")
		return connected(), nil
	}, closeError: func() error {
		if !permit.Load() {
			return errors.New("synthetic still alive")
		}
		return nil
	}}
	c := New(func(ctx context.Context) (Backend, error) { made.Add(1); close(entered); <-ctx.Done(); return f, nil }, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
	defer c.Close()
	_, _ = c.Start(selection(true))
	<-entered
	if _, err := c.Disconnect(context.Background()); err == nil {
		t.Fatal("lost late unconfirmed child")
	}
	if _, err := c.Start(selection(true)); err == nil || made.Load() != 1 {
		t.Fatal("created parallel child after cancelled factory")
	}
	permit.Store(true)
	if _, err := c.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func connected() coreclient.Status {
	now := time.Now()
	return coreclient.Status{State: coreclient.Connected, ListenerReady: true, TunnelVerified: true, HandshakeAt: &now, SocksAddress: "127.0.0.1:1080"}
}
func selection(retry bool) Selection {
	return Selection{ProfileID: "synthetic", SocksAddress: "127.0.0.1:1080", ProbeURL: "https://example.invalid", Reconnect: retry}
}
func waitState(t *testing.T, c *Controller, want coreclient.State) coreclient.Status {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s, e := c.Status(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		if s.State == want {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("state did not reach %s", want)
	return coreclient.Status{}
}

func TestRetryClosesPreviousCoreAndClearsEachSecret(t *testing.T) {
	var calls, live, maxLive atomic.Int32
	var mu sync.Mutex
	var buffers [][]byte
	factory := func(context.Context) (Backend, error) {
		n := live.Add(1)
		for {
			m := maxLive.Load()
			if n <= m || maxLive.CompareAndSwap(m, n) {
				break
			}
		}
		f := &fakeBackend{onClose: func() { live.Add(-1) }}
		f.connect = func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) {
			if calls.Add(1) == 1 {
				return coreclient.Status{}, &coreclient.CoreError{Code: "connect_failed"}
			}
			return connected(), nil
		}
		return f, nil
	}
	load := func(string) ([]byte, error) {
		b := []byte("SYNTHETIC")
		mu.Lock()
		buffers = append(buffers, b)
		mu.Unlock()
		return b, nil
	}
	c := New(factory, load, nil)
	defer c.Close()
	c.wait = func(context.Context, time.Duration) error { return nil }
	if _, e := c.Start(selection(true)); e != nil {
		t.Fatal(e)
	}
	waitState(t, c, coreclient.Connected)
	if calls.Load() != 2 || maxLive.Load() != 1 {
		t.Fatalf("calls=%d max children=%d", calls.Load(), maxLive.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(buffers) != 2 {
		t.Fatal("secret not reloaded")
	}
	for _, b := range buffers {
		for _, x := range b {
			if x != 0 {
				t.Fatal("secret retained between attempts")
			}
		}
	}
}

func TestPermanentAndOptOutErrorsDoNotRetry(t *testing.T) {
	for _, tt := range []struct {
		name  string
		retry bool
		err   error
	}{{"invalid", true, &coreclient.CoreError{Code: "invalid_config"}}, {"port", true, &coreclient.CoreError{Code: "port_unavailable"}}, {"unknown", true, errors.New("unexpected")}, {"optout", false, &coreclient.CoreError{Code: "connect_failed"}}} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			c := New(func(context.Context) (Backend, error) {
				return &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) {
					calls.Add(1)
					return coreclient.Status{}, tt.err
				}}, nil
			}, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
			defer c.Close()
			c.wait = func(context.Context, time.Duration) error { return nil }
			_, _ = c.Start(selection(tt.retry))
			waitState(t, c, coreclient.Error)
			if calls.Load() != 1 {
				t.Fatalf("retried permanent/optout error %d times", calls.Load())
			}
		})
	}
}

func TestRetryBudgetAndDisconnectCancelBackoff(t *testing.T) {
	var calls atomic.Int32
	factory := func(context.Context) (Backend, error) {
		return &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) {
			calls.Add(1)
			return coreclient.Status{}, &coreclient.TransportError{}
		}}, nil
	}
	load := func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }
	c := New(factory, load, nil)
	c.wait = func(context.Context, time.Duration) error { return nil }
	_, _ = c.Start(selection(true))
	waitState(t, c, coreclient.Error)
	if calls.Load() != 5 {
		t.Fatalf("attempt limit=%d", calls.Load())
	}
	_ = c.Close()
	calls.Store(0)
	c = New(factory, load, nil)
	defer c.Close()
	entered := make(chan struct{})
	var once sync.Once
	c.wait = func(ctx context.Context, _ time.Duration) error {
		once.Do(func() { close(entered) })
		<-ctx.Done()
		return ctx.Err()
	}
	_, _ = c.Start(selection(true))
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("did not reach backoff")
	}
	if _, e := c.Disconnect(context.Background()); e != nil {
		t.Fatal(e)
	}
	waitState(t, c, coreclient.Stopped)
	if calls.Load() != 1 {
		t.Fatal("created child after disconnect")
	}
}

func TestTransportLossRecoversButDegradedDoesNot(t *testing.T) {
	var made atomic.Int32
	factory := func(context.Context) (Backend, error) {
		n := made.Add(1)
		f := &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) { return connected(), nil }}
		if n == 1 {
			f.status = func(context.Context) (coreclient.Status, error) {
				return coreclient.Status{}, &coreclient.TransportError{}
			}
		} else {
			f.status = func(context.Context) (coreclient.Status, error) {
				return coreclient.Status{State: coreclient.Degraded, ListenerReady: true}, nil
			}
		}
		return f, nil
	}
	c := New(factory, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
	defer c.Close()
	c.wait = func(context.Context, time.Duration) error { return nil }
	_, _ = c.Start(selection(true))
	waitState(t, c, coreclient.Degraded)
	for i := 0; i < 4; i++ {
		_, _ = c.Status(context.Background())
	}
	if made.Load() != 2 {
		t.Fatalf("degraded destination triggered reconnect: %d", made.Load())
	}
}

func TestDisconnectRejectsLateSuccessfulConnection(t *testing.T) {
	entered := make(chan struct{})
	f := &fakeBackend{connect: func(ctx context.Context, _ coreclient.ConnectRequest) (coreclient.Status, error) {
		close(entered)
		<-ctx.Done()
		return connected(), nil
	}}
	c := New(func(context.Context) (Backend, error) { return f, nil }, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
	defer c.Close()
	_, _ = c.Start(selection(true))
	<-entered
	if _, e := c.Disconnect(context.Background()); e != nil {
		t.Fatal(e)
	}
	s, _ := c.Status(context.Background())
	if s.State != coreclient.Stopped || !f.closed.Load() {
		t.Fatalf("late result survived stop: %+v", s)
	}
}

func TestLateStatusAndCheckCannotRestoreDisconnectedState(t *testing.T) {
	for _, kind := range []string{"status", "check"} {
		t.Run(kind, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			f := &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) { return connected(), nil }, status: func(context.Context) (coreclient.Status, error) { close(entered); <-release; return connected(), nil }}
			c := New(func(context.Context) (Backend, error) { return f, nil }, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
			defer c.Close()
			_, _ = c.Start(selection(false))
			c.mu.Lock()
			done := c.done
			c.mu.Unlock()
			<-done
			result := make(chan coreclient.Status, 1)
			go func() {
				var s coreclient.Status
				if kind == "status" {
					s, _ = c.Status(context.Background())
				} else {
					s, _ = c.CheckTunnel(context.Background())
				}
				result <- s
			}()
			<-entered
			if _, err := c.Disconnect(context.Background()); err != nil {
				t.Fatal(err)
			}
			close(release)
			select {
			case s := <-result:
				if s.State != coreclient.Stopped {
					t.Fatalf("stale %s restored %s", kind, s.State)
				}
			case <-time.After(time.Second):
				t.Fatal("old request did not return")
			}
		})
	}
}
