package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"ScreenCapturer/internal/auth"
	"ScreenCapturer/internal/capture"
	"ScreenCapturer/internal/config"
	"ScreenCapturer/internal/logger"
	"ScreenCapturer/internal/server"
	"ScreenCapturer/internal/singleinstance"
	"ScreenCapturer/internal/tray"
)

var (
	modKernel32       = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = modKernel32.NewProc("AttachConsole")
)

const attachParentProcess = ^uintptr(0)

func main() {
	consoleMode := flag.Bool("console", false, "Run in interactive console mode instead of system tray")
	flag.Parse()

	if *consoleMode {
		// Attach to parent console if launched from terminal in GUI subsystem mode
		_, _, _ = procAttachConsole.Call(attachParentProcess)
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Get().Errorf("CRITICAL PANIC: %v\nStack: %s", r, debug.Stack())
		}
	}()

	// 1. Memory hygiene: 128 MB soft limit and active GC percent
	debug.SetMemoryLimit(128 * 1024 * 1024)
	debug.SetGCPercent(50)

	// 2. DPI Awareness
	capture.EnableDPIAwareness()

	// 3. Single-instance guard
	guard, err := singleinstance.Acquire("ScreenCapturerSingleInstance")
	if err != nil {
		// An instance is already running in the system tray!
		// Automatically open the pairing dashboard so the user gets instant visual feedback.
		if cfg, loadErr := config.LoadOrCreate(); loadErr == nil {
			_ = tray.GenerateAndOpenPairingPage(cfg)
		}
		if *consoleMode {
			fmt.Println("ScreenCapturer is already running in your system tray.")
		}
		os.Exit(0)
	}
	defer guard.Release()

	// 4. Configuration
	cfg, err := config.LoadOrCreate()
	if err != nil {
		if *consoleMode {
			fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		}
		os.Exit(1)
	}

	// 5. Logger (capped and rotating)
	logInstance, err := logger.InitGlobal("", cfg.MaxLogSizeMB, cfg.MaxLogBackups, *consoleMode)
	if err != nil {
		if *consoleMode {
			fmt.Fprintf(os.Stderr, "Logger error: %v\n", err)
		}
		os.Exit(1)
	}
	defer logInstance.Close()

	logInstance.Infof("Starting ScreenCapturer Agent (Port: %d, Engine: %s)", cfg.Port, cfg.CaptureEngine)

	// 6. Capture Engine & Auth
	gdiCap := capture.NewGDICapturer()
	defer gdiCap.Close()

	authMgr := auth.NewManager(cfg.GetToken, cfg.AllowedHosts, cfg.Port)

	// 7. HTTP Server
	srv := server.NewServer(cfg, gdiCap, authMgr)
	logInstance.Infof("Starting HTTP server on :%d...", cfg.Port)
	if err := srv.Start(); err != nil {
		logInstance.Errorf("Failed to start server: %v", err)
		if *consoleMode {
			fmt.Fprintf(os.Stderr, "Server start error: %v\n", err)
		}
		os.Exit(1)
	}
	logInstance.Infof("HTTP server successfully started.")

	// Automatically open the pairing dashboard so the user gets instant reassurance
	_ = tray.GenerateAndOpenPairingPage(cfg)

	lanIP := server.GetPrimaryLANIP()
	pairingURL := fmt.Sprintf("http://%s:%d/#token=%s", lanIP, cfg.Port, cfg.GetToken())
	localURL := fmt.Sprintf("http://127.0.0.1:%d/#token=%s", cfg.Port, cfg.GetToken())

	if *consoleMode {
		fmt.Println("================================================================")
		fmt.Println("                   SCREEN CAPTURER AGENT                        ")
		fmt.Println("================================================================")
		fmt.Printf(" Server Port      : %d\n", cfg.Port)
		fmt.Printf(" Capture Engine   : %s\n", gdiCap.Name())
		fmt.Printf(" Local URL        : %s\n", localURL)
		fmt.Printf(" Mobile Phone URL : %s\n", pairingURL)
		fmt.Println("----------------------------------------------------------------")
		fmt.Println(" Connect your phone to the same Wi-Fi and open the Mobile URL.   ")
		fmt.Println(" Press Ctrl+C in this console or click Turn Off on the page.    ")
		fmt.Println("================================================================")
	}

	// Listen for OS interrupt / terminate signals in background
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logInstance.Infof("Shutdown signal received.")
		srv.TriggerShutdown()
	}()

	// Launch systray in background goroutine (best effort for Windows shell)
	go func() {
		appTray := tray.NewAppTray(cfg, srv)
		appTray.Run()
	}()

	// Keep server alive until shutdown is requested (via Web UI, stop.bat, tray, or Ctrl+C)
	srv.WaitForShutdown()
	logInstance.Infof("ScreenCapturer server stopped.")
}
