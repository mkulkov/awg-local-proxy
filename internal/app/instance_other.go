//go:build !windows

package app

type Instance struct{}

func AcquireInstance() (*Instance, bool, error) { return &Instance{}, false, nil }
func (*Instance) Watch(func())                  {}
func (*Instance) Close()                        {}
