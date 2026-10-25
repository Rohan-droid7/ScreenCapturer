//go:build windows

package server

import (
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	modShell32       = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

// OpenBrowser opens a local file or URL in the default browser using the native Windows ShellExecuteW API.
func OpenBrowser(target string) {
	verb, errVerb := syscall.UTF16PtrFromString("open")
	file, errFile := syscall.UTF16PtrFromString(target)
	if errVerb == nil && errFile == nil {
		ret, _, _ := procShellExecuteW.Call(
			0,
			uintptr(unsafe.Pointer(verb)),
			uintptr(unsafe.Pointer(file)),
			0,
			0,
			1, // SW_SHOWNORMAL
		)
		if ret > 32 {
			return
		}
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
}
