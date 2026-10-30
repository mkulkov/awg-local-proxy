//go:build windows

package profiles

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type DPAPI struct{}
type blob struct {
	size uint32
	data *byte
}

var crypt32 = windows.NewLazySystemDLL("crypt32.dll")
var protectData = crypt32.NewProc("CryptProtectData")
var unprotectData = crypt32.NewProc("CryptUnprotectData")

func crypt(input []byte, decrypt bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("empty DPAPI input")
	}
	in := blob{uint32(len(input)), &input[0]}
	var out blob
	proc := protectData
	if decrypt {
		proc = unprotectData
	}
	// UI_FORBIDDEN only: never CRYPTPROTECT_LOCAL_MACHINE.
	ok, _, err := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(input)
	if ok == 0 {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.data)))
	if out.size > maxSecretBytes+4096 {
		return nil, errors.New("DPAPI output too large")
	}
	return append([]byte(nil), unsafe.Slice(out.data, int(out.size))...), nil
}
func (DPAPI) Protect(b []byte) ([]byte, error)   { return crypt(b, false) }
func (DPAPI) Unprotect(b []byte) ([]byte, error) { return crypt(b, true) }

func restrict(path string) error {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + u.User.Sid.String() + ")")
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}
