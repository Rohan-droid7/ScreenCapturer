//go:build darwin

package capture

// NewDefaultCapturer returns the native screen capture engine for macOS (CoreGraphics).
func NewDefaultCapturer() (Capturer, error) {
	return NewDarwinCapturer()
}
