//go:build windows

package tray

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/systray"

	"ScreenCapturer/internal/config"
	"ScreenCapturer/internal/logger"
	"ScreenCapturer/internal/server"
	"ScreenCapturer/internal/tunnel"
)

// AppTray coordinates systray menu items and server lifecycle.
type AppTray struct {
	cfg        *config.Config
	srv        *server.Server
	tunnelMgr  *tunnel.Manager
	autoStopMu sync.Mutex
	autoTimer  *time.Timer
	userExited bool
}

// NewAppTray constructs an AppTray instance.
func NewAppTray(cfg *config.Config, srv *server.Server, tunnelMgr *tunnel.Manager) *AppTray {
	return &AppTray{
		cfg:       cfg,
		srv:       srv,
		tunnelMgr: tunnelMgr,
	}
}

// Run starts the systray main event loop (blocking).
func (at *AppTray) Run() {
	// Ensure the GUI thread is attached to the active input desktop so Shell_NotifyIcon interacts with Explorer taskbar
	modUser32 := syscall.NewLazyDLL("user32.dll")
	pOpenInputDesktop := modUser32.NewProc("OpenInputDesktop")
	pSetThreadDesktop := modUser32.NewProc("SetThreadDesktop")
	pCloseDesktop := modUser32.NewProc("CloseDesktop")

	if pOpenInputDesktop.Find() == nil && pSetThreadDesktop.Find() == nil {
		hDesk, _, _ := pOpenInputDesktop.Call(0, 0, 0x01FF)
		if hDesk != 0 {
			_, _, _ = pSetThreadDesktop.Call(hDesk)
			if pCloseDesktop.Find() == nil {
				_, _, _ = pCloseDesktop.Call(hDesk)
			}
		}
	}

	systray.Run(at.onReady, at.onExit)

	// If systray exited without an explicit user "Exit" click (e.g. headless or sandbox desktop without shell notify icon)
	if !at.userExited {
		logger.Get().Warnf("Systray unavailable on current desktop context; continuing in background server mode.")
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		_ = at.srv.Stop()
	}
}

