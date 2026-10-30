// Package session owns the single real-core lifecycle and optional bounded retry.
package session

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
)

type Backend interface {
	coreclient.Client
	ConnectProfile(context.Context, coreclient.ConnectRequest) (coreclient.Status, error)
	CheckTunnel(context.Context) (coreclient.Status, error)
}

type Factory func(context.Context) (Backend, error)
type SecretLoader func(string) ([]byte, error)

// Selection contains no secret configuration. Reconnect is a per-connection,
// explicit opt-in; each retry repeats only this already selected probe URL.
type Selection struct {
	ProfileID, SocksAddress, HTTPAddress, ProbeURL string
	Reconnect                                      bool
}

type Controller struct {
	commands                 sync.Mutex
	mu                       sync.Mutex
	factory                  Factory
	load                     SecretLoader
	backend                  Backend
	backendGeneration        uint64
	selection                Selection
	status                   coreclient.Status
	generation               uint64
	desired, running, closed bool
	suspended                bool
	resumeWanted             bool
	resumeSelection          Selection
	cancel                   context.CancelFunc
	done                     chan struct{}
	wait                     func(context.Context, time.Duration) error
	delay                    func(int) time.Duration
}

func New(factory Factory, load SecretLoader, initial Backend) *Controller {
	return &Controller{factory: factory, load: load, backend: initial,
		status: coreclient.Status{State: coreclient.Stopped}, wait: waitContext, delay: retryDelay}
}

func waitContext(ctx context.Context, delay time.Duration) error {
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func retryDelay(retry int) time.Duration {
	if retry < 1 {
		retry = 1
	}
	if retry > 5 {
		retry = 5
	}
	base := time.Second * time.Duration(1<<uint(retry-1))
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return base
	}
	// 75–125 percent jitter, capped below 21 seconds.
	return base*3/4 + time.Duration(binary.LittleEndian.Uint64(b[:])%uint64(base/2+1))
}

// Start returns immediately. Status exposes Connecting/Reconnecting while the
// owned worker runs; Disconnect remains available to cancel it.
func (c *Controller) Start(selection Selection) (coreclient.Status, error) {
	c.commands.Lock()
	defer c.commands.Unlock()
	if selection.ProfileID == "" || c.factory == nil || c.load == nil {
		return coreclient.Status{}, errors.New("select a stored profile before connecting")
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return coreclient.Status{}, errors.New("session is closed")
	}
	if c.suspended {
		status := c.status
		c.mu.Unlock()
		return status, errors.New("session is suspended")
	}
	c.resumeWanted = false
	idle := !c.desired && !c.running && c.status.State == coreclient.Stopped
	c.mu.Unlock()
	if !idle {
		if err := c.stop(); err != nil {
			c.mu.Lock()
			status := c.status
			c.mu.Unlock()
			return status, err
		}
	}
	return c.launch(selection, false), nil
}

func (c *Controller) launch(selection Selection, recovery bool) coreclient.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	gen := c.generation
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.done = make(chan struct{})
	c.running = true
	c.desired = true
	c.selection = selection
	state := coreclient.Connecting
	if recovery {
		state = coreclient.Reconnecting
	}
	c.status = coreclient.Status{State: state, Message: "Ожидание защищённого подключения."}
	done := c.done
	go c.run(ctx, gen, selection, recovery, done)
	return c.status
}

func (c *Controller) run(ctx context.Context, gen uint64, selection Selection, recovery bool, done chan struct{}) {
	defer close(done)
	defer func() {
		c.mu.Lock()
		if c.generation == gen {
			c.running = false
			c.cancel = nil
		}
		c.mu.Unlock()
	}()
	for attempt := 1; attempt <= 5; attempt++ {
		if ctx.Err() != nil {
			return
		}
		if recovery || attempt > 1 {
			c.setStatus(gen, coreclient.Status{State: coreclient.Reconnecting, Message: "Повтор подключения по выбранным настройкам."})
			retry := attempt - 1
			if recovery {
				retry = attempt
			}
			if err := c.wait(ctx, c.delay(retry)); err != nil {
				return
			}
		}
		status, err := c.attempt(ctx, gen, selection)
		if err == nil {
			c.setStatus(gen, status)
			return
		}
		if ctx.Err() != nil {
			return
		}
		if !selection.Reconnect || !coreclient.IsRetryable(err) || attempt == 5 {
			c.mu.Lock()
			if c.generation == gen {
				c.desired = false
				c.status = coreclient.Status{State: coreclient.Error, Message: "Подключение не установлено. Проверьте профиль, порты и доступность сервера."}
			}
			c.mu.Unlock()
			return
		}
	}
}

