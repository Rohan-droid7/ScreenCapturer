#!/bin/sh
# ScreenCapturer Stop Script for macOS
echo "Stopping ScreenCapturer..."
killall screencap 2>/dev/null || pkill -f screencap 2>/dev/null
echo "ScreenCapturer stopped."
