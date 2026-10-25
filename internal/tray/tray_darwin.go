//go:build darwin

package tray

import (
	"time"

	"ScreenCapturer/internal/config"
	"ScreenCapturer/internal/logger"
	"ScreenCapturer/internal/server"
	"ScreenCapturer/internal/tunnel"
)

// AppTray coordinates application lifecycle on macOS.
// Automatically serves the pairing page and auto-stop timer cleanly.
type AppTray struct {
	cfg       *config.Config
	srv       *server.Server
	tunnelMgr *tunnel.Manager
	autoTimer *time.Timer
}

// NewAppTray constructs an AppTray instance.
func NewAppTray(cfg *config.Config, srv *server.Server, tunnelMgr *tunnel.Manager) *AppTray {
	return &AppTray{
		cfg:       cfg,
		srv:       srv,
		tunnelMgr: tunnelMgr,
	}
}

// Run starts the tray/dashboard coordination on macOS.
func (at *AppTray) Run() {
	logger.Get().Infof("macOS agent running. Dashboard available on port %d", at.cfg.Port)

	// Automatically open the pairing control page in the default macOS browser (Safari/Chrome)
	time.Sleep(200 * time.Millisecond)
	if at.srv.IsRunning() {
		_ = server.GenerateAndOpenPairingPage(at.cfg, at.tunnelMgr)
	}

	// Auto-stop monitor
	if at.cfg.AutoStopHours > 0 {
		dur := time.Duration(at.cfg.AutoStopHours) * time.Hour
		at.autoTimer = time.AfterFunc(dur, func() {
			logger.Get().Infof("Auto-stop timer elapsed after %d hours. Stopping server...", at.cfg.AutoStopHours)
			_ = at.srv.Stop()
		})
	}
}
