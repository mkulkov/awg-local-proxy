package preferences

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Synthetic test protector keeps tests independent of the OS and is never used
// by OpenDefault. Encoding ensures assertions catch accidental plaintext writes.
type testProtector struct{ fail bool }

func (p *testProtector) Protect(b []byte) ([]byte, error) {
	if p.fail {
		return nil, errors.New("synthetic failure")
	}
	out := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
	base64.StdEncoding.Encode(out, b)
	return out, nil
}
func (p *testProtector) Unprotect(b []byte) ([]byte, error) {
	out := make([]byte, base64.StdEncoding.DecodedLen(len(b)))
	n, err := base64.StdEncoding.Decode(out, b)
	return out[:n], err
}

func enabledSettings() Settings {
	return Settings{AutoConnect: true, AutoReconnect: true, ProfileID: strings.Repeat("a", 32), ProbeURL: "https://probe.invalid/health?token=SYNTHETIC_PRIVATE_TOKEN", SocksPort: 1080, HTTPPort: 8080, HTTPEnabled: true}
}
func testStore(t *testing.T) (*Store, *testProtector) {
	t.Helper()
	p := &testProtector{}
	s, err := Open(filepath.Join(t.TempDir(), "settings"), p)
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}

func TestAbsentSettingsAreDisabledAndFlagsAreIndependent(t *testing.T) {
	s, _ := testStore(t)
	v, err := s.Load()
	if err != nil || v != (Settings{}) {
		t.Fatalf("absent settings not disabled: %#v %v", v, err)
	}
	if err = s.Save(Settings{AutoReconnect: true}); err != nil {
		t.Fatal(err)
	}
	v, err = s.Load()
	if err != nil || v.AutoConnect || !v.AutoReconnect || v.ProfileID != "" || v.ProbeURL != "" {
		t.Fatal("independent reconnect flag lost")
	}
}

func TestEncryptedRoundTripAndExplicitDisableClearsSelection(t *testing.T) {
	s, _ := testStore(t)
	want := enabledSettings()
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || got != want {
		t.Fatal("settings roundtrip failed")
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		b, err := os.ReadFile(filepath.Join(s.root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("SYNTHETIC_PRIVATE_TOKEN")) || bytes.Contains(b, []byte(want.ProfileID)) {
			t.Fatal("plaintext persisted")
		}
	}
	if len(entries) != 1 || entries[0].Name() != filename {
		t.Fatal("unexpected storage files")
	}
	if err = s.Save(Settings{}); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil || got != (Settings{}) {
		t.Fatal("disable did not clear saved selection")
	}
}

func TestEncryptionFailurePreservesPreviousFile(t *testing.T) {
	s, p := testStore(t)
	if err := s.Save(enabledSettings()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(s.root, filename))
	if err != nil {
		t.Fatal(err)
	}
	p.fail = true
	if err = s.Save(Settings{}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("encryption failure accepted")
	}
	after, err := os.ReadFile(filepath.Join(s.root, filename))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed save changed previous encrypted file")
	}
}

