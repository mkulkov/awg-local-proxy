package autostart

import (
	"errors"
	"testing"
)

type memoryStore struct {
	value        string
	exists       bool
	err          error
	writes, dels int
}

func (s *memoryStore) read() (string, bool, error) { return s.value, s.exists, s.err }
func (s *memoryStore) write(value string) error {
	s.writes++
	if s.err != nil {
		return s.err
	}
	s.value, s.exists = value, true
	return nil
}
func (s *memoryStore) remove() error {
	s.dels++
	if s.err != nil {
		return s.err
	}
	s.value, s.exists = "", false
	return nil
}

func TestMissingEntryStaysDisabledUntilExplicitEnable(t *testing.T) {
	s := &memoryStore{}
	const command = `"C:\Program Files\AWG Local Proxy\AwgLocalProxy.exe"`
	if enabled, err := get(s, command); enabled || err != nil || s.writes != 0 || s.dels != 0 {
		t.Fatal("reading a missing entry must not mutate startup")
	}
	if err := disable(s, command); err != nil || s.writes != 0 || s.dels != 0 {
		t.Fatal("disabling an absent entry must not mutate startup")
	}
	if err := enable(s, command); err != nil || s.value != command || s.writes != 1 {
		t.Fatal("explicit enable did not write the exact command")
	}
	if err := enable(s, command); err != nil || s.writes != 1 {
		t.Fatal("enable is not idempotent")
	}
	if enabled, err := get(s, command); !enabled || err != nil {
		t.Fatal("enabled entry not recognized")
	}
	if err := disable(s, command); err != nil || s.exists || s.dels != 1 {
		t.Fatal("exact owned entry not removed")
	}
}

func TestForeignEntriesArePreserved(t *testing.T) {
	const command = `"C:\AWG\AwgLocalProxy.exe"`
	for _, foreign := range []string{
		`"C:\Other\AwgLocalProxy.exe"`,
		command + " --unexpected",
		`%LOCALAPPDATA%\AwgLocalProxy.exe`,
		`C:\AWG\AwgLocalProxy.exe`,
		"",
	} {
		s := &memoryStore{value: foreign, exists: true}
		if _, err := get(s, command); !errors.Is(err, ErrConflict) {
			t.Fatal("foreign entry not reported as conflict")
		}
		if err := enable(s, command); !errors.Is(err, ErrConflict) {
			t.Fatal("foreign entry enabled/overwritten")
		}
		if err := disable(s, command); !errors.Is(err, ErrConflict) {
			t.Fatal("foreign entry removed")
		}
		if s.value != foreign || !s.exists || s.writes != 0 || s.dels != 0 {
			t.Fatal("foreign entry mutated")
		}
	}
}

func TestReadFailurePreventsStartupMutation(t *testing.T) {
	s := &memoryStore{err: errors.New("test access failure")}
	if err := enable(s, "owned"); err == nil {
		t.Fatal("access failure ignored")
	}
	if err := disable(s, "owned"); err == nil {
		t.Fatal("access failure ignored")
	}
	if s.writes != 0 || s.dels != 0 {
		t.Fatal("mutation attempted after failed read")
	}
}
