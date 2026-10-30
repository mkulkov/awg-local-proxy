package coreclient_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
	"github.com/mkulkov/awg-local-proxy/internal/coreclient/mock"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "--mock-core" {
		if err := mock.Serve(os.Stdin, os.Stdout); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
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
