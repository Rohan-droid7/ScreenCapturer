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

	// 1. Memory hygiene: 128 MB soft limit and active GC percent
	debug.SetMemoryLimit(128 * 1024 * 1024)
	debug.SetGCPercent(50)

	// 2. DPI Awareness
	capture.EnableDPIAwareness()

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

	// 3. Single-instance guard
	logInstance.Infof("Checking single-instance mutex...")
	guard, err := singleinstance.Acquire("ScreenCapturerSingleInstance")
	if err != nil {
		logInstance.Errorf("Single-instance check failed: %v", err)
		if *consoleMode {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
	defer guard.Release()

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
		fmt.Println(" Press Ctrl+C in this console to stop.                          ")
		fmt.Println("================================================================")

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		logInstance.Infof("Shutting down...")
		_ = srv.Stop()
		return
	}

	// Default: Run in Windows System Tray
	logInstance.Infof("Starting Windows System Tray...")
	appTray := tray.NewAppTray(cfg, srv)
	appTray.Run()
	logInstance.Infof("Windows System Tray exited.")
}
