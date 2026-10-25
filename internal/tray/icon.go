package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// GenerateTrayIcon creates a 32x32 clean camera/lens icon in PNG format for the system tray.
func GenerateTrayIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	blue := color.RGBA{R: 59, G: 130, B: 246, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	dark := color.RGBA{R: 15, G: 23, B: 42, A: 255}

	// Camera body rounded rectangle
	for y := 8; y <= 26; y++ {
		for x := 4; x <= 27; x++ {
			img.Set(x, y, blue)
		}
	}
	// Camera flash bump
	for y := 5; y <= 7; y++ {
		for x := 12; x <= 19; x++ {
			img.Set(x, y, blue)
		}
	}
	// Lens outer ring
	for y := 11; y <= 23; y++ {
		for x := 10; x <= 21; x++ {
			dx := float64(x - 15)
			dy := float64(y - 17)
			if dx*dx+dy*dy <= 30 {
				img.Set(x, y, dark)
			}
		}
	}
	// Lens center glass
	for y := 13; y <= 21; y++ {
		for x := 12; x <= 19; x++ {
			dx := float64(x - 15)
			dy := float64(y - 17)
			if dx*dx+dy*dy <= 12 {
				img.Set(x, y, white)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
