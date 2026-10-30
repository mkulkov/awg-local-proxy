//go:build windows

package app

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestLegacyPowerWindowSyntheticEvents(t *testing.T) {
	var got atomic.Int32
	closeWindow, err := registerLegacyPowerEvents(func(e PowerEvent) {
		if e == PowerSuspend {
			got.Add(1)
		}
		if e == PowerResume {
			got.Add(10)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeWindow()
	var hwnd uintptr
	legacyWindows.Range(func(k, _ any) bool { hwnd = k.(uintptr); return false })
	if hwnd == 0 {
		t.Fatal("legacy window not registered")
	}
	send := legacyUser32.NewProc("SendMessageW")
	send.Call(hwnd, wmPowerBroadcast, pbtAPMSuspend, 0)
	send.Call(hwnd, wmPowerBroadcast, pbtAPMResumeAutomatic, 0)
	send.Call(hwnd, wmPowerBroadcast, pbtAPMResumeSuspend, 0)
	if got.Load() != 21 {
		t.Fatalf("unexpected synthetic event count: %d", got.Load())
	}
	done := make(chan error, 1)
	go func() { done <- closeWindow() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("window did not close")
	}
	if _, ok := legacyWindows.Load(hwnd); ok {
		t.Fatal("window still registered after close")
	}
	if err := closeWindow(); err != nil {
		t.Fatal(err)
	}
}
