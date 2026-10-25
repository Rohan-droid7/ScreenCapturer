//go:build darwin

package tray

import (
	"fmt"
	"os/exec"
)

// ShowCaptureToast triggers a native macOS notification via AppleScript when a remote capture succeeds.
func ShowCaptureToast(clientIP string) {
	go func() {
		script := fmt.Sprintf(`display notification "Remote phone captured the display" with title "ScreenCapturer" subtitle "Capture from %s"`, clientIP)
		_ = exec.Command("osascript", "-e", script).Run()
	}()
}
