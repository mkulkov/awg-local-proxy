//go:build !windows

package autostart

func Get() (bool, error) { return false, ErrUnsupported }
func Enable() error      { return ErrUnsupported }
func Disable() error     { return ErrUnsupported }
