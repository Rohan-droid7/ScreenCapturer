//go:build windows

package main

import "syscall"

var (
	modKernel32       = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = modKernel32.NewProc("AttachConsole")
)

const attachParentProcess = ^uintptr(0)

func attachConsoleIfRequested() {
	_, _, _ = procAttachConsole.Call(attachParentProcess)
}