func (at *AppTray) onReady() {
	systray.SetIcon(GenerateTrayIcon())
	systray.SetTitle("ScreenCapturer")
	systray.SetTooltip("ScreenCapturer - Active")

	mStatus := systray.AddMenuItem("● Status: Serving on :"+fmt.Sprint(at.cfg.Port), "Server Status")
	mStatus.Disable()

	systray.AddSeparator()

	mToggle := systray.AddMenuItem("Stop Server", "Pause or resume the HTTP server")
	mOpen := systray.AddMenuItem("Open Control Page", "Open local control page in default browser")
	mCopyLAN := systray.AddMenuItem("Copy Wi-Fi Address (LAN)", "Copy local Wi-Fi pairing URL to clipboard")
	mCopyRemote := systray.AddMenuItem("Copy Remote Address (Cellular / WAN)", "Copy Cloudflare public HTTPS URL")
	mToggleTunnel := systray.AddMenuItem("Toggle Remote Tunnel", "Enable or disable Cloudflare remote access")
	mShowQR := systray.AddMenuItem("Show Pairing QR Code", "View phone pairing QR code")
	mRegen := systray.AddMenuItem("Regenerate Token", "Generate a new secret token and invalidate old one")

	systray.AddSeparator()

	mAutostart := systray.AddMenuItemCheckbox("Start at Windows Logon", "Run automatically when you log into Windows", IsAutostartEnabled())

	systray.AddSeparator()

	mExit := systray.AddMenuItem("Exit", "Quit ScreenCapturer agent completely")

	// Setup auto-stop timer if configured
	at.resetAutoStopTimer(mStatus, mToggle)

	// Hook server capture callback to show toast and refresh idle timer
	at.srv.SetOnCaptureCallback(func() {
		ShowCaptureToast("")
		at.resetAutoStopTimer(mStatus, mToggle)
	})

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				if at.srv.IsRunning() {
					_ = at.srv.Stop()
					mStatus.SetTitle("○ Status: Server Stopped")
					mToggle.SetTitle("Start Server")
					systray.SetTooltip("ScreenCapturer - Stopped")
				} else {
					if err := at.srv.Start(); err == nil {
						mStatus.SetTitle("● Status: Serving on :" + fmt.Sprint(at.cfg.Port))
						mToggle.SetTitle("Stop Server")
						systray.SetTooltip("ScreenCapturer - Active")
						at.resetAutoStopTimer(mStatus, mToggle)
					}
				}

			case <-mOpen.ClickedCh:
				pairingURL := fmt.Sprintf("http://127.0.0.1:%d/pairing", at.cfg.Port)
				server.OpenBrowser(pairingURL)

			case <-mCopyLAN.ClickedCh:
				lanIP := server.GetPrimaryLANIP()
				pairingURL := fmt.Sprintf("http://%s:%d/#token=%s", lanIP, at.cfg.Port, at.cfg.GetToken())
				copyToClipboard(pairingURL)

			case <-mCopyRemote.ClickedCh:
				if at.tunnelMgr != nil && at.tunnelMgr.IsActive() {
					remoteURL := fmt.Sprintf("%s/#token=%s", at.tunnelMgr.GetPublicURL(), at.cfg.GetToken())
					copyToClipboard(remoteURL)
				} else {
					// Open pairing page to check/start tunnel
					at.showQRCodePage()
				}

			case <-mToggleTunnel.ClickedCh:
				if at.tunnelMgr != nil {
					_, _ = at.tunnelMgr.Toggle()
				}

			case <-mShowQR.ClickedCh:
				at.showQRCodePage()

			case <-mRegen.ClickedCh:
				newToken, err := at.cfg.RegenerateToken()
				if err == nil {
					logger.Get().Infof("Token regenerated from tray")
					lanIP := server.GetPrimaryLANIP()
					newURL := fmt.Sprintf("http://%s:%d/#token=%s", lanIP, at.cfg.Port, newToken)
					copyToClipboard(newURL)
					at.showQRCodePage()
				}

			case <-mAutostart.ClickedCh:
				enabled := !mAutostart.Checked()
				if err := SetAutostartEnabled(enabled); err == nil {
					if enabled {
						mAutostart.Check()
					} else {
						mAutostart.Uncheck()
					}
				}

			case <-mExit.ClickedCh:
				at.userExited = true
				systray.Quit()
				return
			}
		}
	}()
}

func (at *AppTray) resetAutoStopTimer(mStatus, mToggle *systray.MenuItem) {
	if at.cfg.AutoStopHours <= 0 {
		return
	}

	at.autoStopMu.Lock()
	defer at.autoStopMu.Unlock()

	if at.autoTimer != nil {
		at.autoTimer.Stop()
	}

	duration := time.Duration(at.cfg.AutoStopHours) * time.Hour
	at.autoTimer = time.AfterFunc(duration, func() {
		logger.Get().Infof("Auto-stop timer expired (%d hours). Stopping server.", at.cfg.AutoStopHours)
		_ = at.srv.Stop()
		mStatus.SetTitle("○ Status: Server Stopped (Idle Timeout)")
		mToggle.SetTitle("Start Server")
		systray.SetTooltip("ScreenCapturer - Stopped")
	})
}

func (at *AppTray) showQRCodePage() {
	_ = server.GenerateAndOpenPairingPage(at.cfg, at.tunnelMgr)
}

func (at *AppTray) onExit() {
	if at.userExited {
		logger.Get().Infof("Exiting via system tray...")
		_ = at.srv.Stop()
	}
}

// copyToClipboard pipes text into the built-in Windows clip.exe utility.
func copyToClipboard(text string) {
	cmd := exec.Command("cmd", "/c", "clip")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}
