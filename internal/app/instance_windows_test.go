//go:build windows

package app

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSingleInstanceActivation(t *testing.T) {
	name := fmt.Sprintf("AwgLocalProxyDemoTest-%d-%d", os.Getpid(), time.Now().UnixNano())
	i, exists, err := acquireInstance(name)
	if err != nil || exists {
		t.Fatalf("first: %v %v", exists, err)
	}
	defer i.Close()
	woke := make(chan struct{}, 1)
	i.Watch(func() {
		select {
		case woke <- struct{}{}:
		default:
		}
	})
	second, exists, err := acquireInstance(name)
	if err != nil || !exists || second != nil {
		t.Fatalf("second: %v %v", exists, err)
	}
	select {
	case <-woke:
	case <-time.After(2 * time.Second):
		t.Fatal("activation not signalled")
	}
	i.Close()
	third, exists, err := acquireInstance(name)
	if err != nil || exists {
		t.Fatalf("reopen: %v %v", exists, err)
	}
	third.Close()
}
