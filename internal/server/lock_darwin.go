//go:build darwin

package server

import (
	"os/exec"
)

// LockPC locks the workstation on macOS.
func LockPC() error {
	// Try CGSession -suspend (switches to login window, locks session)
	if err := exec.Command("/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession", "-suspend").Run(); err == nil {
		return nil
	}
	// Fallback: put display to sleep
	return exec.Command("pmset", "displaysleepnow").Run()
}
