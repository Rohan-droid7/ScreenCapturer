package tray

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"fyne.io/systray"

	"ScreenCapturer/internal/config"
	"ScreenCapturer/internal/logger"
	"ScreenCapturer/internal/server"
)

// AppTray coordinates systray menu items and server lifecycle.
type AppTray struct {
	cfg        *config.Config
	srv        *server.Server
	autoStopMu sync.Mutex
	autoTimer  *time.Timer
	userExited bool
}

// NewAppTray constructs an AppTray instance.
func NewAppTray(cfg *config.Config, srv *server.Server) *AppTray {
	return &AppTray{
		cfg: cfg,
		srv: srv,
	}
}

// Run starts the systray main event loop (blocking).
func (at *AppTray) Run() {
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
	mCopy := systray.AddMenuItem("Copy Phone Address", "Copy mobile pairing URL to clipboard")
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
				localURL := fmt.Sprintf("http://127.0.0.1:%d/#token=%s", at.cfg.Port, at.cfg.GetToken())
				openBrowser(localURL)

			case <-mCopy.ClickedCh:
				lanIP := server.GetPrimaryLANIP()
				pairingURL := fmt.Sprintf("http://%s:%d/#token=%s", lanIP, at.cfg.Port, at.cfg.GetToken())
				copyToClipboard(pairingURL)

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
	_ = GenerateAndOpenPairingPage(at.cfg)
}

func (at *AppTray) onExit() {
	if at.userExited {
		logger.Get().Infof("Exiting via system tray...")
		_ = at.srv.Stop()
	}
}

func openBrowser(url string) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Start-Process", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

func copyToClipboard(text string) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard", "-Value", fmt.Sprintf("'%s'", text))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}
