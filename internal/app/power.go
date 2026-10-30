package app

import (
	"errors"
	"sync"
)

// PowerEvent conveys only lifecycle, never a request to change Windows power
// settings. A handler runs on one worker rather than inside a native callback.
type PowerEvent uint8

const (
	PowerSuspend PowerEvent = iota + 1
	PowerResume
)

type powerDispatcher struct {
	mu             sync.Mutex
	handler        func(PowerEvent)
	wake           chan struct{}
	stop           chan struct{}
	done           chan struct{}
	closed         bool
	pendingSuspend bool
	pending        bool
	last           PowerEvent
}

func newPowerDispatcher(handler func(PowerEvent)) *powerDispatcher {
	d := &powerDispatcher{handler: handler, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	go d.run()
	return d
}

func (d *powerDispatcher) enqueue(event PowerEvent) {
	if event != PowerSuspend && event != PowerResume {
		return
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	// A suspend can never disappear behind a quick resume. Repeated native
	// notifications are coalesced to a bounded suspend-then-final-state queue.
	if event == PowerSuspend {
		d.pendingSuspend = true
	}
	d.pending, d.last = true, event
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *powerDispatcher) next() (PowerEvent, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return 0, false
	}
	if d.pendingSuspend {
		d.pendingSuspend = false
		if d.last == PowerSuspend {
			d.pending = false
		}
		return PowerSuspend, true
	}
	if d.pending {
		d.pending = false
		return d.last, true
	}
	return 0, false
}

func (d *powerDispatcher) run() {
	defer close(d.done)
	for {
		select {
		case <-d.stop:
			return
		case <-d.wake:
			for {
				event, ok := d.next()
				if !ok {
					break
				}
				d.handler(event)
			}
		}
	}
}

func (d *powerDispatcher) stopAccepting() {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		d.pending, d.pendingSuspend = false, false
		close(d.stop)
	}
	d.mu.Unlock()
}

// PowerMonitor keeps native callback lifetime separate from the bounded
// lifecycle worker. Close unregisters and waits for an in-flight handler.
type PowerMonitor struct {
	dispatcher *powerDispatcher
	unregister func() error
	once       sync.Once
	err        error
}

func WatchPowerEvents(handler func(PowerEvent)) (*PowerMonitor, error) {
	if handler == nil {
		return nil, errors.New("power event handler is required")
	}
	d := newPowerDispatcher(handler)
	unregister, err := registerPowerEvents(d.enqueue)
	if err != nil {
		d.stopAccepting()
		<-d.done
		return nil, err
	}
	return &PowerMonitor{dispatcher: d, unregister: unregister}, nil
}

func (m *PowerMonitor) Close() error {
	if m == nil {
		return nil
	}
	m.once.Do(func() {
		m.dispatcher.stopAccepting()
		m.err = m.unregister()
		<-m.dispatcher.done
	})
	return m.err
}