func (c *Controller) attempt(ctx context.Context, gen uint64, s Selection) (coreclient.Status, error) {
	c.mu.Lock()
	backend := c.backend
	if backend != nil {
		c.backendGeneration = gen
	}
	c.mu.Unlock()
	if backend == nil {
		var err error
		backend, err = c.factory(ctx)
		if err != nil {
			return coreclient.Status{}, err
		}
		if backend == nil {
			return coreclient.Status{}, errors.New("core factory returned no process")
		}
		c.mu.Lock()
		c.backend = backend
		c.backendGeneration = gen
		if c.generation != gen || ctx.Err() != nil {
			c.mu.Unlock()
			if err := c.closeBackend(backend, gen); err != nil {
				return coreclient.Status{}, err
			}
			return coreclient.Status{}, context.Canceled
		}
		c.mu.Unlock()
	}
	secret, err := c.load(s.ProfileID)
	if err == nil {
		attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		status, connectErr := backend.ConnectProfile(attemptCtx, coreclient.ConnectRequest{Config: secret, SocksAddress: s.SocksAddress, HTTPAddress: s.HTTPAddress, ProbeURL: s.ProbeURL, TimeoutSeconds: 15})
		cancel()
		clear(secret)
		if connectErr == nil && status.State == coreclient.Connected && status.Validate() == nil {
			return status, nil
		}
		if connectErr == nil {
			connectErr = errors.New("core did not confirm a tunnel")
		}
		err = connectErr
	} else {
		clear(secret)
	}
	if closeErr := c.closeBackend(backend, gen); closeErr != nil {
		return coreclient.Status{}, closeErr
	}
	return coreclient.Status{}, err
}

func (c *Controller) closeBackend(backend Backend, owner uint64) error {
	if err := backend.Close(); err != nil {
		return errors.New("core shutdown was not confirmed")
	}
	c.mu.Lock()
	if c.backendGeneration == owner {
		c.backend = nil
	}
	c.mu.Unlock()
	return nil
}

func (c *Controller) setStatus(gen uint64, status coreclient.Status) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation == gen && c.desired && !c.closed {
		c.status = status
	}
}

func (c *Controller) stop() error {
	c.mu.Lock()
	c.generation++
	gen := c.generation
	c.desired = false
	cancel, done, backend, owner := c.cancel, c.done, c.backend, c.backendGeneration
	c.cancel = nil
	c.status = coreclient.Status{State: coreclient.Stopping}
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if backend != nil {
		// Keep ownership until Close confirms termination. A late factory may
		// publish its child while cancellation is being processed below.
		_ = c.closeBackend(backend, owner)
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			c.mu.Lock()
			c.status = coreclient.Status{State: coreclient.Error, Message: "Остановка процесса ядра не подтверждена."}
			c.mu.Unlock()
			return errors.New("core shutdown did not complete")
		}
	}
	c.mu.Lock()
	backend, owner = c.backend, c.backendGeneration
	c.mu.Unlock()
	if backend != nil {
		if err := c.closeBackend(backend, owner); err != nil {
			c.mu.Lock()
			c.running = false
			c.status = coreclient.Status{State: coreclient.Error, Message: "Остановка процесса ядра не подтверждена."}
			c.mu.Unlock()
			return err
		}
	}
	c.mu.Lock()
	if c.generation == gen {
		c.running = false
		c.done = nil
		c.status = coreclient.Status{State: coreclient.Stopped, Message: "Подключение остановлено."}
	}
	c.mu.Unlock()
	return nil
}

func (c *Controller) Disconnect(context.Context) (coreclient.Status, error) {
	c.commands.Lock()
	defer c.commands.Unlock()
	c.mu.Lock()
	c.resumeWanted = false
	c.mu.Unlock()
	err := c.stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status, err
}
func (c *Controller) Close() error {
	c.commands.Lock()
	defer c.commands.Unlock()
	c.mu.Lock()
	c.closed = true
	c.resumeWanted = false
	c.mu.Unlock()
	return c.stop()
}

