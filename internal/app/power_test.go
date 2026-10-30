package app

import (
	"sync"
	"testing"
	"time"
)

func TestPowerDispatcherKeepsSuspendBeforeCoalescedResume(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	var events []PowerEvent
	d := newPowerDispatcher(func(e PowerEvent) {
		mu.Lock()
		events = append(events, e)
		first := len(events) == 1
		mu.Unlock()
		if first {
			close(entered)
			<-release
		}
	})
	d.enqueue(PowerSuspend)
	<-entered
	for i := 0; i < 1000; i++ {
		d.enqueue(PowerResume)
	}
	d.enqueue(PowerSuspend)
	d.enqueue(PowerResume)
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ready := len(events) >= 3
		mu.Unlock()
		if ready {
			break
		}
		time.Sleep(time.Millisecond)
	}
	d.stopAccepting()
	<-d.done
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 3 || events[0] != PowerSuspend || events[1] != PowerSuspend || events[2] != PowerResume {
		t.Fatalf("power event order = %v", events)
	}
	for i := 0; i < 100; i++ {
		d.enqueue(PowerResume)
	}
}

func TestPowerDispatcherCloseWaitsForHandlerAndDropsPending(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var count int
	d := newPowerDispatcher(func(PowerEvent) { count++; close(entered); <-release })
	d.enqueue(PowerSuspend)
	<-entered
	d.enqueue(PowerResume)
	d.stopAccepting()
	select {
	case <-d.done:
		t.Fatal("close ignored in-flight handler")
	default:
	}
	close(release)
	select {
	case <-d.done:
	case <-time.After(time.Second):
		t.Fatal("handler failed to finish")
	}
	if count != 1 {
		t.Fatal("pending resume ran after close")
	}
}
