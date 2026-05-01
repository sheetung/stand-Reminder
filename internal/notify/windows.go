//go:build windows

package notify

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
)

const (
	appID            = "StandReminder.App"
	showToastTimeout = 15 * time.Second
)

type WindowsNotifier struct {
	openURL string
	locale  string
}

func NewWindowsNotifier() WindowsNotifier {
	return WindowsNotifier{}
}

func (n *WindowsNotifier) SetOpenURL(url string) {
	n.openURL = strings.TrimSpace(url)
}

func (n *WindowsNotifier) SetLocale(locale string) {
	n.locale = locale
}

func (n *WindowsNotifier) Notify(title, message string) error {
	return n.showToast(title, message)
}

func (n *WindowsNotifier) showToast(title, message string) error {
	script := strings.TrimSpace(`
$ErrorActionPreference = 'Stop'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null
Add-Type -AssemblyName System.Security

$appID = $env:STAND_APP_ID
$title = [System.Security.SecurityElement]::Escape($env:STAND_TITLE)
$message = [System.Security.SecurityElement]::Escape($env:STAND_MESSAGE)
$openUrl = $env:STAND_OPEN_URL
$baseUrl = $env:STAND_BASE_URL
$escapedUrl = [System.Security.SecurityElement]::Escape($openUrl)
$escapedSnooze = [System.Security.SecurityElement]::Escape($baseUrl + '/api/action?action=snooze')
$escapedBreak = [System.Security.SecurityElement]::Escape($baseUrl + '/api/action?action=break')
$btnSnooze = [System.Security.SecurityElement]::Escape($env:STAND_BTN_SNOOZE)
$btnBreak = [System.Security.SecurityElement]::Escape($env:STAND_BTN_BREAK)
$btnOpen = [System.Security.SecurityElement]::Escape($env:STAND_BTN_OPEN_CENTER)

$template = '<toast><visual><binding template="ToastGeneric"><text>' + $title + '</text><text>' + $message + '</text></binding></visual></toast>'
if ($openUrl) {
    $template = '<toast activationType="protocol" launch="' + $escapedUrl + '"><visual><binding template="ToastGeneric"><text>' + $title + '</text><text>' + $message + '</text></binding></visual><actions><action content="' + $btnSnooze + '" activationType="protocol" arguments="' + $escapedSnooze + '" /><action content="' + $btnBreak + '" activationType="protocol" arguments="' + $escapedBreak + '" /><action content="' + $btnOpen + '" activationType="protocol" arguments="' + $escapedUrl + '" /></actions></toast>'
}

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
$notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appID)
$notifier.Show($toast)
`)

	return runPowerShellWithTimeout(script, map[string]string{
		"STAND_APP_ID":           appID,
		"STAND_TITLE":            title,
		"STAND_MESSAGE":          message,
		"STAND_OPEN_URL":         n.openURL,
		"STAND_BASE_URL":         n.openURL,
		"STAND_BTN_SNOOZE":       n.localizedBtnSnooze(),
		"STAND_BTN_BREAK":        n.localizedBtnBreak(),
		"STAND_BTN_OPEN_CENTER":  n.localizedBtnOpenCenter(),
	}, showToastTimeout)
}

func (n *WindowsNotifier) localizedBtnSnooze() string {
	if n.locale == "zh-CN" {
		return "延迟 5 分钟"
	}
	return "Snooze 5 min"
}

func (n *WindowsNotifier) localizedBtnBreak() string {
	if n.locale == "zh-CN" {
		return "我已起身活动"
	}
	return "I Took a Break"
}

func (n *WindowsNotifier) localizedBtnOpenCenter() string {
	if n.locale == "zh-CN" {
		return "打开控制中心"
	}
	return "Open Control Center"
}

func runPowerShellWithTimeout(script string, env map[string]string, timeout time.Duration) error {
	encoded := encodePowerShell(script)
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encoded)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Env = append(os.Environ(), flattenEnv(env)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("powershell timed out after %v", timeout)
		}
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("powershell failed: %w: %s", err, trimmed)
		}
		return fmt.Errorf("powershell failed: %w", err)
	}
	return nil
}

func flattenEnv(values map[string]string) []string {
	pairs := make([]string, 0, len(values))
	for key, value := range values {
		pairs = append(pairs, key+"="+value)
	}
	return pairs
}

func encodePowerShell(script string) string {
	encoded := utf16.Encode([]rune(script))
	bytes := make([]byte, 0, len(encoded)*2)
	for _, r := range encoded {
		bytes = append(bytes, byte(r), byte(r>>8))
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
