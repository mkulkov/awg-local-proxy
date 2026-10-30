//go:build windows

package app

import (
	"errors"
	"golang.org/x/sys/windows"
	"sync"
	"unsafe"
)

// Instance holds a per-user, per-session mutex and a bounded activation signal.
type Instance struct {
	mutex, event windows.Handle
	stop         chan struct{}
	once         sync.Once
	wg           sync.WaitGroup
}

func AcquireInstance() (*Instance, bool, error) {
	return acquireInstance("AwgLocalProxyDemo")
}

func acquireInstance(prefix string) (*Instance, bool, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return nil, false, err
	}
	defer token.Close()
	u, err := token.GetTokenUser()
	if err != nil {
		return nil, false, err
	}
	sid := u.User.Sid.String()
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, false, err
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	base := "Local\\" + prefix + "-" + sid
	eventName, _ := windows.UTF16PtrFromString(base + "-activate")
	event, err := windows.CreateEvent(sa, 0, 0, eventName)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, false, err
	}
	mutexName, _ := windows.UTF16PtrFromString(base + "-mutex")
	mutex, err := windows.CreateMutex(sa, false, mutexName)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.SetEvent(event)
		windows.CloseHandle(event)
		windows.CloseHandle(mutex)
		return nil, true, nil
	}
	if err != nil {
		windows.CloseHandle(event)
		return nil, false, err
	}
	return &Instance{mutex: mutex, event: event, stop: make(chan struct{})}, false, nil
}

func (i *Instance) Watch(show func()) {
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		for {
			select {
			case <-i.stop:
				return
			default:
			}
			result, err := windows.WaitForSingleObject(i.event, 250)
			if err != nil {
				return
			}
			if result == windows.WAIT_OBJECT_0 {
				show()
			}
		}
	}()
}
func (i *Instance) Close() {
	i.once.Do(func() {
		close(i.stop)
		i.wg.Wait()
		windows.CloseHandle(i.event)
		windows.CloseHandle(i.mutex)
	})
}
