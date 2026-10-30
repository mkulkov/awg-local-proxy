package profiles

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Test-only reversible marker; production always uses DPAPI, never this helper.
type fixtureProtector struct{ fail bool }

func (p fixtureProtector) Protect(b []byte) ([]byte, error) {
	if p.fail {
		return nil, errors.New("failure")
	}
	return append([]byte("TEST-ONLY:"), b...), nil
}
func (p fixtureProtector) Unprotect(b []byte) ([]byte, error) {
	if !bytes.HasPrefix(b, []byte("TEST-ONLY:")) {
		return nil, errors.New("corrupt")
	}
	return b[10:], nil
}

func TestMetadataAndEncryptedBlobSeparated(t *testing.T) {
	s, err := Open(t.TempDir(), fixtureProtector{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.CreateEncrypted("Test profile", []byte("SYNTHETIC_STORAGE_MARKER"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.root, m.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("SYNTHETIC_STORAGE_MARKER")) {
		t.Fatal("secret in metadata")
	}
	secret, err := s.ReadSecret(m.ID)
	if err != nil || string(secret) != "SYNTHETIC_STORAGE_MARKER" {
		t.Fatal("roundtrip failed")
	}
	if err := s.Rename(m.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].Name != "Renamed" {
		t.Fatal("metadata mismatch")
	}
	if _, err := s.ReadSecret("../escape"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err := os.WriteFile(filepath.Join(s.root, m.ID+".dpapi"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadSecret(m.ID); err == nil {
		t.Fatal("corruption accepted")
	}
}

func TestEncryptionFailureWritesNoPlaintext(t *testing.T) {
	s, err := Open(t.TempDir(), fixtureProtector{fail: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateEncrypted("Test", []byte("SYNTHETIC_STORAGE_MARKER")); err == nil {
		t.Fatal("failure ignored")
	}
	entries, err := os.ReadDir(s.root)
	if err != nil || len(entries) != 0 {
		t.Fatal("file left after encryption failure")
	}
}

func TestDemoMetadataAndSchemaFailure(t *testing.T) {
	s, err := Open(t.TempDir(), fixtureProtector{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.CreateDemo("Демо")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadSecret(m.ID); err == nil {
		t.Fatal("demo has secret")
	}
	if err := os.WriteFile(filepath.Join(s.root, m.ID+".json"), []byte(`{"schema":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(); err == nil {
		t.Fatal("unknown schema migrated silently")
	}
}

func TestDeleteOnlySelectedProfile(t *testing.T) {
	s, err := Open(t.TempDir(), fixtureProtector{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.CreateEncrypted("Delete test", []byte("SYNTHETIC_STORAGE_MARKER"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateDemo("Keep test")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("../escape"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err := s.Delete(m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadSecret(m.ID); err == nil {
		t.Fatal("secret still available")
	}
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].ID != other.ID {
		t.Fatal("unrelated profile changed")
	}
}
