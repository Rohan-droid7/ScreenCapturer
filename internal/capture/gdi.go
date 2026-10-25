package capture

import (
	"fmt"
	"image"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	procGetDC                  = modUser32.NewProc("GetDC")
	procReleaseDC              = modUser32.NewProc("ReleaseDC")
	procGetSystemMetrics       = modUser32.NewProc("GetSystemMetrics")
	procOpenInputDesktop       = modUser32.NewProc("OpenInputDesktop")
	procSetThreadDesktop       = modUser32.NewProc("SetThreadDesktop")
	procCloseDesktop           = modUser32.NewProc("CloseDesktop")

	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procBitBlt                 = modGdi32.NewProc("BitBlt")
	procGetDIBits              = modGdi32.NewProc("GetDIBits")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
)

var modGdi32 = syscall.NewLazyDLL("gdi32.dll")

const (
	smCxScreen = 0
	smCyScreen = 1

	srccopy    = 0x00CC0020
	captureBlt = 0x40000000

	biRGB        = 0
	dibRGBColors = 0

	desktopAllAccess = 0x01FF
)

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]uint32
}

// GDICapturer implements the Capturer interface using Windows GDI.
type GDICapturer struct{}

// NewGDICapturer initializes a new GDI capturer.
func NewGDICapturer() *GDICapturer {
	return &GDICapturer{}
}

func (g *GDICapturer) Name() string {
	return "gdi"
}

func (g *GDICapturer) Close() error {
	return nil
}

// Capture grabs the primary monitor display via BitBlt and GetDIBits on the active input desktop.
func (g *GDICapturer) Capture() (*image.RGBA, error) {
	type result struct {
		img *image.RGBA
		err error
	}

	resCh := make(chan result, 1)

	// Run capture on a dedicated OS thread so SetThreadDesktop operates on a clean thread context
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// 1. Attach thread to active interactive input desktop
		hDesk, _, errDesk := procOpenInputDesktop.Call(0, 0, desktopAllAccess)
		if hDesk == 0 {
			resCh <- result{err: fmt.Errorf("%w: cannot access input desktop (err=%v)", ErrScreenLocked, errDesk)}
			return
		}
		defer procCloseDesktop.Call(hDesk)

		sRet, _, _ := procSetThreadDesktop.Call(hDesk)
		if sRet == 0 {
			// If setting thread desktop fails, still attempt capture on current desktop
		}

		// 2. Query primary monitor resolution
		rW, _, _ := procGetSystemMetrics.Call(uintptr(smCxScreen))
		rH, _, _ := procGetSystemMetrics.Call(uintptr(smCyScreen))
		width := int(rW)
		height := int(rH)
		if width <= 0 || height <= 0 {
			resCh <- result{err: ErrDisplayUnavailable}
			return
		}

		// 3. Acquire screen DC
		hdcScreen, _, _ := procGetDC.Call(0)
		if hdcScreen == 0 {
			resCh <- result{err: ErrDisplayUnavailable}
			return
		}
		defer procReleaseDC.Call(0, hdcScreen)

		// 4. Create memory DC and compatible bitmap
		hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
		if hdcMem == 0 {
			resCh <- result{err: fmt.Errorf("%w: CreateCompatibleDC failed", ErrCaptureFailed)}
			return
		}
		defer procDeleteDC.Call(hdcMem)

		hBitmap, _, _ := procCreateCompatibleBitmap.Call(hdcScreen, uintptr(width), uintptr(height))
		if hBitmap == 0 {
			resCh <- result{err: fmt.Errorf("%w: CreateCompatibleBitmap failed", ErrCaptureFailed)}
			return
		}
		defer procDeleteObject.Call(hBitmap)

		hOld, _, _ := procSelectObject.Call(hdcMem, hBitmap)
		if hOld == 0 {
			resCh <- result{err: fmt.Errorf("%w: SelectObject failed", ErrCaptureFailed)}
			return
		}
		defer procSelectObject.Call(hdcMem, hOld)

		// 5. BitBlt: first try with CAPTUREBLT (includes layered windows); fall back to SRCCOPY if denied
		bltRet, _, bltErr := procBitBlt.Call(
			hdcMem, 0, 0, uintptr(width), uintptr(height),
			hdcScreen, 0, 0,
			uintptr(srccopy|captureBlt),
		)
		if bltRet == 0 {
			// Fallback to SRCCOPY only
			bltRet, _, bltErr = procBitBlt.Call(
				hdcMem, 0, 0, uintptr(width), uintptr(height),
				hdcScreen, 0, 0,
				uintptr(srccopy),
			)
			if bltRet == 0 {
				resCh <- result{err: fmt.Errorf("%w: BitBlt failed: %v", ErrCaptureFailed, bltErr)}
				return
			}
		}

		// 6. Read pixel buffer with GetDIBits directly into RGBA image
		img := image.NewRGBA(image.Rect(0, 0, width, height))

		var bi bitmapInfo
		bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
		bi.BmiHeader.BiWidth = int32(width)
		bi.BmiHeader.BiHeight = -int32(height) // Negative height specifies top-down DIB
		bi.BmiHeader.BiPlanes = 1
		bi.BmiHeader.BiBitCount = 32
		bi.BmiHeader.BiCompression = biRGB
		bi.BmiHeader.BiSizeImage = uint32(width * height * 4)

		res, _, getErr := procGetDIBits.Call(
			hdcMem,
			hBitmap,
			0,
			uintptr(height),
			uintptr(unsafe.Pointer(&img.Pix[0])),
			uintptr(unsafe.Pointer(&bi)),
			uintptr(dibRGBColors),
		)
		if res == 0 {
			resCh <- result{err: fmt.Errorf("%w: GetDIBits failed: %v", ErrCaptureFailed, getErr)}
			return
		}

		// 7. Convert BGRA to RGBA in-place and set Alpha to 255
		pix := img.Pix
		for i := 0; i < len(pix); i += 4 {
			b := pix[i]
			r := pix[i+2]
			pix[i] = r
			// pix[i+1] is green, left unchanged
			pix[i+2] = b
			pix[i+3] = 255
		}

		resCh <- result{img: img}
	}()

	res := <-resCh
	return res.img, res.err
}
