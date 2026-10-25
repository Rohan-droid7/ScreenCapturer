//go:build darwin

package tray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const plistLabel = "com.screencapturer.agent"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, plistLabel+".plist"), nil
}

// IsAutostartEnabled checks if the macOS LaunchAgent plist exists.
func IsAutostartEnabled() bool {
	p, err := plistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// SetAutostartEnabled creates or deletes a LaunchAgent plist for auto-starting at user login.
func SetAutostartEnabled(enabled bool) error {
	p, err := plistPath()
	if err != nil {
		return err
	}

	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`, plistLabel, exePath)

		if err := os.WriteFile(p, []byte(plistContent), 0644); err != nil {
			return fmt.Errorf("failed to write launch agent plist: %w", err)
		}
		_ = exec.Command("launchctl", "load", p).Run()
	} else {
		_ = exec.Command("launchctl", "unload", p).Run()
		_ = os.Remove(p)
	}
	return nil
}
