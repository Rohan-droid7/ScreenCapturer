package server

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func createTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 255) / w),
				G: uint8((y * 255) / h),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}

func TestEncodeImage(t *testing.T) {
	img := createTestImage(100, 100)

	tests := []struct {
		name         string
		format       string
		quality      int
		expectedMIME string
		expectErr    bool
		magicBytes   []byte
	}{
		{
			name:         "Default PNG",
			format:       "png",
			quality:      90,
			expectedMIME: "image/png",
			expectErr:    false,
			magicBytes:   []byte{0x89, 'P', 'N', 'G'},
		},
		{
			name:         "Empty format defaults to PNG",
			format:       "",
			quality:      90,
			expectedMIME: "image/png",
			expectErr:    false,
			magicBytes:   []byte{0x89, 'P', 'N', 'G'},
		},
		{
			name:         "JPEG format",
			format:       "jpeg",
			quality:      90,
			expectedMIME: "image/jpeg",
			expectErr:    false,
			magicBytes:   []byte{0xFF, 0xD8},
		},
		{
			name:         "JPG alias format",
			format:       "jpg",
			quality:      80,
			expectedMIME: "image/jpeg",
			expectErr:    false,
			magicBytes:   []byte{0xFF, 0xD8},
		},
		{
			name:         "Invalid format error",
			format:       "webp",
			quality:      90,
			expectedMIME: "",
			expectErr:    true,
			magicBytes:   nil,
		},
		{
			name:         "Case insensitive PNG",
			format:       "PNG",
			quality:      90,
			expectedMIME: "image/png",
			expectErr:    false,
			magicBytes:   []byte{0x89, 'P', 'N', 'G'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			mime, err := EncodeImage(&buf, img, tt.format, tt.quality)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error for format %q, got nil", tt.format)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if mime != tt.expectedMIME {
				t.Errorf("expected MIME %q, got %q", tt.expectedMIME, mime)
			}

			if len(tt.magicBytes) > 0 {
				data := buf.Bytes()
				if len(data) < len(tt.magicBytes) {
					t.Fatalf("encoded output too short: %d bytes", len(data))
				}
				if !bytes.Equal(data[:len(tt.magicBytes)], tt.magicBytes) {
					t.Errorf("magic byte mismatch: got %x, want %x", data[:len(tt.magicBytes)], tt.magicBytes)
				}
			}
		})
	}
}

func TestEncodeJPEGSizeVsQuality(t *testing.T) {
	img := createTestImage(200, 200)

	var bufLow, bufHigh bytes.Buffer
	_, err := EncodeImage(&bufLow, img, "jpeg", 30)
	if err != nil {
		t.Fatalf("low quality encode failed: %v", err)
	}

	_, err = EncodeImage(&bufHigh, img, "jpeg", 95)
	if err != nil {
		t.Fatalf("high quality encode failed: %v", err)
	}

	if bufLow.Len() >= bufHigh.Len() {
		t.Errorf("expected low quality JPEG (%d bytes) to be smaller than high quality (%d bytes)",
			bufLow.Len(), bufHigh.Len())
	}
}
