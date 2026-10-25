//go:build windows

package capture

// NewDefaultCapturer returns the native screen capture engine for Windows (GDI).
func NewDefaultCapturer() (Capturer, error) {
	return NewGDICapturer(), nil
}
