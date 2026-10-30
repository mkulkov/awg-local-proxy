//go:build windows

package coreclient

import (
	"context"
	"os"
	"testing"
	"time"
)

// Run with an independently built, explicitly pinned core. No real profile is
// needed; the core must reject this synthetic invalid configuration.
func TestBuiltCoreControlIntegration(t *testing.T) {
	path, hash := os.Getenv("AWG_TEST_CORE_EXE"), os.Getenv("AWG_TEST_CORE_SHA256")
	if path == "" || hash == "" {
		t.Skip("set AWG_TEST_CORE_EXE and AWG_TEST_CORE_SHA256 to test a built core")
	}
	client, err := StartReal(path, hash)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = client.Hello(ctx); err != nil {
		t.Fatal(err)
	}
	if err = client.ValidateConfig(ctx, []byte("synthetic invalid profile")); err == nil {
		t.Fatal("invalid config accepted")
	}
	s, err := client.Status(ctx)
	if err != nil || s.State != Stopped || s.ListenerReady || s.TunnelVerified {
		t.Fatalf("unexpected stopped state: %#v, %v", s, err)
	}
	if _, err = client.Disconnect(ctx); err != nil {
		t.Fatal(err)
	}
}
