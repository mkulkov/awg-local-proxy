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

// Windows 7/Server 2008 R2 delivers WM_POWERBROADCAST to top-level
// windows, but does not provide PowerRegisterSuspendResumeNotification.
const (
	wmPowerBroadcast = 0x0218
	wmClose          = 0x0010
	wmDestroy        = 0x0002
)

var (
	legacyUser32       = windows.NewLazySystemDLL("user32.dll")
	legacyKernel32     = windows.NewLazySystemDLL("kernel32.dll")
	legacyRegister     = legacyUser32.NewProc("RegisterClassExW")
	legacyCreate       = legacyUser32.NewProc("CreateWindowExW")
	legacyDefProc      = legacyUser32.NewProc("DefWindowProcW")
	legacyDestroy      = legacyUser32.NewProc("DestroyWindow")
	legacyPostQuit     = legacyUser32.NewProc("PostQuitMessage")
	legacyPostMessage  = legacyUser32.NewProc("PostMessageW")
	legacyGetMessage   = legacyUser32.NewProc("GetMessageW")
	legacyDispatch     = legacyUser32.NewProc("DispatchMessageW")
	legacyModule       = legacyKernel32.NewProc("GetModuleHandleW")
	legacyWindowProc   = syscall.NewCallback(legacyPowerWindowProc)
	legacyClassName, _ = windows.UTF16PtrFromString("AwgLocalProxyPowerEventsV1")
	legacyClassOnce    sync.Once
	legacyClassErr     error
	legacyWindows      sync.Map // HWND -> *legacyPowerRegistration
)

type legacyWindowClass struct {
	size, style                        uint32
	windowProc                         uintptr
	classExtra, winExtra               int32
	instance, icon, cursor, background uintptr
	menuName, className                *uint16
	smallIcon                          uintptr
}

type legacyMessage struct {
	hwnd           uintptr
	message        uint32
	wparam, lparam uintptr
	time           uint32
	x, y           int32
	private        uint32
}

type legacyPowerRegistration struct {
	callback func(PowerEvent)
	active   atomic.Bool
	hwnd     uintptr
	done     chan struct{}
	loopErr  error // read only after done closes
}

func legacyPowerWindowProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	value, ok := legacyWindows.Load(hwnd)
	if ok {
		r := value.(*legacyPowerRegistration)
		if msg == wmPowerBroadcast && r.active.Load() {
			switch wparam {
			case pbtAPMSuspend:
				r.callback(PowerSuspend)
			case pbtAPMResumeSuspend, pbtAPMResumeAutomatic:
				r.callback(PowerResume)
			}
			return 1
		}
		if msg == wmClose {
			legacyDestroy.Call(hwnd)
			return 0
		}
		if msg == wmDestroy {
			legacyPostQuit.Call(0)
			return 0
		}
	}
	result, _, _ := legacyDefProc.Call(hwnd, uintptr(msg), wparam, lparam)
	return result
}

func registerLegacyPowerEvents(callback func(PowerEvent)) (func() error, error) {
	legacyClassOnce.Do(func() {
		instance, _, err := legacyModule.Call(0)
		if instance == 0 {
			legacyClassErr = err
			return
		}
		class := legacyWindowClass{size: uint32(unsafe.Sizeof(legacyWindowClass{})), windowProc: legacyWindowProc, instance: instance, className: legacyClassName}
		atom, _, err := legacyRegister.Call(uintptr(unsafe.Pointer(&class)))
		runtime.KeepAlive(class)
		if atom == 0 {
			legacyClassErr = err
		}
	})
	if legacyClassErr != nil {
		return nil, legacyClassErr
	}
	r := &legacyPowerRegistration{callback: callback, done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(r.done)
		instance, _, _ := legacyModule.Call(0)
		hwnd, _, err := legacyCreate.Call(0, uintptr(unsafe.Pointer(legacyClassName)), uintptr(unsafe.Pointer(legacyClassName)), 0, 0, 0, 0, 0, 0, 0, instance, 0)
		if hwnd == 0 {
			ready <- err
			return
		}
		r.hwnd = hwnd
		r.active.Store(true)
		legacyWindows.Store(hwnd, r)
		ready <- nil
		var message legacyMessage
		for {
			result, _, err := legacyGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
			if int32(result) == -1 {
				r.loopErr = err
				break
			}
			if result == 0 {
				break
			}
			legacyDispatch.Call(uintptr(unsafe.Pointer(&message)))
		}
		r.active.Store(false)
		// If the message loop ended unexpectedly, destroy on its owning thread.
		// A second DestroyWindow after WM_CLOSE is harmless.
		legacyDestroy.Call(hwnd)
		legacyWindows.Delete(hwnd)
	}()
	if err := <-ready; err != nil {
		<-r.done
		return nil, err
	}
	var closeMu sync.Mutex
	return func() error {
		closeMu.Lock()
		defer closeMu.Unlock()
		r.active.Store(false)
		select {
		case <-r.done:
			return r.loopErr
		default:
		}
		result, _, err := legacyPostMessage.Call(r.hwnd, wmClose, 0, 0)
		if result == 0 {
			if err == syscall.Errno(0) {
				return errors.New("could not close legacy power notification window")
			}
			return err
		}
		<-r.done
		return r.loopErr
	}, nil
}
