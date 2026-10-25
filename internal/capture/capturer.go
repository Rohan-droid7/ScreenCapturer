package capture

import (
	"errors"
	"image"
)

var (
	ErrScreenLocked       = errors.New("screen is locked or secure desktop active")
	ErrDisplayUnavailable = errors.New("display device unavailable or sleeping")
	ErrCaptureFailed      = errors.New("underlying screen capture operation failed")
)

// Capturer captures primary screen frames on demand.
type Capturer interface {
	// Capture acquires the current display frame. Returns RGBA image.
	Capture() (*image.RGBA, error)
	// Name returns the identifier of this capture engine ("gdi", "dxgi").
	Name() string
	// Close releases any persistent resources.
	Close() error
}

// CaptureWithRetry captures a frame, retrying once on failure before returning an error.
func CaptureWithRetry(c Capturer) (*image.RGBA, error) {
	img, err := c.Capture()
	if err == nil {
		return img, nil
	}

	// Retry once
	img2, err2 := c.Capture()
	if err2 == nil {
		return img2, nil
	}

	return nil, err2
}
