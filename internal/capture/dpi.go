//go:build windows

package capture

import (
	"syscall"
	"unsafe"
)

var (
	modUser32 = syscall.NewLazyDLL("user32.dll")
	modShcore = syscall.NewLazyDLL("shcore.dll")

	procSetProcessDpiAwarenessContext = modUser32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware           = modUser32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwareness        = modShcore.NewProc("SetProcessDpiAwareness")
)

const (
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is ((DPI_AWARENESS_CONTEXT)-4)
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3) // 0xFFFFFFFFFFFFFFFC on 64-bit
	processPerMonitorDpiAware           = 2
)

// EnableDPIAwareness ensures the process is DPI-aware so captures reflect physical desktop pixels.
func EnableDPIAwareness() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		r, _, _ := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
		if r != 0 {
			return
		}
	}

	if procSetProcessDpiAwareness.Find() == nil {
		r, _, _ := procSetProcessDpiAwareness.Call(uintptr(processPerMonitorDpiAware))
		if r == 0 { // S_OK
			return
		}
	}

	if procSetProcessDPIAware.Find() == nil {
		_, _, _ = procSetProcessDPIAware.Call()
	}
}

// IsDesktopAvailable checks whether the interactive input desktop is accessible.
// If Windows is locked, showing UAC secure desktop, or display is suspended, OpenInputDesktop fails.
func IsDesktopAvailable() bool {
	procOpenInputDesktop := modUser32.NewProc("OpenInputDesktop")
	procCloseDesktop := modUser32.NewProc("CloseDesktop")
	if procOpenInputDesktop.Find() != nil {
		return true // Fallback if API not located
	}

	const desktopSwitchDesktop = 0x0100
	hDesktop, _, _ := procOpenInputDesktop.Call(0, 0, desktopSwitchDesktop)
	if hDesktop == 0 {
		return false
	}
	if procCloseDesktop.Find() == nil {
		_, _, _ = procCloseDesktop.Call(hDesktop)
	}
	return true
}

func init() {
	_ = unsafe.Sizeof(0)
}
