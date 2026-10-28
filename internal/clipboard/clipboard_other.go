//go:build !windows && !darwin

package clipboard

import (
	"errors"
	"os/exec"
	"strings"
)

// Read retrieves text from Linux clipboard using wl-paste or xclip or xsel.
func Read() (string, error) {
	if out, err := exec.Command("wl-paste", "--no-newline").Output(); err == nil {
		return string(out), nil
	}
	if out, err := exec.Command("xclip", "-selection", "clipboard", "-out").Output(); err == nil {
		return string(out), nil
	}
	if out, err := exec.Command("xsel", "--clipboard", "--output").Output(); err == nil {
		return string(out), nil
	}
	return "", errors.New("no supported clipboard utility found (please install wl-clipboard, xclip, or xsel)")
}

// Write sets text to Linux clipboard using wl-copy or xclip or xsel.
func Write(text string) error {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err == nil {
		return nil
	}

	cmd = exec.Command("xclip", "-selection", "clipboard", "-in")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err == nil {
		return nil
	}

	cmd = exec.Command("xsel", "--clipboard", "--input")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err == nil {
		return nil
	}

	return errors.New("no supported clipboard utility found (please install wl-clipboard, xclip, or xsel)")
}
