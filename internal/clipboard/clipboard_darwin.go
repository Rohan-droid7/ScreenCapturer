//go:build darwin

package clipboard

import (
	"os/exec"
	"strings"
)

// Read retrieves text from macOS clipboard via pbpaste.
func Read() (string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Write sets text to macOS clipboard via pbcopy.
func Write(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
