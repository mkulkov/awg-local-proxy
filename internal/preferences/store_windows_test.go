//go:build windows

package preferences

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

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
		if err := protectedSingleUserDACL(sd, user.User.Sid); err != nil {
			t.Fatal(err)
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

// Check the descriptor's binary semantics, not its SDDL spelling: Windows may
// serialize a full SID using a well-known alias such as LA (RID 500) or SY.
// FILE_ALL_ACCESS is STANDARD_RIGHTS_REQUIRED | SYNCHRONIZE | 0x1ff.
func protectedSingleUserDACL(sd *windows.SECURITY_DESCRIPTOR, expected *windows.SID) error {
	const fileAllAccess windows.ACCESS_MASK = 0x001f01ff
	if sd == nil || expected == nil || !expected.IsValid() {
		return errors.New("invalid security descriptor or expected trustee")
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PRESENT == 0 || control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("protected DACL missing")
	}
	dacl, defaulted, err := sd.DACL()
	if err != nil || dacl == nil || defaulted || dacl.AceCount != 1 {
		return errors.New("DACL must contain exactly one explicit trustee")
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if windows.GetAce(dacl, 0, &ace) != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
		return errors.New("full-access allow ACE missing")
	}
	// Inherit-only or inherited entries must not replace the explicitly applied,
	// effective grant. OI/CI flags are permitted for the protected directory.
	const inheritance = windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE
	if ace.Header.AceFlags & ^uint8(inheritance) != 0 || ace.Mask != fileAllAccess {
		return errors.New("unexpected access mask or ACE flags")
	}
	trustee := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !trustee.IsValid() || !trustee.Equals(expected) {
		return errors.New("DACL trustee differs from current user")
	}
	return nil
}

func TestProtectedDACLUsesSIDSemanticsInsteadOfSDDLAliases(t *testing.T) {
	expected, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		t.Fatal(err)
	}
	for _, sddl := range []string{"D:P(A;OICI;FA;;;SY)", "D:P(A;OICI;FA;;;S-1-5-18)"} {
		sd, err := windows.SecurityDescriptorFromString(sddl)
		if err != nil {
			t.Fatal(err)
		}
		if err := protectedSingleUserDACL(sd, expected); err != nil {
			t.Fatal("equivalent alias/full SID descriptor rejected:", err)
		}
	}
}

func TestProtectedDACLRejectsBroadenedOrIneffectivePermissions(t *testing.T) {
	expected, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		t.Fatal(err)
	}
	for name, sddl := range map[string]string{
		"unprotected":   "D:(A;OICI;FA;;;SY)",
		"wrong trustee": "D:P(A;OICI;FA;;;BU)",
		"extra trustee": "D:P(A;OICI;FA;;;SY)(A;OICI;FR;;;BU)",
		"denied":        "D:P(D;OICI;FA;;;SY)",
		"read only":     "D:P(A;OICI;FR;;;SY)",
		"inherit only":  "D:P(A;OICIIO;FA;;;SY)",
		"empty DACL":    "D:P",
	} {
		t.Run(name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString(sddl)
			if err != nil {
				t.Fatal(err)
			}
			if protectedSingleUserDACL(sd, expected) == nil {
				t.Fatal("unsafe descriptor accepted")
			}
		})
	}
}
