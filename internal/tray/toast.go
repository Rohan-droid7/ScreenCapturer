//go:build windows

package tray

import (
	"os/exec"
	"syscall"
)

// ShowCaptureToast triggers a lightweight Windows notification when a remote capture succeeds.
func ShowCaptureToast(clientIP string) {
	// Execute via powershell asynchronously with hidden window flag so no console pops up
	psScript := `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null
$template = @"
<toast>
    <visual>
        <binding template="ToastGeneric">
            <text>Screen Captured</text>
            <text>Remote phone captured the display</text>
        </binding>
    </visual>
</toast>
"@
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("ScreenCapturer").Show($toast)
`
	go func() {
		cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", psScript)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Run()
	}()
}
