//go:build windows

package main

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func showStartupError(message string) {
	text, err := windows.UTF16PtrFromString(message)
	if err != nil {
		return
	}
	title, err := windows.UTF16PtrFromString("AWG Local Proxy")
	if err != nil {
		return
	}
	messageBox := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	if messageBox.Find() != nil {
		return
	}
	messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
	runtime.KeepAlive(text)
	runtime.KeepAlive(title)
}
