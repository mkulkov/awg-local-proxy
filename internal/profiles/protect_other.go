//go:build !windows

package profiles

import "errors"

type DPAPI struct{}

func (DPAPI) Protect([]byte) ([]byte, error)   { return nil, errors.New("DPAPI requires Windows") }
func (DPAPI) Unprotect([]byte) ([]byte, error) { return nil, errors.New("DPAPI requires Windows") }
func restrict(string) error                    { return nil }