func TestCorruptionSchemaAliasesAndUnknownFieldsFailClosed(t *testing.T) {
	s, p := testStore(t)
	for _, plain := range []string{
		`{"schema":2,"settings":{"auto_connect":false,"auto_reconnect":false,"profile_id":"","probe_url":"","socks_port":0,"http_port":0,"http_enabled":false}}`,
		`{"schema":1,"Schema":1,"settings":{}}`,
		`{"schema":1,"settings":{"auto_connect":false,"auto_connect":true}}`,
		`{"schema":1,"settings":{"AutoConnect":true}}`,
		`{"schema":1,"settings":null}`,
		`{"schema":1,"settings":{}}`,
		`{"schema":1,"settings":{"auto_connect":false,"auto_reconnect":false,"profile_id":"","probe_url":"","socks_port":0,"http_port":0,"http_enabled":false},"unknown":"SYNTHETIC"}`,
		`not JSON`,
	} {
		b, _ := p.Protect([]byte(plain))
		if err := os.WriteFile(filepath.Join(s.root, filename), b, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := s.Load()
		if err == nil || got != (Settings{}) {
			t.Fatal("invalid settings did not fail closed")
		}
	}
	if err := os.WriteFile(filepath.Join(s.root, filename), []byte("corrupt cipher"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err == nil || got != (Settings{}) {
		t.Fatal("corrupt encrypted data did not fail closed")
	}
}

func TestSettingsValidationRejectsImplicitOrUnsafeSelection(t *testing.T) {
	base := enabledSettings()
	mutations := []func(*Settings){
		func(v *Settings) { v.ProfileID = "" }, func(v *Settings) { v.ProfileID = strings.Repeat("G", 32) },
		func(v *Settings) { v.ProbeURL = "" }, func(v *Settings) { v.ProbeURL = "ftp://probe.invalid/" },
		func(v *Settings) { v.ProbeURL = "https://user:secret@probe.invalid/" }, func(v *Settings) { v.ProbeURL = "https://probe.invalid/#fragment" },
		func(v *Settings) { v.ProbeURL = "https://probe.invalid:0/" }, func(v *Settings) { v.ProbeURL = "https://probe.invalid:65536/" },
		func(v *Settings) { v.ProbeURL = "https://probe.invalid:/" }, func(v *Settings) { v.ProbeURL = "https://probe.invalid/\n" },
		func(v *Settings) { v.SocksPort = 0 }, func(v *Settings) { v.SocksPort = 65536 }, func(v *Settings) { v.HTTPPort = v.SocksPort },
		func(v *Settings) { v.AutoConnect = false },
	}
	for _, mutate := range mutations {
		v := base
		mutate(&v)
		if v.Validate() == nil {
			t.Fatal("unsafe settings accepted")
		}
	}
	base.HTTPEnabled = false
	base.HTTPPort = base.SocksPort
	if base.Validate() != nil {
		t.Fatal("disabled HTTP unnecessarily conflicts")
	}
}

func TestAutoConnectChecksOnlyTheSavedProfileAndFailsClosed(t *testing.T) {
	s, _ := testStore(t)
	want := enabledSettings()
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	calls := 0
	got, err := s.LoadForAutoConnect(func(id string) error {
		calls++
		if id != want.ProfileID {
			t.Fatal("profile substituted")
		}
		return errors.New("synthetic removed or corrupt profile")
	})
	if !errors.Is(err, ErrProfileUnavailable) || got != (Settings{}) || calls != 1 {
		t.Fatal("missing/corrupt profile did not fail closed")
	}
	got, err = s.LoadForAutoConnect(nil)
	if !errors.Is(err, ErrProfileUnavailable) || got != (Settings{}) {
		t.Fatal("nil profile checker accepted")
	}
	got, err = s.LoadForAutoConnect(func(id string) error {
		if id != want.ProfileID {
			t.Fatal("wrong profile")
		}
		return nil
	})
	if err != nil || got != want {
		t.Fatal("exact valid profile rejected")
	}
}

func TestBoundedReadsAndConcurrentAtomicUpdates(t *testing.T) {
	s, _ := testStore(t)
	if err := os.WriteFile(filepath.Join(s.root, filename), bytes.Repeat([]byte("x"), maxCiphertextBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Load(); err == nil || got != (Settings{}) {
		t.Fatal("oversized settings accepted")
	}
	if err := s.Save(Settings{}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if err := s.Save(Settings{AutoReconnect: j%2 == 0}); err != nil {
					t.Error(err)
				}
				if _, err := s.Load(); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestStorageRejectsRelativePathsAndDirectoryTarget(t *testing.T) {
	if _, err := Open("relative", &testProtector{}); err == nil {
		t.Fatal("relative storage root accepted")
	}
	s, _ := testStore(t)
	if err := os.Mkdir(filepath.Join(s.root, filename), 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(Settings{}); err == nil {
		t.Fatal("directory target overwritten")
	}
	if got, err := s.Load(); err == nil || got != (Settings{}) {
		t.Fatal("directory target loaded")
	}
}
