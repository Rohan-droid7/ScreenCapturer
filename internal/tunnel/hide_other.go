//go:build !windows

package tunnel

import "os/exec"

func configureHideWindow(cmd *exec.Cmd) {
	// No-op on Unix/macOS
}
