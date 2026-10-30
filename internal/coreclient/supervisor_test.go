package coreclient_test

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
	"github.com/mkulkov/awg-local-proxy/internal/coreclient/mock"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "--real-supervisor-owner" && os.Getenv("AWG_TEST_REAL_OWNER_MARKER") != "" {
		os.Exit(runRealOwner())
	}
	if len(os.Args) == 2 && os.Args[1] == "--control-stdio" && os.Getenv("AWG_TEST_REAL_CORE") != "" {
		os.Exit(runRealFixture())
	}
	if len(os.Args) == 2 && os.Args[1] == "--mock-core" {
		if marker := os.Getenv("AWG_TEST_STALLED_CORE"); marker != "" {
			r := bufio.NewReader(os.Stdin)
			m, err := coreclient.ReadMessage(r)
			if err != nil {
				os.Exit(2)
			}
			if err := coreclient.WriteMessage(os.Stdout, "hello", m.ID, coreclient.Hello{ProtocolVersion: 1, CoreVersion: "mock-0.1.0"}); err != nil {
				os.Exit(2)
			}
			if _, err := coreclient.ReadMessage(r); err != nil {
				os.Exit(2)
			}
			if err := os.WriteFile(marker, []byte("request received"), 0600); err != nil {
				os.Exit(2)
			}
			time.Sleep(time.Minute)
			os.Exit(0)
		}
		if err := mock.Serve(os.Stdin, os.Stdout); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestSupervisorCloseInterruptsBlockedRequest(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "request-received")
	t.Setenv("AWG_TEST_STALLED_CORE", marker)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	s, err := coreclient.StartMock(exe, hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	requestDone := make(chan error, 1)
	go func() { _, err := s.Connect(ctx); requestDone <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not receive request")
		}
		time.Sleep(10 * time.Millisecond)
	}
	queuedCtx, cancelQueued := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelQueued()
	queued := make(chan error, 1)
	go func() { _, err := s.Status(queuedCtx); queued <- err }()
	select {
	case err := <-queued:
		if err != context.DeadlineExceeded {
			t.Fatalf("queued request: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued request ignored its deadline")
	}
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close blocked behind IPC request")
	}
	select {
	case err := <-requestDone:
		if err == nil {
			t.Fatal("stalled request succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("request did not stop with its child")
	}
}

func TestSupervisorOwnedMockLifecycle(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	s, err := coreclient.StartMock(exe, hex.EncodeToString(h[:]))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < 100; i++ {
		st, err := s.Connect(ctx)
		if err != nil || st.State != coreclient.Degraded || !st.Demo {
			t.Fatalf("connect: %+v %v", st, err)
		}
		st, err = s.Disconnect(ctx)
		if err != nil || st.State != coreclient.Stopped {
			t.Fatalf("disconnect: %+v %v", st, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal("close is not idempotent")
	}
	if _, err := s.Status(ctx); err == nil {
		t.Fatal("closed core accepted request")
	}
	if _, err := coreclient.StartMock(exe, "incorrect"); err == nil {
		t.Fatal("hash mismatch accepted")
	}
	if _, err := coreclient.StartMock("relative.exe", hex.EncodeToString(h[:])); err == nil {
		t.Fatal("relative path accepted")
	}
}
