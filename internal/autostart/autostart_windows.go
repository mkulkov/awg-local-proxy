//go:build windows

package autostart

import (
	"encoding/binary"
	"errors"
	"unicode/utf16"

	"golang.org/x/sys/windows/registry"
)

const runPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// Get reports whether this executable has an enabled startup entry. A missing
// entry is disabled. An unexpected existing value returns ErrConflict.
func Get() (bool, error) {
	registryMu.Lock()
	defer registryMu.Unlock()
	command, err := currentCommand()
	if err != nil {
		return false, err
	}
	store, close, err := openStore(runPath, false, false)
	if err != nil {
		return false, err
	}
	defer close()
	return get(store, command)
}

// Enable writes only this application's value after explicit user opt-in.
// It never replaces a value pointing at another executable.
func Enable() error {
	registryMu.Lock()
	defer registryMu.Unlock()
	command, err := currentCommand()
	if err != nil {
		return err
	}
	store, close, err := openStore(runPath, true, true)
	if err != nil {
		return err
	}
	defer close()
	return enable(store, command)
}

// Disable removes only the exact entry for this executable. It does not remove
// the Run key, other values, or entries from previous locations automatically.
func Disable() error {
	registryMu.Lock()
	defer registryMu.Unlock()
	command, err := currentCommand()
	if err != nil {
		return err
	}
	store, close, err := openStore(runPath, true, false)
	if err != nil {
		return err
	}
	defer close()
	return disable(store, command)
}

type registryStore struct {
	key     registry.Key
	missing bool
}

func openStore(path string, writable, create bool) (*registryStore, func(), error) {
	access := uint32(registry.QUERY_VALUE)
	if writable {
		access |= registry.SET_VALUE
	}
	var key registry.Key
	var err error
	if create {
		key, _, err = registry.CreateKey(registry.CURRENT_USER, path, access)
	} else {
		key, err = registry.OpenKey(registry.CURRENT_USER, path, access)
	}
	if errors.Is(err, registry.ErrNotExist) && !create {
		return &registryStore{missing: true}, func() {}, nil
	}
	if err != nil {
		return nil, nil, errors.New("cannot access current-user autostart entry")
	}
	return &registryStore{key: key}, func() { _ = key.Close() }, nil
}

func (s *registryStore) read() (string, bool, error) {
	if s.missing {
		return "", false, nil
	}
	// Use a fixed buffer instead of GetStringValue's growable allocation. A
	// larger or differently typed value is foreign and must be preserved.
	data := make([]byte, 2*(maxCommandUnits+1))
	n, kind, err := s.key.GetValue(valueName, data)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if errors.Is(err, registry.ErrShortBuffer) {
		return "", true, ErrConflict
	}
	if err != nil {
		return "", false, errors.New("cannot read current-user autostart entry")
	}
	if kind != registry.SZ || n < 2 || n > len(data) || n%2 != 0 {
		return "", true, ErrConflict
	}
	units := make([]uint16, n/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	if units[len(units)-1] != 0 {
		return "", true, ErrConflict
	}
	units = units[:len(units)-1]
	for _, unit := range units {
		if unit == 0 {
			return "", true, ErrConflict
		}
	}
	decoded := utf16.Decode(units)
	encoded := utf16.Encode(decoded)
	if len(encoded) != len(units) {
		return "", true, ErrConflict
	}
	for i := range encoded {
		if encoded[i] != units[i] {
			return "", true, ErrConflict
		}
	}
	return string(decoded), true, nil
}

func (s *registryStore) write(value string) error {
	if s.missing || s.key.SetStringValue(valueName, value) != nil {
		return errors.New("cannot enable current-user autostart")
	}
	return nil
}

func (s *registryStore) remove() error {
	if s.missing {
		return nil
	}
	err := s.key.DeleteValue(valueName)
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return errors.New("cannot disable current-user autostart")
	}
	return nil
}
