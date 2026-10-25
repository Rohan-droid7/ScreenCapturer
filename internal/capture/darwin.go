//go:build darwin

package capture

import (
	"fmt"
	"image"
	"reflect"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CGPoint matches CoreGraphics CGPoint
type CGPoint struct {
	X float64
	Y float64
}

// CGSize matches CoreGraphics CGSize
type CGSize struct {
	Width  float64
	Height float64
}

// CGRect matches CoreGraphics CGRect
type CGRect struct {
	Origin CGPoint
	Size   CGSize
}

type CGDirectDisplayID uint32
type CGWindowListOption uint32
type CGWindowImageOption uint32

const (
	kCGWindowListOptionOnScreenOnly CGWindowListOption  = 1 << 0
	kCGNullWindowID                 uint32              = 0
	kCGWindowImageDefault           CGWindowImageOption = 0
)

var (
	cgMainDisplayID          func() CGDirectDisplayID
	cgDisplayBounds          func(display CGDirectDisplayID) CGRect
	cgWindowListCreateImage  func(bounds CGRect, listOption CGWindowListOption, windowID uint32, imageOption CGWindowImageOption) uintptr
	cgImageGetWidth          func(image uintptr) int
	cgImageGetHeight         func(image uintptr) int
	cgImageGetBytesPerRow    func(image uintptr) int
	cgImageGetBitsPerPixel   func(image uintptr) int
	cgImageGetDataProvider   func(image uintptr) uintptr
	cgDataProviderCopyData   func(provider uintptr) uintptr
	cfDataGetLength          func(theData uintptr) int
	cfDataGetBytePtr         func(theData uintptr) uintptr
	cfRelease                func(cf uintptr)
	darwinLoaded             bool
)

func initDarwinCoreGraphics() error {
	if darwinLoaded {
		return nil
	}

	cgLib, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("failed to open CoreGraphics.framework: %w", err)
	}

	cfLib, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("failed to open CoreFoundation.framework: %w", err)
	}

	purego.RegisterLibFunc(&cgMainDisplayID, cgLib, "CGMainDisplayID")
	purego.RegisterLibFunc(&cgDisplayBounds, cgLib, "CGDisplayBounds")
	purego.RegisterLibFunc(&cgWindowListCreateImage, cgLib, "CGWindowListCreateImage")
	purego.RegisterLibFunc(&cgImageGetWidth, cgLib, "CGImageGetWidth")
	purego.RegisterLibFunc(&cgImageGetHeight, cgLib, "CGImageGetHeight")
	purego.RegisterLibFunc(&cgImageGetBytesPerRow, cgLib, "CGImageGetBytesPerRow")
	purego.RegisterLibFunc(&cgImageGetBitsPerPixel, cgLib, "CGImageGetBitsPerPixel")
	purego.RegisterLibFunc(&cgImageGetDataProvider, cgLib, "CGImageGetDataProvider")
	purego.RegisterLibFunc(&cgDataProviderCopyData, cgLib, "CGDataProviderCopyData")

	purego.RegisterLibFunc(&cfDataGetLength, cfLib, "CFDataGetLength")
	purego.RegisterLibFunc(&cfDataGetBytePtr, cfLib, "CFDataGetBytePtr")
	purego.RegisterLibFunc(&cfRelease, cfLib, "CFRelease")

	darwinLoaded = true
	return nil
}

// DarwinCoreGraphicsCapturer captures macOS screens natively via CoreGraphics without invoking any screenshot apps or UI.
type DarwinCoreGraphicsCapturer struct{}

// NewDarwinCapturer creates a new instance of macOS CoreGraphics capturer.
func NewDarwinCapturer() (*DarwinCoreGraphicsCapturer, error) {
	if err := initDarwinCoreGraphics(); err != nil {
		return nil, err
	}
	return &DarwinCoreGraphicsCapturer{}, nil
}

func (c *DarwinCoreGraphicsCapturer) Name() string {
	return "coregraphics"
}

func (c *DarwinCoreGraphicsCapturer) Close() error {
	return nil
}

// Capture grabs the primary monitor image and returns RGBA pixel data.
func (c *DarwinCoreGraphicsCapturer) Capture() (*image.RGBA, error) {
	if err := initDarwinCoreGraphics(); err != nil {
		return nil, err
	}

	mainDisplay := cgMainDisplayID()
	bounds := cgDisplayBounds(mainDisplay)
	if bounds.Size.Width <= 0 || bounds.Size.Height <= 0 {
		return nil, ErrDisplayUnavailable
	}

	// Capture display using CGWindowListCreateImage
	// Screen Recording permission is required by macOS; if denied, it returns 0.
	cgImage := cgWindowListCreateImage(bounds, kCGWindowListOptionOnScreenOnly, kCGNullWindowID, kCGWindowImageDefault)
	if cgImage == 0 {
		return nil, fmt.Errorf("%w: macOS denied capture (ensure Screen Recording permission is enabled in System Settings)", ErrCaptureFailed)
	}
	defer cfRelease(cgImage)

	width := cgImageGetWidth(cgImage)
	height := cgImageGetHeight(cgImage)
	bytesPerRow := cgImageGetBytesPerRow(cgImage)
	bpp := cgImageGetBitsPerPixel(cgImage)

	if width <= 0 || height <= 0 || bpp != 32 {
		return nil, fmt.Errorf("unsupported image format: %dx%d, %d bpp", width, height, bpp)
	}

	provider := cgImageGetDataProvider(cgImage)
	if provider == 0 {
		return nil, fmt.Errorf("failed to get CGImage data provider")
	}

	cfData := cgDataProviderCopyData(provider)
	if cfData == 0 {
		return nil, fmt.Errorf("failed to copy data from CGDataProvider")
	}
	defer cfRelease(cfData)

	dataLen := cfDataGetLength(cfData)
	bytePtr := cfDataGetBytePtr(cfData)
	if bytePtr == 0 || dataLen <= 0 {
		return nil, fmt.Errorf("invalid pixel data pointer")
	}

	// Construct Go byte slice from raw pointer
	var rawBytes []byte
	header := (*reflect.SliceHeader)(unsafe.Pointer(&rawBytes))
	header.Data = bytePtr
	header.Len = dataLen
	header.Cap = dataLen

	// Allocate target RGBA image
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Copy and convert BGRA/ARGB to RGBA
	// On macOS CoreGraphics with 32-bit little-endian, bytes are BGRA:
	// byte 0: Blue, byte 1: Green, byte 2: Red, byte 3: Alpha
	for y := 0; y < height; y++ {
		srcRow := y * bytesPerRow
		dstRow := y * img.Stride
		for x := 0; x < width; x++ {
			si := srcRow + x*4
			di := dstRow + x*4

			b := rawBytes[si+0]
			g := rawBytes[si+1]
			r := rawBytes[si+2]

			img.Pix[di+0] = r
			img.Pix[di+1] = g
			img.Pix[di+2] = b
			img.Pix[di+3] = 255 // Force solid alpha
		}
	}

	return img, nil
}

// EnableDPIAwareness is a no-op on macOS as CoreGraphics handles Retina/DPI natively
func EnableDPIAwareness() {}

// IsDesktopAvailable returns true on macOS
func IsDesktopAvailable() bool {
	return true
}
