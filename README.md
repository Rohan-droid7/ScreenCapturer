# ScreenCapturer 📸

A high-performance, ultra-lightweight Windows 10/11 background utility written in Go that lets you trigger on-demand screen captures of your laptop directly from your phone on the local Wi-Fi.

No cloud services, no external database, no video recording, no background polling, and zero CPU usage while idle.

---

## 📑 Table of Contents
- [How It Works](#-how-it-works)
- [Do You Need to Package the Whole Project?](#-do-you-need-to-package-the-whole-project)
- [How to Turn On / Off](#-how-to-turn-on--off)
- [Installation & Setup](#-installation--setup)
  - [Option A: Pre-built Single Executable](#option-a-pre-built-single-executable)
  - [Option B: Build from Source](#option-b-build-from-source)
- [Phone Walkthrough](#-phone-walkthrough)
- [Performance & Benchmarks](#-performance--benchmarks)
- [Memory Management & What Can Go Wrong](#-memory-management--what-can-go-wrong)
- [Security Model](#-security-model)

---

## 🧠 How It Works

ScreenCapturer is engineered with a strict **"Do nothing until requested"** architecture:

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

1. **Native Windows Capture without cgo**: Uses standard Windows GDI (`BitBlt` and `GetDIBits`) directly via `golang.org/x/sys/windows`.
2. **DPI-Aware**: Enabled with Per-Monitor V2 awareness (`SetProcessDpiAwarenessContext`) so screenshots always capture your display's full physical resolution (e.g. 1920×1080, 2560×1440, 4K) even if Windows display scaling is set to 125%, 150%, or 200%.
3. **In-place Zero-Copy Conversion**: GDI yields BGRA pixels. ScreenCapturer converts BGRA to RGBA and sets alpha to 255 in-place directly inside the target image buffer, eliminating full-frame memory duplication.
4. **Embedded Mobile Frontend**: All HTML, CSS, and modern JavaScript files are bundled directly into the executable using Go's `embed.FS`.

---

## 📦 Do You Need to Package the Whole Project?

**No!** You only need **one single file**: `screencap.exe` (~7.4 MB).

Because all HTML, CSS, and JavaScript are embedded directly inside the compiled binary via `go:embed`, the application requires:
- ❌ No Node.js / Python / frontend runtime
- ❌ No external assets, DLLs, or folder structures
- ❌ No database or configuration files to copy

Just copy `screencap.exe` anywhere and double-click it. It automatically initializes its configuration and pairing token in `%APPDATA%\ScreenCapturer\config.json` on first run.

---

## ⚡ How to Turn On / Off

### 1. From the Web Dashboard (Easiest & Recommended)
Whenever ScreenCapturer is running, the pairing control page opens in your browser.
- Simply click the red **"🛑 Turn Off ScreenCapturer"** button at the bottom of the page.
- It will immediately and cleanly stop the server, close all network ports, and terminate the process.

### 2. Double-Click `stop.bat`
In your ScreenCapturer folder, simply double-click **`stop.bat`** to instantly shut it down with one click.

### 3. Console Mode
If running via terminal (`.\screencap.exe --console`), simply press **`Ctrl + C`** to shut down.

---

## 🚀 Installation & Setup

### Option A: Pre-built Single Executable
1. Download `screencap.exe`.
2. Double-click `screencap.exe`.
3. If prompted by Windows Defender Firewall, check **Private networks** and click **Allow access**.

### Option B: Build from Source
**Prerequisites:** Go 1.22+ installed on Windows.

```powershell
# 1. Clone repository
git clone https://github.com/Rohan-droid7341/ScreenCapturer.git
cd ScreenCapturer

# 2. Build the production GUI executable (no console window)
go build -ldflags "-H=windowsgui -s -w" -o screencap.exe ./cmd/screencap

# Or build with console output enabled for debugging:
go build -o screencap.exe ./cmd/screencap
```

### Windows Firewall Setup (One-Time)
If your phone cannot connect, run this command in **PowerShell (Run as Administrator)**:
```powershell
New-NetFirewallRule -DisplayName "ScreenCapturer" -Direction Inbound -LocalPort 8765 -Protocol TCP -Action Allow -Profile Private
```

---

## 📱 Phone Walkthrough

### Step 1: Connect to Same Wi-Fi
Make sure your phone and laptop are connected to the same Wi-Fi network.

### Step 2: Open Mobile Control Page
Choose either:
- **Scan QR Code:** Right-click the tray icon ➔ **"Show Pairing QR Code"**, then scan the QR code with your phone camera.
- **Direct Link:** Right-click tray icon ➔ **"Copy Phone Address"**, and open the link on your mobile browser (Safari, Chrome, Firefox).

<!-- Placeholder for Pairing Screen -->
> ![Phone Pairing QR](docs/images/qr-pairing.png)

### Step 3: Capture and Save
1. Select your preferred format:
   - **JPEG (90%)**: ~100–140 ms roundtrip (recommended for speed and photos).
   - **PNG**: Lossless pixel-perfect image (recommended for reading small text).
2. Tap the large **📸 Capture Screen** button.
3. The image displays instantly with file size and roundtrip latency.
4. Tap **💾 Save** or **📤 Share**, or simply long-press the image to save to your camera roll.

<!-- Placeholder for Mobile UI Screen -->
> ![Mobile Interface Preview](docs/images/mobile-ui.png)

---

## 📊 Performance & Benchmarks

Measured on Windows 11 (AMD64, 1920×1080 native resolution):

| Metric | Target (PRD) | Measured Result |
| :--- | :---: | :---: |
| **Idle Memory (Working Set)** | 10–30 MB | **8.64 MB** |
| **Peak Memory (100 Captures Stress)** | 20–60 MB | **46.14 MB** |
| **Final Memory after GC** | Minimal | **37.06 MB** |
| **Screen Capture Acquisition** | 20–200 ms | **58.44 ms** |
| **JPEG Encoding (Quality 90)** | 20–200 ms | **39.40 ms** (~290 KB) |
| **PNG Encoding (BestSpeed)** | 20–200 ms | **109.14 ms** (~755 KB) |
| **100 Sequential Captures Duration** | — | **20.82 seconds** (Avg 208 ms/frame) |
| **Goroutine Leak Check** | 0 leaks | **Delta: 0** (1 baseline ➔ 1 final) |
| **GDI Handle Leak Check** | 0 leaks | **Delta: 0** (0 baseline ➔ 0 final) |

---

## 🛡️ Memory Management & What Can Go Wrong

### How Memory is Managed
1. **Zero Full-Frame Copies**: BitBlt reads directly into memory; in-place swap to RGBA.
2. **Runtime Memory Limit**: Set to `debug.SetMemoryLimit(128 MB)` with `debug.SetGCPercent(50)`.
3. **Explicit Deallocation**: `debug.FreeOSMemory()` is called immediately following every capture to release OS memory back to Windows.
4. **Browser Blob URL Revocation**: The mobile webpage automatically calls `URL.revokeObjectURL(oldUrl)` before creating a new image preview, preventing mobile browser memory bloat.

### What Else Can Go Wrong & How It Is Handled
- **Laptop Locked (`Win + L`) or Screensaver Active**:
  - *Behavior*: `OpenInputDesktop` detects locked workstation; server returns HTTP `503 Service Unavailable` with `{"error": "Screen is currently locked", "code": "SCREEN_LOCKED"}`.
  - *Result*: The mobile UI displays a friendly error banner instead of hanging or returning corrupt images.
- **Laptop Put to Sleep / Wakes Up**:
  - The capture engine binds dynamically per-request. When the laptop wakes up, captures resume immediately without requiring an app restart.
- **Wi-Fi IP Address Changes (DHCP renewal)**:
  - The server binds to `0.0.0.0:8765` (all local interfaces). If your laptop changes Wi-Fi IP, simply right-click the tray icon and select **"Show Pairing QR Code"** to get the updated URL.
- **Port Already in Use**:
  - If another application is using port 8765, ScreenCapturer catches `WSAEADDRINUSE` and logs an actionable error message. Port can be adjusted in `%APPDATA%\ScreenCapturer\config.json`.
- **Guest Wi-Fi Client Isolation**:
  - Some hotel/office guest Wi-Fi networks block devices from connecting to each other.
  - *Solution*: Enable a mobile hotspot on your phone and connect your laptop to it, or use a secure mesh VPN like **Tailscale**.

---

## 🔒 Security Model

- **32-Byte Cryptographic Token**: Generated using `crypto/rand` on first run. Never hardcoded.
- **URL Fragment Transport**: The token is passed in the URL fragment (`#token=<token>`). Fragments are never transmitted in HTTP request lines, browser history, or server access logs.
- **Constant-Time Verification**: Validated using `crypto/subtle.ConstantTimeCompare` to eliminate timing attacks.
- **Brute-Force Protection**: 5 consecutive failed authentication attempts automatically locks out the offending client IP for 60 seconds.
- **Host Header Validation**: Prevents DNS rebinding attacks.
- **Zero Sensitive Logging**: Tokens and image payload bytes are strictly prohibited from appearing in log files.

---

## 📄 License
MIT License. Created with Go and native Windows APIs.
