package tray

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName        = "ScreenCapturer"
)

// IsAutostartEnabled checks if the HKCU Run key contains ScreenCapturer.
func IsAutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(appName)
	return err == nil && val != ""
}

// SetAutostartEnabled adds or removes the executable from HKCU Run key.
func SetAutostartEnabled(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}
		// Wrap in quotes in case of spaces in path
		cmd := fmt.Sprintf("\"%s\"", exePath)
		if err := k.SetStringValue(appName, cmd); err != nil {
			return fmt.Errorf("failed to write autostart registry entry: %w", err)
		}
	} else {
		_ = k.DeleteValue(appName)
	}
	return nil
}
