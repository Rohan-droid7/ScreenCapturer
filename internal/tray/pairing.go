package tray

import (
	"fmt"
	"os"
	"path/filepath"

	"ScreenCapturer/internal/config"
	"ScreenCapturer/internal/server"
)

// GenerateAndOpenPairingPage creates an interactive pairing HTML dashboard and opens it in the default browser.
func GenerateAndOpenPairingPage(cfg *config.Config) error {
	lanIP := server.GetPrimaryLANIP()
	token := cfg.GetToken()
	pairingURL := fmt.Sprintf("http://%s:%d/#token=%s", lanIP, cfg.Port, token)
	localURL := fmt.Sprintf("http://127.0.0.1:%d/#token=%s", cfg.Port, token)

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ScreenCapturer • Phone Pairing & Controls</title>
  <style>
    :root {
      --bg: #0f172a;
      --card: #1e293b;
      --border: #334155;
      --text: #f8fafc;
      --muted: #94a3b8;
      --accent: #3b82f6;
      --accent-hover: #2563eb;
      --success-bg: #064e3b;
      --success-border: #10b981;
      --success-text: #6ee7b7;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: var(--bg);
      color: var(--text);
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
      padding: 24px;
    }
    .container {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 16px;
      padding: 32px;
      max-width: 480px;
      width: 100%%;
      text-align: center;
      box-shadow: 0 20px 40px rgba(0,0,0,0.5);
    }
    .badge {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      background: var(--success-bg);
      border: 1px solid var(--success-border);
      color: var(--success-text);
      padding: 8px 16px;
      border-radius: 20px;
      font-size: 0.85rem;
      font-weight: 600;
      margin-bottom: 20px;
    }
    .pulse-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%%;
      background: var(--success-border);
      box-shadow: 0 0 10px var(--success-border);
    }
    h1 {
      font-size: 1.5rem;
      font-weight: 700;
      margin-bottom: 8px;
    }
    .subtitle {
      color: var(--muted);
      font-size: 0.95rem;
      margin-bottom: 24px;
      line-height: 1.5;
    }
    .reassurance-box {
      background: rgba(59, 130, 246, 0.1);
      border: 1px solid rgba(59, 130, 246, 0.3);
      padding: 12px 16px;
      border-radius: 10px;
      font-size: 0.85rem;
      color: #93c5fd;
      margin-bottom: 24px;
      text-align: left;
      line-height: 1.4;
    }
    .qr-container {
      background: #ffffff;
      padding: 16px;
      border-radius: 12px;
      display: inline-block;
      margin-bottom: 16px;
      box-shadow: 0 4px 12px rgba(0,0,0,0.2);
    }
    .qr-container img {
      display: block;
      width: 200px;
      height: 200px;
    }
    .url-display {
      background: var(--bg);
      border: 1px solid var(--border);
      padding: 10px 14px;
      border-radius: 8px;
      font-family: monospace;
      font-size: 0.8rem;
      word-break: break-all;
      color: #38bdf8;
      margin-bottom: 20px;
      user-select: all;
    }
    .actions {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }
    .btn {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      padding: 12px 20px;
      border-radius: 8px;
      font-weight: 600;
      font-size: 0.95rem;
      text-decoration: none;
      border: none;
      cursor: pointer;
      transition: background 0.15s ease;
    }
    .btn-primary {
      background: var(--accent);
      color: #ffffff;
    }
    .btn-primary:hover {
      background: var(--accent-hover);
    }
    .btn-secondary {
      background: transparent;
      color: var(--muted);
      border: 1px solid var(--border);
      font-size: 0.85rem;
    }
    .btn-secondary:hover {
      background: var(--border);
      color: var(--text);
    }
    .footer-help {
      margin-top: 24px;
      padding-top: 16px;
      border-top: 1px solid var(--border);
      font-size: 0.78rem;
      color: var(--muted);
      text-align: center;
      line-height: 1.5;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="badge">
      <span class="pulse-dot"></span>
      Running in Windows System Tray
    </div>

    <h1>Connect Your Phone</h1>
    <p class="subtitle">1. Connect phone to the <strong>same Wi-Fi</strong>.<br>2. Point your camera at this QR code:</p>

    <!-- Reassurance callout -->
    <div class="reassurance-box">
      🛡️ <strong>Feel free to close this browser tab!</strong><br>
      ScreenCapturer is actively running in the background. It will capture and deliver screenshots whenever you tap Capture on your phone.
    </div>

    <div class="qr-container" id="qrcode">
      <img src="https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=%s" alt="Pairing QR Code" />
    </div>

    <div class="url-display" title="Click to copy">%s</div>

    <div class="actions">
      <a href="%s" target="_blank" class="btn btn-primary">📸 Test Capture in This Browser</a>
      <button onclick="navigator.clipboard.writeText('%s'); alert('Phone link copied to clipboard!');" class="btn btn-secondary">📋 Copy Phone Address</button>
    </div>

    <div class="footer-help">
      <strong>How to turn off:</strong> Look for the camera icon in your taskbar tray (near the clock). Right-click it and choose <strong>Exit</strong> or <strong>Stop Server</strong>.
    </div>
  </div>
</body>
</html>`, pairingURL, pairingURL, localURL, pairingURL)

	appData, err := config.ConfigFilePath()
	if err != nil {
		return err
	}
	qrPath := filepath.Join(filepath.Dir(appData), "pairing.html")

	if err := os.WriteFile(qrPath, []byte(htmlContent), 0600); err != nil {
		return fmt.Errorf("failed to write pairing html: %w", err)
	}

	openBrowser(qrPath)
	return nil
}
