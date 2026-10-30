package coreclient_test

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
)

func runRealFixture() int {
	r := bufio.NewReader(os.Stdin)
	mode := os.Getenv("AWG_TEST_REAL_CORE")
	status := coreclient.Status{State: coreclient.Stopped}
	for {
		m, err := coreclient.ReadMessage(r)
		if err != nil {
			return 0
		}
		switch m.Type {
		case "hello":
			version := coreclient.RealCoreVersion
			if mode == "wrong_identity" {
				version = "mock-0.1.0"
			}
			if coreclient.WriteMessage(os.Stdout, "hello", m.ID, coreclient.Hello{ProtocolVersion: 1, CoreVersion: version, Commit: "independent", Capabilities: []string{"real_core", "stdio", "socks5", "http", "amneziawg"}}) != nil {
				return 2
			}
		case "validate_config":
			var payload struct {
				Config string `json:"config"`
			}
			if json.Unmarshal(m.Payload, &payload) != nil || payload.Config != "SYNTHETIC_CONFIG\n" {
				return 2
			}
			if coreclient.WriteMessage(os.Stdout, "validated", m.ID, map[string]bool{"valid": true}) != nil {
				return 2
			}
		case "connect":
			if mode == "error_transient" || mode == "error_permanent" || mode == "error_unknown" {
				code := "connect_failed"
				if mode == "error_permanent" {
					code = "invalid_config"
				}
				if mode == "error_unknown" {
					code = "SYNTHETIC_PRIVATE_SECRET"
				}
				if coreclient.WriteMessage(os.Stdout, "error", m.ID, map[string]string{"code": code}) != nil {
					return 2
				}
				continue
			}
			if mode == "stalled" {
				if marker := os.Getenv("AWG_TEST_REAL_OWNER_MARKER"); marker != "" {
					if os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0600) != nil {
						return 2
					}
				}
				time.Sleep(time.Minute)
				return 0
			}
			now := time.Now()
			status = coreclient.Status{State: coreclient.Connected, ListenerReady: true, TunnelVerified: true, HandshakeAt: &now, Message: "SYNTHETIC_SECRET_FROM_CHILD", SocksAddress: "127.0.0.1:1080"}
			if mode == "unverified" {
				status.TunnelVerified = false
			}
			if mode == "missing_address" {
				status.SocksAddress = ""
			}
			if mode == "wrong_address" {
				status.SocksAddress = "127.0.0.1:1081"
			}
			if coreclient.WriteMessage(os.Stdout, "status", m.ID, status) != nil {
				return 2
			}
		case "get_status", "disconnect", "check_tunnel":
			if m.Type == "disconnect" {
				status = coreclient.Status{State: coreclient.Stopped}
			}
			if coreclient.WriteMessage(os.Stdout, "status", m.ID, status) != nil {
				return 2
			}
		case "shutdown":
			if coreclient.WriteMessage(os.Stdout, "shutdown", m.ID, struct{}{}) != nil {
				return 2
			}
			return 0
		default:
			return 2
		}
	}
}

func runRealOwner() int {
	if os.Setenv("AWG_TEST_REAL_CORE", "stalled") != nil {
		return 2
	}
	exe, err := os.Executable()
	if err != nil {
		return 2
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		return 2
	}
	h := sha256.Sum256(b)
	s, err := coreclient.StartReal(exe, hex.EncodeToString(h[:]))
	if err != nil {
		return 2
	}
	defer s.Close()
	options := syntheticConnectRequest()
	options.TimeoutSeconds = 60
	_, _ = s.ConnectProfile(context.Background(), options)
	return 0
}

func startRealFixture(t *testing.T, mode string) *coreclient.RealSupervisor {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("real managed core requires Windows Job Objects")
	}
	t.Setenv("AWG_TEST_REAL_CORE", mode)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	s, err := coreclient.StartReal(exe, hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func syntheticConnectRequest() coreclient.ConnectRequest {
	return coreclient.ConnectRequest{Config: []byte("SYNTHETIC_CONFIG\n"), SocksAddress: "127.0.0.1:1080", ProbeURL: "https://example.invalid/check", TimeoutSeconds: 1}
}

func TestRealSupervisorValidatedLifecycle(t *testing.T) {
	s := startRealFixture(t, "normal")
	ctx := context.Background()
	if err := s.ValidateConfig(ctx, []byte("SYNTHETIC_CONFIG\n")); err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateConfig(ctx, []byte(strings.Repeat("\n", coreclient.MaxMessageBytes/3))); err == nil {
		t.Fatal("escaped frame limit ignored")
	}
	if _, err := s.Status(ctx); err != nil {
		t.Fatal("local frame validation stopped a healthy core")
	}
	st, err := s.ConnectProfile(ctx, syntheticConnectRequest())
	if err != nil || st.State != coreclient.Connected || strings.Contains(st.Message, "SYNTHETIC_SECRET") {
		t.Fatalf("unexpected sanitized state: %+v %v", st, err)
	}
	if st.SocksAddress != "127.0.0.1:1080" {
		t.Fatal("core proxy address lost")
	}
	if st, err = s.CheckTunnel(ctx); err != nil || st.State != coreclient.Connected {
		t.Fatal("tunnel check failed")
	}
	st, err = s.Disconnect(ctx)
	if err != nil || st.State != coreclient.Stopped {
		t.Fatal("disconnect failed")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal("Close not idempotent")
	}
}

func TestRealSupervisorRejectsUnverifiedState(t *testing.T) {
	s := startRealFixture(t, "unverified")
	if _, err := s.ConnectProfile(context.Background(), syntheticConnectRequest()); err == nil {
		t.Fatal("unverified tunnel accepted")
	}
	if _, err := s.Status(context.Background()); err == nil {
		t.Fatal("invalid core remained usable")
	}
}

func TestRealSupervisorRequiresRequestedAddresses(t *testing.T) {
	for _, mode := range []string{"missing_address", "wrong_address"} {
		t.Run(mode, func(t *testing.T) {
			s := startRealFixture(t, mode)
			if _, err := s.ConnectProfile(context.Background(), syntheticConnectRequest()); err == nil {
				t.Fatal("unconfirmed listener accepted")
			}
		})
	}
}

func TestRealSupervisorCloseKillsStalledJob(t *testing.T) {
	s := startRealFixture(t, "stalled")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := s.ConnectProfile(ctx, syntheticConnectRequest()); done <- err }()
	time.Sleep(100 * time.Millisecond)
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not terminate the stalled job")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("stalled request succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("request did not exit")
	}
}

func TestRealSupervisorReturnsOnlySafeTypedRetryCodes(t *testing.T) {
	for _, tt := range []struct {
		mode  string
		retry bool
	}{{"error_transient", true}, {"error_permanent", false}, {"error_unknown", false}} {
		t.Run(tt.mode, func(t *testing.T) {
			s := startRealFixture(t, tt.mode)
			_, err := s.ConnectProfile(context.Background(), syntheticConnectRequest())
			if err == nil || coreclient.IsRetryable(err) != tt.retry || strings.Contains(err.Error(), "SYNTHETIC_PRIVATE_SECRET") {
				t.Fatal("unsafe or misclassified child error")
			}
		})
	}
}
