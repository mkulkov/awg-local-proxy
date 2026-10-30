// Package autostart manages an opt-in, current-user GUI startup entry.
// It does not connect a profile or change network settings.
package autostart

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf16"
)

var (
	// ErrConflict means the named value exists but is not this executable's
	// exact entry. The caller must not replace or delete it automatically.
	ErrConflict    = errors.New("autostart entry belongs to another executable or has an unexpected format")
	ErrUnsupported = errors.New("autostart is supported only on Windows")
	ErrExecutable  = errors.New("current executable cannot be used for autostart")
)

const valueName = "AwgLocalProxy"
const maxCommandUnits = 260

// Serialize this process's read/check/write operations. Other software running
// as the same user can still edit HKCU concurrently; this is not an isolation
// boundary against same-user malware.
var registryMu sync.Mutex

type valueStore interface {
	read() (value string, exists bool, err error)
	write(value string) error
	remove() error
}

func currentCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", ErrExecutable
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", ErrExecutable
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrExecutable
	}
	return commandForExecutable(executable)
}

func commandForExecutable(executable string) (string, error) {
	if !filepath.IsAbs(executable) || strings.HasPrefix(executable, `\\`) ||
		strings.ContainsAny(executable, "\x00\r\n\"") ||
		!strings.EqualFold(filepath.Ext(executable), ".exe") {
		return "", ErrExecutable
	}
	// The executable is the sole argument. Forced quoting also protects paths
	// with spaces; no command interpreter or environment expansion is involved.
	command := `"` + filepath.Clean(executable) + `"`
	if len(utf16.Encode([]rune(command))) > maxCommandUnits {
		return "", ErrExecutable
	}
	return command, nil
}

func get(store valueStore, command string) (bool, error) {
	value, exists, err := store.read()
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	if value != command {
		return false, ErrConflict
	}
	return true, nil
}

func enable(store valueStore, command string) error {
	enabled, err := get(store, command)
	if err != nil || enabled {
		return err
	}
	return store.write(command)
}

func disable(store valueStore, command string) error {
	enabled, err := get(store, command)
	if err != nil || !enabled {
		return err
	}
	return store.remove()
}
