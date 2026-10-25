//go:build !windows

package server

import (
	"os/exec"
)

// OpenBrowser opens a URL using the OS default launcher.
func OpenBrowser(target string) {
	_ = exec.Command("open", target).Start()
}
