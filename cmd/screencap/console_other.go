//go:build !windows

package main

func attachConsoleIfRequested() {
	// No-op on Unix/macOS
}
