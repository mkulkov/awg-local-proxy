package session

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
)

func powerController(t *testing.T, reconnect bool) (*Controller, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var made, closed atomic.Int32
	c := New(func(context.Context) (Backend, error) {
		made.Add(1)
		return &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) { return connected(), nil }, onClose: func() { closed.Add(1) }}, nil
	}, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
	c.wait = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Start(selection(reconnect)); err != nil {
		t.Fatal(err)
	}
	waitState(t, c, coreclient.Connected)
	return c, &made, &closed
}

func TestPowerResumeRequiresPriorActiveOptIn(t *testing.T) {
	for _, reconnect := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[reconnect], func(t *testing.T) {
			c, made, closed := powerController(t, reconnect)
			if _, err := c.Suspend(); err != nil {
				t.Fatal(err)
			}
			if closed.Load() != 1 {
				t.Fatal("suspend left old core alive")
			}
			if _, err := c.Start(selection(true)); err == nil {
				t.Fatal("manual start opened core while suspended")
			}
			_, _ = c.Suspend() // Duplicate preserves original recovery intent.
			if _, err := c.Resume(); err != nil {
				t.Fatal(err)
			}
			if reconnect {
				waitState(t, c, coreclient.Connected)
				if made.Load() != 2 {
					t.Fatal("resume did not create exactly one fresh core")
				}
			} else if made.Load() != 1 {
				t.Fatal("resume connected without opt-in")
			}
			_, _ = c.Resume() // Windows may emit automatic + interactive resume.
			if made.Load() != map[bool]int32{false: 1, true: 2}[reconnect] {
				t.Fatal("duplicate resume created another core")
			}
		})
	}
}

func TestPowerDisconnectAndQuitCancelResumeIntent(t *testing.T) {
	for _, quit := range []bool{false, true} {
		c, made, _ := powerController(t, true)
		_, _ = c.Suspend()
		if quit {
			if err := c.Close(); err != nil {
				t.Fatal(err)
			}
		} else if _, err := c.Disconnect(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Resume(); err != nil {
			t.Fatal(err)
		}
		if made.Load() != 1 {
			t.Fatal("resume overrode explicit stop")
		}
	}
}

func TestPowerUnsolicitedResumeDoesNotConnect(t *testing.T) {
	var made atomic.Int32
	c := New(func(context.Context) (Backend, error) { made.Add(1); return nil, errors.New("must not run") }, func(string) ([]byte, error) { t.Fatal("unexpected profile access"); return nil, nil }, nil)
	defer c.Close()
	_, _ = c.Resume()
	_, _ = c.Suspend()
	_, _ = c.Resume()
	if made.Load() != 0 {
		t.Fatal("idle power events started a core")
	}
}

func TestPowerUnconfirmedShutdownDoesNotResume(t *testing.T) {
	var made atomic.Int32
	var permit atomic.Bool
	f := &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) { return connected(), nil }, closeError: func() error {
		if !permit.Load() {
			return errors.New("synthetic alive child")
		}
		return nil
	}}
	c := New(func(context.Context) (Backend, error) { made.Add(1); return f, nil }, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
	defer func() { permit.Store(true); _ = c.Close() }()
	_, _ = c.Start(selection(true))
	waitState(t, c, coreclient.Connected)
	if _, err := c.Suspend(); err == nil {
		t.Fatal("unconfirmed close accepted")
	}
	_, _ = c.Resume()
	if made.Load() != 1 {
		t.Fatal("resume launched alongside uncertain child")
	}
}

func TestPowerSuspendCancelsBackoffBeforeResume(t *testing.T) {
	var made, waits atomic.Int32
	backoff := make(chan struct{})
	c := New(func(context.Context) (Backend, error) {
		n := made.Add(1)
		return &fakeBackend{connect: func(context.Context, coreclient.ConnectRequest) (coreclient.Status, error) {
			if n == 1 {
				return coreclient.Status{}, &coreclient.TransportError{}
			}
			return connected(), nil
		}}, nil
	}, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
	defer c.Close()
	c.wait = func(ctx context.Context, _ time.Duration) error {
		if waits.Add(1) == 1 {
			close(backoff)
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	}
	_, _ = c.Start(selection(true))
	<-backoff
	if _, err := c.Suspend(); err != nil {
		t.Fatal(err)
	}
	if made.Load() != 1 {
		t.Fatal("backoff launched while suspended")
	}
	if _, err := c.Resume(); err != nil {
		t.Fatal(err)
	}
	waitState(t, c, coreclient.Connected)
	if made.Load() != 2 {
		t.Fatal("resume did not use one new process")
	}
}

func TestPowerSuspendRejectsLateConnectedResult(t *testing.T) {
	entered := make(chan struct{})
	f := &fakeBackend{connect: func(ctx context.Context, _ coreclient.ConnectRequest) (coreclient.Status, error) {
		close(entered)
		<-ctx.Done()
		return connected(), nil
	}}
	c := New(func(context.Context) (Backend, error) { return f, nil }, func(string) ([]byte, error) { return []byte("SYNTHETIC"), nil }, nil)
	defer c.Close()
	_, _ = c.Start(selection(false))
	<-entered
	status, err := c.Suspend()
	if err != nil || status.State != coreclient.Stopped || status.ListenerReady || !f.closed.Load() {
		t.Fatal("suspend accepted stale connected result")
	}
	status, err = c.Resume()
	if err != nil || status.State != coreclient.Stopped {
		t.Fatal("resume restarted non-opt-in session")
	}
}
