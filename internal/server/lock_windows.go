//go:build windows

package server

import (
	"fmt"
	"syscall"
)

var (
	modUser32           = syscall.NewLazyDLL("user32.dll")
	procLockWorkStation = modUser32.NewProc("LockWorkStation")
)

// LockPC locks the Windows workstation (equivalent to Win + L).
func LockPC() error {
	ret, _, err := procLockWorkStation.Call()
	if ret == 0 {
		return fmt.Errorf("LockWorkStation failed: %w", err)
	}
	return nil
}
