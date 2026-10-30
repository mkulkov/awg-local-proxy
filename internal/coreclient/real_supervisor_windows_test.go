//go:build windows

package coreclient_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestRealCoreJobTerminatesAfterOwnerDeath(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "owned-child-pid")
	t.Setenv("AWG_TEST_REAL_OWNER_MARKER", marker)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	owner := exec.Command(exe, "--real-supervisor-owner")
	owner.Stdout = io.Discard
	owner.Stderr = io.Discard
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Process.Kill(); _ = owner.Wait() }()
	var childPID int
	deadline := time.Now().Add(6 * time.Second)
	for {
		if b, err := os.ReadFile(marker); err == nil {
			childPID, err = strconv.Atoi(string(b))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("owner did not launch its controlled child")
		}
		time.Sleep(10 * time.Millisecond)
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(childPID))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	defer windows.TerminateProcess(h, 2)
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	state, err := windows.WaitForSingleObject(h, 3000)
	if err != nil || state != windows.WAIT_OBJECT_0 {
		t.Fatalf("owned core survived owner death: wait=%d err=%v", state, err)
	}
}
