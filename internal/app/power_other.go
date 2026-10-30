//go:build !windows

package app

import "errors"

func registerPowerEvents(func(PowerEvent)) (func() error, error) {
	return nil, errors.New("Windows power notifications are unavailable")
}
