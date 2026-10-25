package main

import (
	"bytes"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"ScreenCapturer/internal/capture"
	"ScreenCapturer/internal/server"
)

var (
	modUser32Benchmark   = syscall.NewLazyDLL("user32.dll")
	procGetGuiResources  = modUser32Benchmark.NewProc("GetGuiResources")
	modPsapi             = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMem    = modPsapi.NewProc("GetProcessMemoryInfo")
	modKernel32          = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentProc   = modKernel32.NewProc("GetCurrentProcess")
)

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

func getWorkingSetMB() float64 {
	hProc, _, _ := procGetCurrentProc.Call()
	var pmc processMemoryCounters
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	r, _, _ := procGetProcessMem.Call(hProc, uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.CB))
	if r == 0 {
		return 0
	}
	return float64(pmc.WorkingSetSize) / (1024 * 1024)
}

func getGDIHandles() (uint32, uint32) {
	hProc, _, _ := procGetCurrentProc.Call()
	gdi, _, _ := procGetGuiResources.Call(hProc, 0)  // GR_GDIOBJECTS = 0
	user, _, _ := procGetGuiResources.Call(hProc, 1) // GR_USEROBJECTS = 1
	return uint32(gdi), uint32(user)
}

func main() {
	capture.EnableDPIAwareness()
	capEngine := capture.NewGDICapturer()
	defer capEngine.Close()

	fmt.Println("==================================================")
	fmt.Println("       SCREENCAPTURER BENCHMARK SUITE             ")
	fmt.Println("==================================================")

	runtime.GC()
	initialGoroutines := runtime.NumGoroutine()
	initialGDI, initialUser := getGDIHandles()
	initialMem := getWorkingSetMB()

	fmt.Printf("Baseline: Goroutines=%d | GDI Handles=%d | User Handles=%d | Mem=%.2f MB\n\n",
		initialGoroutines, initialGDI, initialUser, initialMem)

	// Stage 1: Single request detailed latency comparison (PNG vs JPEG)
	fmt.Println("--- Single Request Stage Latency (1920x1080) ---")

	// 1. Capture
	t0 := time.Now()
	img, err := capEngine.Capture()
	capDur := time.Since(t0)
	if err != nil {
		fmt.Printf("Capture failed: %v\n", err)
		return
	}
	fmt.Printf("Capture Acquisition Time : %.2f ms (Bounds: %dx%d)\n",
		float64(capDur.Microseconds())/1000.0, img.Bounds().Dx(), img.Bounds().Dy())

	// 2. PNG Encode (BestSpeed)
	var pngBuf bytes.Buffer
	tEncPNG := time.Now()
	_, _ = server.EncodeImage(&pngBuf, img, "png", 0)
	pngEncDur := time.Since(tEncPNG)
	fmt.Printf("PNG BestSpeed Encode     : %.2f ms | Size: %d KB\n",
		float64(pngEncDur.Microseconds())/1000.0, pngBuf.Len()/1024)

	// 3. JPEG Encode (Quality 90)
	var jpgBuf bytes.Buffer
	tEncJPG := time.Now()
	_, _ = server.EncodeImage(&jpgBuf, img, "jpeg", 90)
	jpgEncDur := time.Since(tEncJPG)
	fmt.Printf("JPEG Quality 90 Encode   : %.2f ms | Size: %d KB\n",
		float64(jpgEncDur.Microseconds())/1000.0, jpgBuf.Len()/1024)

	// Stage 2: 100 Sequential Captures Stress Test
	fmt.Println("\n--- Running 100 Sequential Captures (50 PNG + 50 JPEG) ---")
	var peakMem float64
	var totalCapTime time.Duration
	var totalEncodeTime time.Duration

	start100 := time.Now()
	for i := 1; i <= 100; i++ {
		cStart := time.Now()
		cImg, cErr := capEngine.Capture()
		if cErr != nil {
			fmt.Printf("Error at capture %d: %v\n", i, cErr)
			continue
		}
		totalCapTime += time.Since(cStart)

		var buf bytes.Buffer
		fmtChoice := "png"
		if i%2 == 0 {
			fmtChoice = "jpeg"
		}

		eStart := time.Now()
		_, _ = server.EncodeImage(&buf, cImg, fmtChoice, 90)
		totalEncodeTime += time.Since(eStart)

		curMem := getWorkingSetMB()
		if curMem > peakMem {
			peakMem = curMem
		}

		cImg = nil
		if i%25 == 0 {
			fmt.Printf("Completed %d/100 captures... (Current Working Set: %.2f MB)\n", i, curMem)
		}
	}
	total100Dur := time.Since(start100)

	// Force GC and free OS memory after stress test
	runtime.GC()
	finalMem := getWorkingSetMB()
	finalGoroutines := runtime.NumGoroutine()
	finalGDI, finalUser := getGDIHandles()

	fmt.Println("\n==================================================")
	fmt.Println("              BENCHMARK RESULTS SUMMARY           ")
	fmt.Println("==================================================")
	fmt.Printf("100 Captures Completed In : %.2f s (Avg %.2f ms/capture)\n",
		total100Dur.Seconds(), float64(total100Dur.Milliseconds())/100.0)
	fmt.Printf("Average Capture Step Time : %.2f ms\n", float64(totalCapTime.Milliseconds())/100.0)
	fmt.Printf("Average Encode Step Time  : %.2f ms\n", float64(totalEncodeTime.Milliseconds())/100.0)
	fmt.Printf("Baseline Working Set Mem  : %.2f MB\n", initialMem)
	fmt.Printf("Peak Working Set Mem      : %.2f MB\n", peakMem)
	fmt.Printf("Final Working Set Mem     : %.2f MB\n", finalMem)
	fmt.Printf("Goroutine Leak Check      : %d -> %d (Delta: %+d)\n",
		initialGoroutines, finalGoroutines, finalGoroutines-initialGoroutines)
	fmt.Printf("GDI Handle Leak Check     : %d -> %d (Delta: %+d)\n",
		initialGDI, finalGDI, int(finalGDI)-int(initialGDI))
	fmt.Printf("User Handle Leak Check    : %d -> %d (Delta: %+d)\n",
		initialUser, finalUser, int(finalUser)-int(initialUser))
	fmt.Println("==================================================")
}
