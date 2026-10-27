//go:build !windows && !darwin

package server

import (
	"errors"
	"os/exec"
)

// LockPC locks the desktop session on Linux / Unix systems.
func LockPC() error {
	// Try loginctl lock-session (standard on systemd desktops: GNOME, KDE, etc.)
	if err := exec.Command("loginctl", "lock-session").Run(); err == nil {
		return nil
	}
	// Try xdg-screensaver lock (X11 desktop standard)
	if err := exec.Command("xdg-screensaver", "lock").Run(); err == nil {
		return nil
	}
	return errors.New("remote lock is not supported on this platform or no session lock utility found")
}
