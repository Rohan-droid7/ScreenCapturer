//go:build windows

package singleinstance

import (
	"errors"
	"syscall"
	"unsafe"
)

var (
	modKernel32      = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW = modKernel32.NewProc("CreateMutexW")
	procCloseHandle  = modKernel32.NewProc("CloseHandle")

	ErrAlreadyRunning = errors.New("another instance of ScreenCapturer is already running")
)

const (
	errorAlreadyExists = 183
)

// Guard holds the Windows mutex handle to prevent multiple instances.
type Guard struct {
	handle uintptr
}

// Acquire attempts to acquire a named Windows mutex. Returns ErrAlreadyRunning if taken.
func Acquire(name string) (*Guard, error) {
	nameUTF16, err := syscall.UTF16PtrFromString("Local\\" + name)
	if err != nil {
		return nil, err
	}

	h, _, errCall := procCreateMutexW.Call(
		0,
		0, // initial owner = false
		uintptr(unsafe.Pointer(nameUTF16)),
	)
	if h == 0 {
		return nil, errCall
	}

	errno, ok := errCall.(syscall.Errno)
	if ok && errno == errorAlreadyExists {
		_, _, _ = procCloseHandle.Call(h)
		return nil, ErrAlreadyRunning
	}

	return &Guard{handle: h}, nil
}

// Release closes the mutex handle.
func (g *Guard) Release() {
	if g.handle != 0 {
		_, _, _ = procCloseHandle.Call(g.handle)
		g.handle = 0
	}
}
