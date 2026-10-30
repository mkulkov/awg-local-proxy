//go:build windows

package autostart

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestExecutableCommandUsesOnlyQuotedAbsoluteEXE(t *testing.T) {
	for _, path := range []string{`C:\AWG\AwgLocalProxy.exe`, `C:\Program Files\AWG Local Proxy\AwgLocalProxy.EXE`} {
		command, err := commandForExecutable(path)
		if err != nil || command != `"`+path+`"` {
			t.Fatalf("unexpected executable command: %q, %v", command, err)
		}
	}
	for _, path := range []string{
		`AwgLocalProxy.exe`, `C:AwgLocalProxy.exe`, `C:\AWG\launch.cmd`,
		`\\server\share\AwgLocalProxy.exe`, `C:\AWG\bad"name.exe`,
		"C:\\AWG\\bad\nname.exe", "C:\\AWG\\bad\x00name.exe",
		`C:\` + strings.Repeat("a", 260) + `.exe`,
	} {
		if _, err := commandForExecutable(path); !errors.Is(err, ErrExecutable) {
			t.Fatalf("unsafe path accepted: %q", path)
		}
	}
}

func isolatedStore(t *testing.T) *registryStore {
	t.Helper()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	// This is deliberately outside the real Windows Run path. Delete only the
	// unique leaf created by this test; never enumerate or remove user keys.
	path := `Software\AwgLocalProxyTests\Autostart-` + hex.EncodeToString(random[:])
	key, exists, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		key.Close()
		t.Fatal("test namespace unexpectedly exists")
	}
	t.Cleanup(func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Errorf("could not remove unique test key: %v", err)
		}
	})
	return &registryStore{key: key}
}

func TestRegistryBackendPreservesOtherValuesAndForeignTypes(t *testing.T) {
	store := isolatedStore(t)
	const command = `"C:\Test\AwgLocalProxy.exe"`
	if err := store.key.SetStringValue("UnrelatedTestValue", "preserve"); err != nil {
		t.Fatal(err)
	}
	if enabled, err := get(store, command); enabled || err != nil {
		t.Fatal("new isolated key must be disabled")
	}
	if err := enable(store, command); err != nil {
		t.Fatal(err)
	}
	if enabled, err := get(store, command); !enabled || err != nil {
		t.Fatal("registry entry not enabled")
	}
	if err := disable(store, command); err != nil {
		t.Fatal(err)
	}
	if value, _, err := store.key.GetStringValue("UnrelatedTestValue"); value != "preserve" || err != nil {
		t.Fatal("unrelated registry value changed")
	}
	if err := store.key.SetExpandStringValue(valueName, command); err != nil {
		t.Fatal(err)
	}
	if err := disable(store, command); !errors.Is(err, ErrConflict) {
		t.Fatal("unexpected registry type was removed")
	}
	if err := enable(store, command); !errors.Is(err, ErrConflict) {
		t.Fatal("unexpected registry type was overwritten")
	}
	if err := store.key.SetStringValue(valueName, strings.Repeat("x", maxCommandUnits+50)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.read(); !errors.Is(err, ErrConflict) {
		t.Fatal("oversized value must fail closed")
	}
}

func TestMissingRegistryPathDoesNotGetCreatedByReadOrDisable(t *testing.T) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	path := `Software\AwgLocalProxyTests\Missing-` + hex.EncodeToString(random[:])
	s, close, err := openStore(path, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	if enabled, err := get(s, "owned"); enabled || err != nil {
		t.Fatal("missing path must be disabled")
	}
	if err := disable(s, "owned"); err != nil {
		t.Fatal(err)
	}
	if key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE); err == nil {
		key.Close()
		t.Fatal("read/disable created registry path")
	} else if !errors.Is(err, registry.ErrNotExist) {
		t.Fatal(err)
	}
}
