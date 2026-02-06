//go:build !android

package notify

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"unicode/utf16"
)

type commandNotifier struct{}

func newNotifier() Notifier {
	return &commandNotifier{}
}

func (n *commandNotifier) Send(title, body string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("notify-send", title, body).Run()
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		return exec.Command("osascript", "-e", script).Run()
	case "windows":
		return sendWindowsNotification(title, body)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func sendWindowsNotification(title, body string) error {
	script := `
$ErrorActionPreference = "Stop"
$title = [System.Environment]::GetEnvironmentVariable("WETTER_NOTIFY_TITLE")
$body = [System.Environment]::GetEnvironmentVariable("WETTER_NOTIFY_BODY")
if ([string]::IsNullOrWhiteSpace($title)) { $title = "Notification" }
if ($null -eq $body) { $body = "" }

function Show-Toast {
  [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
  [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
  $escTitle = [System.Security.SecurityElement]::Escape($title)
  $escBody = [System.Security.SecurityElement]::Escape($body)
  $xml = New-Object Windows.Data.Xml.Dom.XmlDocument
  $xml.LoadXml("<toast><visual><binding template='ToastGeneric'><text>$escTitle</text><text>$escBody</text></binding></visual></toast>")
  $toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
  $notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("Wetterabhaengig")
  $notifier.Show($toast)
}

function Show-Balloon {
  Add-Type -AssemblyName System.Windows.Forms | Out-Null
  Add-Type -AssemblyName System.Drawing | Out-Null
  $n = New-Object System.Windows.Forms.NotifyIcon
  $n.Icon = [System.Drawing.SystemIcons]::Information
  $n.BalloonTipIcon = [System.Windows.Forms.ToolTipIcon]::Info
  $n.BalloonTipTitle = $title
  $n.BalloonTipText = $body
  $n.Visible = $true
  $n.ShowBalloonTip(4000)
  Start-Sleep -Milliseconds 4500
  $n.Dispose()
}

try { Show-Toast } catch { Show-Balloon }
`
	encodedScript := encodePowerShell(script)
	shells := []string{"powershell.exe", "pwsh.exe", "pwsh"}
	var lastErr error

	for _, shell := range shells {
		path, err := exec.LookPath(shell)
		if err != nil {
			lastErr = err
			continue
		}
		cmd := exec.Command(path, "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-EncodedCommand", encodedScript)
		cmd.Env = append(os.Environ(),
			"WETTER_NOTIFY_TITLE="+title,
			"WETTER_NOTIFY_BODY="+body,
		)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = fmt.Errorf("%s: %w", shell, err)
		}
	}

	if lastErr == nil {
		return fmt.Errorf("windows notification failed: PowerShell runtime not found")
	}
	return fmt.Errorf("windows notification failed: %w", lastErr)
}

func encodePowerShell(script string) string {
	encoded := utf16.Encode([]rune(script))
	buf := make([]byte, len(encoded)*2)
	for i, ch := range encoded {
		buf[i*2] = byte(ch)
		buf[i*2+1] = byte(ch >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