// Suspend stops the owned process and cancels attempts/backoff. Recovery is
// retained only for an already desired session whose user opted into reconnect.
// Native power callbacks call this on a worker, never on the Windows callback.
func (c *Controller) Suspend() (coreclient.Status, error) {
	c.commands.Lock()
	defer c.commands.Unlock()
	c.mu.Lock()
	if c.closed || c.suspended {
		status := c.status
		c.mu.Unlock()
		return status, nil
	}
	wanted := c.desired && c.selection.Reconnect
	selection := c.selection
	c.suspended = true
	c.resumeWanted = false
	c.mu.Unlock()
	err := c.stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil && wanted && !c.closed {
		c.resumeWanted = true
		c.resumeSelection = selection
		c.status = coreclient.Status{State: coreclient.Reconnecting, Message: "Сеанс приостановлен до выхода из сна."}
	}
	return c.status, err
}

// Resume consumes a single suspend intent. Duplicate or unsolicited resume
// events cannot start a process, and Disconnect/Close invalidate the intent.
func (c *Controller) Resume() (coreclient.Status, error) {
	c.commands.Lock()
	defer c.commands.Unlock()
	c.mu.Lock()
	if !c.suspended {
		status := c.status
		c.mu.Unlock()
		return status, nil
	}
	c.suspended = false
	wanted, selection := c.resumeWanted && !c.closed, c.resumeSelection
	c.resumeWanted = false
	c.resumeSelection = Selection{}
	status := c.status
	c.mu.Unlock()
	if !wanted {
		return status, nil
	}
	// Suspend records recovery only after confirmed child shutdown. Keep this
	// check defensive: never launch while an owned process is still uncertain.
	c.mu.Lock()
	unsafe := c.backend != nil || c.running
	c.mu.Unlock()
	if unsafe {
		return status, errors.New("suspended core shutdown was not confirmed")
	}
	return c.launch(selection, true), nil
}
func (c *Controller) Connect(context.Context) (coreclient.Status, error) {
	return coreclient.Status{}, errors.New("select a profile before connecting")
}
func (c *Controller) Hello(ctx context.Context) (coreclient.Hello, error) {
	c.mu.Lock()
	b := c.backend
	c.mu.Unlock()
	if b == nil {
		return coreclient.Hello{}, errors.New("core is stopped")
	}
	return b.Hello(ctx)
}

func (c *Controller) Status(ctx context.Context) (coreclient.Status, error) {
	c.mu.Lock()
	gen, b, s, running := c.generation, c.backend, c.status, c.running
	c.mu.Unlock()
	if running || b == nil || s.State != coreclient.Connected && s.State != coreclient.Degraded {
		return s, nil
	}
	status, err := b.Status(ctx)
	if err == nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.generation != gen {
			return c.status, nil
		}
		if c.desired && !c.closed {
			c.status = status
			if status.State == coreclient.Stopped || status.State == coreclient.Error {
				c.desired = false
			}
		}
		return c.status, nil
	}
	return c.failure(gen, err)
}

func (c *Controller) CheckTunnel(ctx context.Context) (coreclient.Status, error) {
	c.mu.Lock()
	gen, b, running := c.generation, c.backend, c.running
	c.mu.Unlock()
	if b == nil || running {
		return coreclient.Status{}, errors.New("no connected tunnel")
	}
	status, err := b.CheckTunnel(ctx)
	if err != nil {
		return c.failure(gen, err)
	}
	c.mu.Lock()
	if c.generation != gen {
		current := c.status
		c.mu.Unlock()
		return current, nil
	}
	if c.generation == gen {
		c.status = status
		if status.State == coreclient.Error || status.State == coreclient.Stopped {
			c.desired = false
		}
	}
	c.mu.Unlock()
	return status, nil
}

func (c *Controller) failure(gen uint64, err error) (coreclient.Status, error) {
	c.commands.Lock()
	defer c.commands.Unlock()
	c.mu.Lock()
	if c.generation != gen {
		status := c.status
		c.mu.Unlock()
		return status, nil
	}
	s, desired, closed := c.selection, c.desired, c.closed
	c.mu.Unlock()
	if stopErr := c.stop(); stopErr != nil {
		return coreclient.Status{State: coreclient.Error}, stopErr
	}
	if !closed && desired && s.Reconnect && coreclient.IsRetryable(err) {
		return c.launch(s, true), nil
	}
	c.mu.Lock()
	c.status = coreclient.Status{State: coreclient.Error, Message: "Процесс ядра недоступен. Подключитесь повторно."}
	status := c.status
	c.mu.Unlock()
	return status, nil
}
