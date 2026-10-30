//go:build windows

package app

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNativePowerRegistrationAndSyntheticCallback(t *testing.T) {
	var seen atomic.Int32
	unregister, err := registerPowerEvents(func(e PowerEvent) {
		if e == PowerSuspend {
			seen.Add(1)
		}
		if e == PowerResume {
			seen.Add(10)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()
	var id uintptr
	powerCallbacks.Range(func(k, v any) bool {
		r := v.(*nativePowerRegistration)
		if r.id > id {
			id = r.id
		}
		return true
	})
	if id == 0 {
		t.Fatal("native registration not retained")
	}
	nativePowerCallback(id, pbtAPMSuspend, 0)
	nativePowerCallback(id, pbtAPMResumeAutomatic, 0)
	nativePowerCallback(id, pbtAPMResumeSuspend, 0)
	if seen.Load() != 21 {
		t.Fatal("native event mapping failed")
	}
	if err = unregister(); err != nil {
		t.Fatal(err)
	}
	nativePowerCallback(id, pbtAPMSuspend, 0)
	if seen.Load() != 21 {
		t.Fatal("late callback reached closed registration")
	}
}

func TestNativePowerMonitorRegistersWithoutChangingPowerState(t *testing.T) {
	m, err := WatchPowerEvents(func(PowerEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- m.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("native unregister did not finish")
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
}
