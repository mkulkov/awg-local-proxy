//go:build windows

package app

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	deviceNotifyCallback  = 2
	pbtAPMSuspend         = 4
	pbtAPMResumeSuspend   = 7
	pbtAPMResumeAutomatic = 18
)

var (
	powerDLL             = windows.NewLazySystemDLL("powrprof.dll")
	powerRegister        = powerDLL.NewProc("PowerRegisterSuspendResumeNotification")
	powerUnregister      = powerDLL.NewProc("PowerUnregisterSuspendResumeNotification")
	powerCallbacks       sync.Map
	powerID              atomic.Uintptr
	powerCallbackAddress = syscall.NewCallback(nativePowerCallback)
)

type nativePowerRegistration struct {
	callback   func(PowerEvent)
	active     atomic.Bool
	parameters struct{ callback, context uintptr }
	handle     uintptr
	id         uintptr
}

func nativePowerCallback(context, kind, setting uintptr) uintptr {
	value, ok := powerCallbacks.Load(context)
	if !ok {
		return 0
	}
	r := value.(*nativePowerRegistration)
	if !r.active.Load() {
		return 0
	}
	switch kind {
	case pbtAPMSuspend:
		r.callback(PowerSuspend)
	case pbtAPMResumeSuspend, pbtAPMResumeAutomatic:
		r.callback(PowerResume)
	}
	return 0
}

func registerPowerEvents(callback func(PowerEvent)) (func() error, error) {
	if err := powerRegister.Find(); err != nil {
		if errors.Is(err, windows.ERROR_PROC_NOT_FOUND) {
			return registerLegacyPowerEvents(callback)
		}
		return nil, err
	}
	if err := powerUnregister.Find(); err != nil {
		if errors.Is(err, windows.ERROR_PROC_NOT_FOUND) {
			return registerLegacyPowerEvents(callback)
		}
		return nil, err
	}
	r := &nativePowerRegistration{callback: callback, id: powerID.Add(1)}
	if r.id == 0 {
		return nil, windows.ERROR_NOT_ENOUGH_MEMORY
	}
	r.parameters.callback, r.parameters.context = powerCallbackAddress, r.id
	r.active.Store(true)
	powerCallbacks.Store(r.id, r)
	result, _, _ := powerRegister.Call(deviceNotifyCallback, uintptr(unsafe.Pointer(&r.parameters)), uintptr(unsafe.Pointer(&r.handle)))
	runtime.KeepAlive(r)
	if result != 0 {
		powerCallbacks.Delete(r.id)
		return nil, syscall.Errno(result)
	}
	var once sync.Once
	var unregisterErr error
	return func() error {
		once.Do(func() {
			// A late callback must become a no-op even if native unregister
			// fails. Retain its immutable parameters until process exit in that
			// exceptional case, so Windows cannot hold a dangling recipient.
			r.active.Store(false)
			result, _, _ := powerUnregister.Call(r.handle)
			runtime.KeepAlive(r)
			if result != 0 {
				unregisterErr = syscall.Errno(result)
			} else {
				powerCallbacks.Delete(r.id)
			}
		})
		return unregisterErr
	}, nil
}
