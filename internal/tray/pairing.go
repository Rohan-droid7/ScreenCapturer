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
      --bg: #09090b;
      --card: #121215;
      --border: #222227;
      --text: #fafafa;
      --muted: #71717a;
      --accent: #2563eb;
      --accent-hover: #1d4ed8;
      --success-bg: #052e16;
      --success-border: #15803d;
      --success-text: #86efac;
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
    .btn-danger {
      background: rgba(220, 38, 38, 0.2);
      color: #fca5a5;
      border: 1px solid rgba(220, 38, 38, 0.4);
      margin-top: 6px;
    }
    .btn-danger:hover {
      background: #dc2626;
      color: #ffffff;
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
  <div class="container" id="mainCard">
    <div class="badge" id="statusBadge">
      <span class="pulse-dot"></span>
      ScreenCapturer is Active
    </div>

    <h1>Connect Your Phone</h1>
    <p class="subtitle">1. Connect phone to the <strong>same Wi-Fi</strong>.<br>2. Point your camera at this QR code:</p>

    <!-- Reassurance callout -->
    <div class="reassurance-box">
      🛡️ <strong>Feel free to close this browser tab!</strong><br>
      ScreenCapturer is running in the background. It will capture and deliver screenshots whenever you tap Capture on your phone.
    </div>

    <div class="qr-container" id="qrcode">
      <img src="https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=%s" alt="Pairing QR Code" />
    </div>

    <div class="url-display" title="Click to copy">%s</div>

    <div class="actions">
      <a href="%s" target="_blank" class="btn btn-primary">📸 Test Capture in This Browser</a>
      <button onclick="navigator.clipboard.writeText('%s'); alert('Phone link copied to clipboard!');" class="btn btn-secondary">📋 Copy Phone Address</button>
      <button onclick="turnOffScreenCapturer()" id="shutdownBtn" class="btn btn-danger">🛑 Turn Off ScreenCapturer</button>
    </div>

    <div class="footer-help">
      <strong>Want to turn it off?</strong> You can click the red button above, or double-click <strong>stop.bat</strong> in your folder.
    </div>
  </div>

  <script>
    async function turnOffScreenCapturer() {
      if (!confirm('Are you sure you want to turn off ScreenCapturer?\\n\\nCapture requests from your phone will stop until you open it again.')) {
        return;
      }
      const btn = document.getElementById('shutdownBtn');
      btn.disabled = true;
      btn.textContent = 'Stopping...';
      try {
        await fetch('http://127.0.0.1:%d/api/shutdown', {
          method: 'POST',
          headers: {
            'Authorization': 'Bearer %s'
          }
        });
      } catch (_) {}

      document.getElementById('mainCard').innerHTML = 
        '<div style="padding: 24px 0;">' +
          '<div style="font-size: 3.5rem; margin-bottom: 16px;">🛑</div>' +
          '<h2 style="color: #f8fafc; margin-bottom: 8px;">ScreenCapturer is Off</h2>' +
          '<p style="color: #94a3b8; font-size: 0.95rem; line-height: 1.5; margin-bottom: 24px;">' +
            'The background agent has completely shut down.<br>' +
            'All network ports have been closed.' +
          '</p>' +
          '<div style="background: rgba(255,255,255,0.05); padding: 12px; border-radius: 8px; font-size: 0.85rem; color: #cbd5e1;">' +
            'To turn it back on later, just double-click <strong>screencap.exe</strong>.' +
          '</div>' +
        '</div>';
    }
  </script>
</body>
</html>`, pairingURL, pairingURL, localURL, pairingURL, cfg.Port, token)

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
