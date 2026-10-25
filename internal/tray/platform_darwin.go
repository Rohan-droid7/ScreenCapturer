//go:build darwin

package tray

import (
	"os/exec"
)

func openBrowser(url string) {
	_ = exec.Command("open", url).Start()
}

func copyToClipboard(text string) {
	cmd := exec.Command("pbcopy")
	in, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	_, _ = in.Write([]byte(text))
	_ = in.Close()
	_ = cmd.Wait()
}
