package tunnel

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"syscall"
	"time"

	"ScreenCapturer/internal/config"
	"ScreenCapturer/internal/logger"
)

var (
	// Matches Cloudflare Quick Tunnel trycloudflare.com URLs
	tunnelURLRegex = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
)

// Status represents the current state of the Cloudflare tunnel.
type Status struct {
	Active     bool   `json:"active"`
	Connecting bool   `json:"connecting"`
	PublicURL  string `json:"public_url,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Manager orchestrates the lifecycle of the cloudflared quick tunnel process.
type Manager struct {
	mu           sync.RWMutex
	cmd          *exec.Cmd
	publicURL    string
	active       bool
	connecting   bool
	lastError    string
	port         int
	onURLFoundCb func(string)
	stopChan     chan struct{}
}

// NewManager creates an instance of the tunnel manager.
func NewManager(port int) *Manager {
	return &Manager{
		port: port,
	}
}

// SetOnURLFound registers a callback invoked when the public tunnel URL is discovered.
func (m *Manager) SetOnURLFound(cb func(string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onURLFoundCb = cb
}

// GetStatus returns the current status snapshot.
func (m *Manager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Status{
		Active:     m.active,
		Connecting: m.connecting,
		PublicURL:  m.publicURL,
		Error:      m.lastError,
	}
}

// GetPublicURL returns the active trycloudflare URL or empty string.
func (m *Manager) GetPublicURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.publicURL
}

// IsActive returns whether the tunnel is currently running.
func (m *Manager) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// getBinaryPath locates an existing cloudflared binary or returns the destination path where it should be downloaded.
func getBinaryPath() (string, error) {
	// 1. Check if cloudflared is installed in system PATH
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("cloudflared.exe"); err == nil {
		return path, nil
	}

	// 2. Check local AppData/ScreenCapturer/bin
	configPath, err := config.ConfigFilePath()
	if err != nil {
		return "", err
	}
	binDir := filepath.Join(filepath.Dir(configPath), "bin")
	binName := "cloudflared"
	if runtime.GOOS == "windows" {
		binName = "cloudflared.exe"
	}
	localBin := filepath.Join(binDir, binName)
	if _, err := os.Stat(localBin); err == nil {
		return localBin, nil
	}

	return localBin, nil
}

// EnsureBinary ensures cloudflared is present, automatically downloading the official release if missing.
func EnsureBinary() (string, error) {
	binPath, err := getBinaryPath()
	if err != nil {
		return "", err
	}

	// If binary already exists and is executable, use it
	if fi, err := os.Stat(binPath); err == nil && fi.Size() > 0 {
		return binPath, nil
	}

	binDir := filepath.Dir(binPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Determine download URL for the current platform
	var downloadURL string
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"
		case "arm64":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-arm64.exe"
		case "386":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-386.exe"
		default:
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"
		}
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-arm64.tgz"
		default:
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-amd64.tgz"
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64"
		case "arm64":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64"
		case "arm":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm"
		default:
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64"
		}
	default:
		return "", fmt.Errorf("unsupported operating system for cloudflared: %s", runtime.GOOS)
	}

	logger.Get().Infof("Downloading official Cloudflare tunnel binary from %s to %s...", downloadURL, binPath)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download cloudflared: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download cloudflared: HTTP %d", resp.StatusCode)
	}

	tmpPath := binPath + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write cloudflared binary: %w", err)
	}
	out.Close()

	if err := os.Rename(tmpPath, binPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to finalize cloudflared binary: %w", err)
	}

	logger.Get().Infof("Cloudflare tunnel binary successfully ready at %s", binPath)
	return binPath, nil
}

// Start spawns the cloudflared quick tunnel asynchronously.
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.active || m.connecting {
		m.mu.Unlock()
		return nil
	}
	m.connecting = true
	m.lastError = ""
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	go func() {
		binPath, err := EnsureBinary()
		if err != nil {
			m.mu.Lock()
			m.connecting = false
			m.active = false
			m.lastError = fmt.Sprintf("Failed to locate cloudflared: %v", err)
			m.mu.Unlock()
			logger.Get().Errorf("%s", m.lastError)
			return
		}

		targetURL := fmt.Sprintf("http://127.0.0.1:%d", m.port)
		// Run cloudflared quick tunnel without config or accounts
		cmd := exec.Command(binPath, "tunnel", "--url", targetURL, "--no-autoupdate")

		// Hide console window on Windows
		if runtime.GOOS == "windows" {
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		}

		// Read combined stderr/stdout since cloudflared prints tunnel info to stderr
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			m.mu.Lock()
			m.connecting = false
			m.lastError = fmt.Sprintf("Failed to acquire pipe: %v", err)
			m.mu.Unlock()
			logger.Get().Errorf("%s", m.lastError)
			return
		}

		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			m.mu.Lock()
			m.connecting = false
			m.lastError = fmt.Sprintf("Failed to acquire stdout pipe: %v", err)
			m.mu.Unlock()
			logger.Get().Errorf("%s", m.lastError)
			return
		}

		if err := cmd.Start(); err != nil {
			m.mu.Lock()
			m.connecting = false
			m.lastError = fmt.Sprintf("Failed to start cloudflared process: %v", err)
			m.mu.Unlock()
			logger.Get().Errorf("%s", m.lastError)
			return
		}

		m.mu.Lock()
		m.cmd = cmd
		m.mu.Unlock()

		logger.Get().Infof("Cloudflare tunnel process started (PID: %d)", cmd.Process.Pid)

		// Scanner to monitor stderr and stdout for the trycloudflare.com URL
		multiReader := io.MultiReader(stderrPipe, stdoutPipe)
		scanner := bufio.NewScanner(multiReader)

		urlFound := false
		var cb func(string)

		go func() {
			for scanner.Scan() {
				line := scanner.Text()

				if !urlFound {
					if match := tunnelURLRegex.FindString(line); match != "" {
						urlFound = true
						m.mu.Lock()
						m.publicURL = match
						m.active = true
						m.connecting = false
						cb = m.onURLFoundCb
						m.mu.Unlock()

						logger.Get().Infof("✅ Cloudflare Quick Tunnel ESTABLISHED: %s (Access from ANY network)", match)

						if cb != nil {
							go cb(match)
						}
					}
				}
			}
		}()

		// Watchdog: If URL isn't found within 30 seconds, mark error
		go func() {
			time.Sleep(30 * time.Second)
			m.mu.Lock()
			if m.connecting && !m.active {
				m.connecting = false
				m.lastError = "Tunnel initialization timed out after 30s"
				logger.Get().Warnf("Cloudflare tunnel timeout")
			}
			m.mu.Unlock()
		}()

		// Wait for command exit
		_ = cmd.Wait()

		m.mu.Lock()
		m.active = false
		m.connecting = false
		m.publicURL = ""
		m.cmd = nil
		m.mu.Unlock()

		logger.Get().Infof("Cloudflare tunnel process terminated")
	}()

	return nil
}

// Stop gracefully terminates the tunnel process.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		logger.Get().Infof("Stopping Cloudflare tunnel...")
		_ = m.cmd.Process.Kill()
		m.cmd = nil
	}

	m.active = false
	m.connecting = false
	m.publicURL = ""
	return nil
}

// Toggle toggles the tunnel state between active and stopped.
func (m *Manager) Toggle() (bool, error) {
	if m.IsActive() {
		return false, m.Stop()
	}
	return true, m.Start()
}
