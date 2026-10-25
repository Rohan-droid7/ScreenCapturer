package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

// GenerateTrayIcon creates a valid Windows .ico file containing a 32x32 crisp camera icon.
func GenerateTrayIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	blue := color.RGBA{R: 59, G: 130, B: 246, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	dark := color.RGBA{R: 15, G: 23, B: 42, A: 255}

	// Camera body
	for y := 8; y <= 26; y++ {
		for x := 4; x <= 27; x++ {
			img.Set(x, y, blue)
		}
	}
	// Flash mount
	for y := 5; y <= 7; y++ {
		for x := 12; x <= 19; x++ {
			img.Set(x, y, blue)
		}
	}
	// Outer lens
	for y := 11; y <= 23; y++ {
		for x := 10; x <= 21; x++ {
			dx := float64(x - 15)
			dy := float64(y - 17)
			if dx*dx+dy*dy <= 30 {
				img.Set(x, y, dark)
			}
		}
	}
	// Lens reflection
	for y := 13; y <= 21; y++ {
		for x := 12; x <= 19; x++ {
			dx := float64(x - 15)
			dy := float64(y - 17)
			if dx*dx+dy*dy <= 12 {
				img.Set(x, y, white)
			}
		}
	}

	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)
	pngBytes := pngBuf.Bytes()

	// Build .ico format wrapping the PNG
	var ico bytes.Buffer
	// ICONDIR header (6 bytes)
	_ = binary.Write(&ico, binary.LittleEndian, uint16(0))      // Reserved
	_ = binary.Write(&ico, binary.LittleEndian, uint16(1))      // Type: 1 = ICO
	_ = binary.Write(&ico, binary.LittleEndian, uint16(1))      // Count: 1 image

	// ICONDIRENTRY (16 bytes)
	ico.WriteByte(byte(size))                                   // Width
	ico.WriteByte(byte(size))                                   // Height
	ico.WriteByte(0)                                            // Colors
	ico.WriteByte(0)                                            // Reserved
	_ = binary.Write(&ico, binary.LittleEndian, uint16(1))      // Color planes
	_ = binary.Write(&ico, binary.LittleEndian, uint16(32))     // Bits per pixel
	_ = binary.Write(&ico, binary.LittleEndian, uint32(len(pngBytes))) // Size of image data
	_ = binary.Write(&ico, binary.LittleEndian, uint32(22))     // Offset to image data (6+16=22)

	// Append PNG data
	ico.Write(pngBytes)

	return ico.Bytes()
}
