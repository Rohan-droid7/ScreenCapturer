//go:build windows

package clipboard

import (
	"fmt"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard          = user32.NewProc("OpenClipboard")
	procCloseClipboard         = user32.NewProc("CloseClipboard")
	procEmptyClipboard         = user32.NewProc("EmptyClipboard")
	procGetClipboardData       = user32.NewProc("GetClipboardData")
	procSetClipboardData       = user32.NewProc("SetClipboardData")
	procIsClipboardFormatAvail = user32.NewProc("IsClipboardFormatAvailable")
	procGlobalAlloc            = kernel32.NewProc("GlobalAlloc")
	procGlobalLock             = kernel32.NewProc("GlobalLock")
	procGlobalUnlock           = kernel32.NewProc("GlobalUnlock")
	procGlobalFree             = kernel32.NewProc("GlobalFree")
	procRtlMoveMemory          = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// Read retrieves text from the Windows system clipboard.
func Read() (string, error) {
	var opened bool
	for i := 0; i < 5; i++ {
		r, _, _ := procOpenClipboard.Call(0)
		if r != 0 {
			opened = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		return "", fmt.Errorf("failed to open clipboard (resource locked by another app)")
	}
	defer procCloseClipboard.Call()

	avail, _, _ := procIsClipboardFormatAvail.Call(uintptr(cfUnicodeText))
	if avail == 0 {
		return "", ErrNoText
	}

	hMem, _, _ := procGetClipboardData.Call(uintptr(cfUnicodeText))
	if hMem == 0 {
		return "", nil
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return "", fmt.Errorf("failed to lock global memory for clipboard read")
	}
	defer procGlobalUnlock.Call(hMem)

	var u16 []uint16
	p := (*uint16)(unsafe.Pointer(ptr))
	for *p != 0 {
		u16 = append(u16, *p)
		p = (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 2))
		if len(u16) > 5000000 { // Max 10MB safety cap
			break
		}
	}
	return string(utf16.Decode(u16)), nil
}

// Write sets the text on the Windows system clipboard.
func Write(text string) error {
	encoded := utf16.Encode([]rune(text))
	u16 := make([]uint16, len(encoded)+1)
	copy(u16, encoded)
	u16[len(encoded)] = 0

	byteLen := uintptr(len(u16) * 2)
	hMem, _, _ := procGlobalAlloc.Call(uintptr(gmemMoveable), byteLen)
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed to allocate %d bytes", byteLen)
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("GlobalLock failed")
	}

	procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&u16[0])), byteLen)
	procGlobalUnlock.Call(hMem)

	var opened bool
	for i := 0; i < 5; i++ {
		r, _, _ := procOpenClipboard.Call(0)
		if r != 0 {
			opened = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("OpenClipboard failed (resource locked by another app)")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()
	r, _, err := procSetClipboardData.Call(uintptr(cfUnicodeText), hMem)
	if r == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("SetClipboardData failed: %w", err)
	}
	// Once SetClipboardData succeeds, the OS owns hMem.
	return nil
}
