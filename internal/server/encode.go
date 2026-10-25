package server

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported image format; use 'png' or 'jpeg'")
)

// EncodeImage encodes an image.RGBA directly to the provided writer in the requested format.
func EncodeImage(w io.Writer, img *image.RGBA, format string, jpegQuality int) (string, error) {
	fmtLower := strings.ToLower(strings.TrimSpace(format))
	if fmtLower == "" {
		fmtLower = "png"
	}

	switch fmtLower {
	case "png":
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := enc.Encode(w, img); err != nil {
			return "", fmt.Errorf("png encoding failed: %w", err)
		}
		return "image/png", nil

	case "jpeg", "jpg":
		if jpegQuality < 1 || jpegQuality > 100 {
			jpegQuality = 90
		}
		opt := &jpeg.Options{Quality: jpegQuality}
		if err := jpeg.Encode(w, img, opt); err != nil {
			return "", fmt.Errorf("jpeg encoding failed: %w", err)
		}
		return "image/jpeg", nil

	default:
		return "", ErrUnsupportedFormat
	}
}
