//go:build windows

package preferences

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mkulkov/awg-local-proxy/internal/profiles"
	"golang.org/x/sys/windows"
)

func TestNativeDPAPIAndProtectedCurrentUserDACL(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "native"), profiles.DPAPI{})
	if err != nil {
		t.Fatal(err)
	}
	want := enabledSettings()
	if err = s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || got != want {
		t.Fatal("native DPAPI roundtrip failed")
	}
	b, err := os.ReadFile(filepath.Join(s.root, filename))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("SYNTHETIC_PRIVATE_TOKEN")) {
		t.Fatal("native plaintext persisted")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{s.root, filepath.Join(s.root, filename)} {
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		text := sd.String()
		if !strings.Contains(text, "D:P") || !strings.Contains(text, user.User.Sid.String()) {
			t.Fatal("protected current-user DACL missing")
		}
		if strings.Count(text, "(A;") != 1 {
			t.Fatal("unexpected additional allow ACE")
		}
	}
	b[len(b)/2] ^= 0xff
	if err = os.WriteFile(filepath.Join(s.root, filename), b, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err = s.Load(); err == nil || got != (Settings{}) {
		t.Fatal("corrupted native blob accepted")
	}
}
