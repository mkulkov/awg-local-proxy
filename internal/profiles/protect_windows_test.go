//go:build windows

package profiles

import (
	"bytes"
	"testing"
)

func TestDPAPICurrentUserRoundtrip(t *testing.T) {
	p := DPAPI{}
	plain := []byte("SYNTHETIC_DPAPI_MARKER")
	b, err := p.Protect(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, plain) {
		t.Fatal("plaintext in encrypted blob")
	}
	got, err := p.Unprotect(b)
	if err != nil || !bytes.Equal(plain, got) {
		t.Fatal("DPAPI roundtrip failed")
	}
	b[0] ^= 0xff
	if _, err := p.Unprotect(b); err == nil {
		t.Fatal("corrupt blob accepted")
	}
}
