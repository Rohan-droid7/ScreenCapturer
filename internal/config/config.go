package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// AppName is the identifier used for appdata and mutexes.
const AppName = "ScreenCapturer"

// Config holds persistent user configuration.
type Config struct {
	Token         string   `json:"token"`
	PIN           string   `json:"pin"`
	Port          int      `json:"port"`
	AllowedHosts  []string `json:"allowed_hosts"`
	AutoStopHours int      `json:"auto_stop_hours"` // 0 = disabled
	CaptureEngine string   `json:"capture_engine"`  // "gdi" or "dxgi"
	DefaultFormat string   `json:"default_format"`  // "png" or "jpeg"
	JPEGQuality   int      `json:"jpeg_quality"`    // 1-100, default 90
	MaxLogSizeMB  int      `json:"max_log_size_mb"` // default 5 MB
	MaxLogBackups int      `json:"max_log_backups"` // default 3
	EnableTunnel  bool     `json:"enable_tunnel"`   // enable Cloudflare tunnel for cellular/WAN access

	mu sync.RWMutex
}

// DefaultConfig returns safe default settings.
func DefaultConfig() *Config {
	engine := "gdi"
	if runtime.GOOS == "darwin" {
		engine = "coregraphics"
	}
	return &Config{
		Port:          8765,
		AllowedHosts:  []string{"localhost", "127.0.0.1"},
		AutoStopHours: 0,
		CaptureEngine: engine,
		DefaultFormat: "png",
		JPEGQuality:   90,
		MaxLogSizeMB:  5,
		MaxLogBackups: 3,
		EnableTunnel:  true,
	}
}

// ConfigFilePath returns the absolute path to the configuration file.
// On Windows: %APPDATA%\ScreenCapturer\config.json
// On macOS: ~/Library/Application Support/ScreenCapturer/config.json
func ConfigFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, hErr := os.UserHomeDir()
		if hErr != nil {
			return "", fmt.Errorf("unable to determine user config directory: %w", err)
		}
		configDir = filepath.Join(home, ".config")
	}
	dir := filepath.Join(configDir, AppName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create app data directory: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

// GeneratePIN generates a simple 4-digit numeric pairing PIN.
func GeneratePIN() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	num := (int(b[0])<<8 | int(b[1])) % 10000
	return fmt.Sprintf("%04d", num)
}

// GenerateToken generates a cryptographically secure 32-byte hex-encoded token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto rand failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// LoadOrCreate loads existing config or creates a new one with a generated 32-byte token and 4-digit PIN.
func LoadOrCreate() (*Config, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			token, genErr := GenerateToken()
			if genErr != nil {
				return nil, genErr
			}
			cfg.Token = token
			cfg.PIN = GeneratePIN()
			if saveErr := cfg.Save(); saveErr != nil {
				return nil, saveErr
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// If token was missing or empty, generate one and save
	if cfg.Token == "" {
		token, genErr := GenerateToken()
		if genErr != nil {
			return nil, genErr
		}
		cfg.Token = token
		cfg.PIN = GeneratePIN()
		_ = cfg.Save()
	} else if cfg.PIN == "" {
		cfg.PIN = GeneratePIN()
		_ = cfg.Save()
	}

	return cfg, nil
}

// Save writes the configuration to disk atomically.
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path, err := ConfigFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.tmp", path)
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp config file: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit config file: %w", err)
	}

	return nil
}

// RegenerateToken creates and saves a new 32-byte token and 4-digit PIN.
func (c *Config) RegenerateToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newToken, err := GenerateToken()
	if err != nil {
		return "", err
	}
	c.Token = newToken
	c.PIN = GeneratePIN()

	path, err := ConfigFilePath()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	return newToken, nil
}

// GetToken returns the current token thread-safely.
func (c *Config) GetToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Token
}

// GetPIN returns the current 4-digit PIN thread-safely.
func (c *Config) GetPIN() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PIN
}
