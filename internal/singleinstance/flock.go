//go:build darwin || linux

package singleinstance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var (
	ErrAlreadyRunning = errors.New("another instance of ScreenCapturer is already running")
)

// Guard holds the file lock handle.
type Guard struct {
	file *os.File
}

// Acquire creates and locks a PID lockfile in the user's temp directory.
func Acquire(name string) (*Guard, error) {
	lockPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.lock", name))
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lockfile: %w", err)
	}

	// Non-blocking exclusive lock
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		return nil, ErrAlreadyRunning
	}

	return &Guard{file: f}, nil
}

// Release unlocks and removes the lock file.
func (g *Guard) Release() {
	if g != nil && g.file != nil {
		_ = syscall.Flock(int(g.file.Fd()), syscall.LOCK_UN)
		_ = g.file.Close()
		_ = os.Remove(g.file.Name())
	}
}
