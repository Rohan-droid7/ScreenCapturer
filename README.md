# ScreenCapturer 📸

A lightweight cross-platform utility for Windows 10/11 & macOS that lets you capture your laptop's screen directly from your phone over local Wi-Fi. 

No cloud, no accounts, no video recording, no background polling, no snipping tool invocation, and 0% CPU while idle.

---

## 🚀 Quick Start (No Go or Coding Required!)

You **do NOT** need to install Go, Node.js, or any programming tools. Download the pre-built package for your OS:

### 1. Download
👉 **[Go to GitHub Releases v1.0.0](https://github.com/Rohan-droid7/ScreenCapturer/releases/latest)**

- **Windows 10 / 11:** Download `ScreenCapturer-v1.0.0-windows-amd64.zip`
- **macOS (Apple Silicon M1/M2/M3/M4):** Download `ScreenCapturer-v1.0.0-darwin-arm64.zip`
- **macOS (Intel):** Download `ScreenCapturer-v1.0.0-darwin-amd64.zip`

---

### 2. Start ScreenCapturer

#### On Windows:
- Extract the zip and double-click **`screencap.exe`**.
- Your web browser will automatically open showing your pairing QR code and connection status.
- *(If Windows Defender Firewall prompts you, check **Private networks** and click **Allow access**).*

#### On macOS:
- Extract the zip.
- Open Terminal in the extracted folder and run:
  ```bash
  chmod +x screencap stop.sh
  ./screencap
  ```
- Your default browser (Safari/Chrome) will open showing your pairing QR code.
- *(Note: On first launch, macOS will prompt you to grant **Screen Recording** permission in System Settings ➔ Privacy & Security).*

---

### 3. Connect & Capture from Your Phone
1. Make sure your phone is connected to the same Wi-Fi as your computer.
2. Scan the QR code shown on your laptop screen with your phone camera and open the link.
3. Tap **Capture** — your screen appears instantly.
4. Tap **Save** or long-press the preview to save to your camera roll.

<p align="center">
  <img src="docs/images/mobile-capture-desktop.jpg" width="300" alt="Mobile Capture Desktop Preview" />
  &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="docs/images/mobile-capture-meet.jpg" width="300" alt="Mobile Capture Video Call Preview" />
</p>

---

### 4. How to Turn It Off
When you are done, shut down the app anytime:
- **From your browser:** Click the red **"🛑 Turn Off ScreenCapturer"** button at the bottom of the pairing page.
- **On Windows:** Double-click **`stop.bat`** or click **Exit** in the System Tray.
- **On macOS:** Run `./stop.sh` or press `Ctrl + C` in Terminal.

---

## 🧠 How It Works

ScreenCapturer uses an on-demand, event-driven architecture that only takes a screenshot at the exact moment your phone requests one:

```
┌──────────────┐                  ┌──────────────────────────────────────────────┐
│ Mobile Phone │                  │               Windows Laptop                 │
│              │                  │                                              │
│ Tap Capture  ├─────────────────►│ 1. Validate Bearer Token & Host Header      │
│              │  POST /capture   │ 2. Check 5-attempt/60s IP Lockout            │
│              │                  │ 3. Acquire Screen Capture Mutex              │
│              │                  │ 4. BitBlt + GetDIBits on Input Desktop (GDI) │
│              │                  │ 5. In-place BGRA-to-RGBA Translation         │
│              │                  │ 6. Fast Encode (JPEG 90% or PNG BestSpeed)   │
│ Display Image│◄─────────────────┤ 7. Direct HTTP Response + FreeOSMemory()     │
└──────────────┘   Image Stream   └──────────────────────────────────────────────┘
```

1. **Native Windows Capture (GDI)**: Captures pixels directly using standard Win32 APIs (`BitBlt` + `GetDIBits`) without any slow external screen recorder or Snipping Tool.
2. **Full Resolution DPI-Aware**: Uses Per-Monitor V2 awareness so screenshots are captured at full hardware resolution (1920×1080, 2K, 4K) even with Windows 125%/150% scaling.
3. **Zero-Copy Conversion**: Directly swaps pixel channels in memory from Windows BGRA to standard RGBA without duplicating frame buffers.
4. **Self-Contained Executable**: All web assets (HTML/CSS/JS) are compiled directly inside the `.exe`.

---

## ⚙️ Advanced Details & Troubleshooting

<details>
<summary><b>🛠️ Building from Source (For Developers)</b></summary>

If you have Go installed and want to build the executable yourself:

```powershell
# 1. Clone repository
git clone https://github.com/Rohan-droid7/ScreenCapturer.git
cd ScreenCapturer

# 2. Build the production GUI binary (silent background window)
go build -ldflags "-H=windowsgui -s -w" -o screencap.exe ./cmd/screencap

# Or build with console logging enabled for testing:
go build -o screencap.exe ./cmd/screencap
```
</details>

<details>
<summary><b>🧱 Windows Firewall Setup (If Phone Cannot Connect)</b></summary>

If your phone browser says "Cannot connect" or times out, Windows Firewall may be blocking incoming connections. Run this command in **PowerShell (Run as Administrator)**:

```powershell
New-NetFirewallRule -DisplayName "ScreenCapturer" -Direction Inbound -LocalPort 8765 -Protocol TCP -Action Allow -Profile Private
```
*(Note: Hotel or corporate guest Wi-Fi networks may have "Client Isolation" turned on. If so, simply connect your laptop to your phone's personal mobile hotspot, or use a mesh tool like Tailscale).*
</details>

<details>
<summary><b>📊 Performance & Benchmarks</b></summary>

Measured on Windows 11 (AMD64, 1920×1080 physical display):

| Metric | Target | Measured Result |
| :--- | :---: | :---: |
| **Idle Memory (Working Set)** | 10–30 MB | **8.64 MB** |
| **Peak Memory (100 Captures Stress Test)** | 20–60 MB | **46.14 MB** |
| **Screen Capture Acquisition** | 20–200 ms | **58.44 ms** |
| **JPEG Encoding (Quality 90)** | 20–200 ms | **39.40 ms** (~290 KB) |
| **PNG Encoding (BestSpeed)** | 20–200 ms | **109.14 ms** (~755 KB) |
| **100 Sequential Captures Duration** | — | **20.82 seconds** (Avg 208 ms/frame) |
| **Goroutine / Handle Leaks** | 0 leaks | **0 leaks detected** |
</details>

<details>
<summary><b>🛡️ Security & Memory Management</b></summary>

- **Token-Based Authentication**: A 32-byte cryptographically random token is generated on first launch and stored in `%APPDATA%\ScreenCapturer\config.json`.
- **Safe Fragment Passing**: The token is delivered via URL fragment (`#token=...`), preventing it from ever being sent in plain HTTP request headers, browser history, or server access logs.
- **Brute Force Lockout**: 5 failed token attempts locks the offending client IP out for 60 seconds.
- **Automatic Garbage Collection**: `debug.FreeOSMemory()` is called immediately following each capture to release operating system memory back to Windows.
- **Locked Screen Protection**: If your laptop is locked (`Win + L`), the server returns an informative error message (`SCREEN_LOCKED`) instead of freezing or outputting corrupt blank frames.
</details>

---

## 📄 License
MIT License.
