//go:build windows

package notify

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
)

const (
	appID            = "StandReminder.App"
	shortcutTimeout  = 30 * time.Second
	showToastTimeout = 15 * time.Second
)

type WindowsNotifier struct {
	exePath    string
	openURL    string
	shortcutOK bool
	shortcutMu sync.Mutex
}

func NewWindowsNotifier() WindowsNotifier {
	exePath, _ := resolveNotificationTarget()
	return WindowsNotifier{exePath: exePath}
}

func (n *WindowsNotifier) SetOpenURL(url string) {
	n.openURL = strings.TrimSpace(url)
}

func (n *WindowsNotifier) Notify(title, message string) error {
	if err := n.ensureShortcut(); err != nil {
		return fmt.Errorf("shortcut setup failed: %w", err)
	}

	return n.showToast(title, message)
}

func resolveNotificationTarget() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	cwd, err := os.Getwd()
	if err == nil && strings.Contains(strings.ToLower(exePath), "go-build") {
		candidate := filepath.Join(cwd, "stand-reminder.exe")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}

	return exePath, nil
}

func (n *WindowsNotifier) ensureShortcut() error {
	n.shortcutMu.Lock()
	if n.shortcutOK {
		n.shortcutMu.Unlock()
		return nil
	}
	n.shortcutMu.Unlock()

	shortcutPath := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Stand Reminder.lnk")
	script := strings.TrimSpace(`
$ErrorActionPreference = 'Stop'
$ShortcutPath = $env:STAND_SHORTCUT_PATH
$TargetPath = $env:STAND_TARGET_PATH
$AppID = $env:STAND_APP_ID

Add-Type -Language CSharp @"
using System;
using System.Runtime.InteropServices;
using System.Runtime.InteropServices.ComTypes;

[ComImport, Guid("00021401-0000-0000-C000-000000000046")]
internal class CShellLink {}

[ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("000214F9-0000-0000-C000-000000000046")]
internal interface IShellLinkW
{
    void GetPath([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszFile, int cchMaxPath, IntPtr pfd, uint fFlags);
    void GetIDList(out IntPtr ppidl);
    void SetIDList(IntPtr pidl);
    void GetDescription([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszName, int cchMaxName);
    void SetDescription([MarshalAs(UnmanagedType.LPWStr)] string pszName);
    void GetWorkingDirectory([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszDir, int cchMaxPath);
    void SetWorkingDirectory([MarshalAs(UnmanagedType.LPWStr)] string pszDir);
    void GetArguments([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszArgs, int cchMaxPath);
    void SetArguments([MarshalAs(UnmanagedType.LPWStr)] string pszArgs);
    void GetHotkey(out short pwHotkey);
    void SetHotkey(short wHotkey);
    void GetShowCmd(out int piShowCmd);
    void SetShowCmd(int iShowCmd);
    void GetIconLocation([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszIconPath, int cchIconPath, out int piIcon);
    void SetIconLocation([MarshalAs(UnmanagedType.LPWStr)] string pszIconPath, int iIcon);
    void SetRelativePath([MarshalAs(UnmanagedType.LPWStr)] string pszPathRel, uint dwReserved);
    void Resolve(IntPtr hwnd, uint fFlags);
    void SetPath([MarshalAs(UnmanagedType.LPWStr)] string pszFile);
}

[ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99")]
internal interface IPropertyStore
{
    void GetCount(out uint cProps);
    void GetAt(uint iProp, out PROPERTYKEY pkey);
    void GetValue(ref PROPERTYKEY key, out PROPVARIANT pv);
    void SetValue(ref PROPERTYKEY key, ref PROPVARIANT pv);
    void Commit();
}

[StructLayout(LayoutKind.Sequential, Pack = 4)]
internal struct PROPERTYKEY
{
    public Guid fmtid;
    public uint pid;

    public PROPERTYKEY(Guid guid, uint id)
    {
        fmtid = guid;
        pid = id;
    }
}

[StructLayout(LayoutKind.Sequential)]
internal struct PROPVARIANT
{
    public ushort vt;
    public ushort wReserved1;
    public ushort wReserved2;
    public ushort wReserved3;
    public IntPtr p;
    public int p2;

    public static PROPVARIANT FromString(string value)
    {
        var pv = new PROPVARIANT();
        pv.vt = 31;
        pv.p = Marshal.StringToCoTaskMemUni(value);
        return pv;
    }
}

public static class ShortcutHelper
{
    public static void CreateShortcut(string shortcutPath, string targetPath, string appId)
    {
        var link = (IShellLinkW)new CShellLink();
        link.SetPath(targetPath);
        link.SetWorkingDirectory(System.IO.Path.GetDirectoryName(targetPath));
        link.SetDescription("Stand Reminder notifications");
        link.SetIconLocation(targetPath, 0);

        var propertyStore = (IPropertyStore)link;
        var appIdKey = new PROPERTYKEY(new Guid("9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3"), 5);
        var value = PROPVARIANT.FromString(appId);
        propertyStore.SetValue(ref appIdKey, ref value);
        propertyStore.Commit();

        ((IPersistFile)link).Save(shortcutPath, true);
    }
}
"@

$directory = Split-Path -Parent $ShortcutPath
if (-not (Test-Path $directory)) {
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
}

[ShortcutHelper]::CreateShortcut($ShortcutPath, $TargetPath, $AppID)
`)

	err := runPowerShellWithTimeout(script, map[string]string{
		"STAND_SHORTCUT_PATH": shortcutPath,
		"STAND_TARGET_PATH":   n.exePath,
		"STAND_APP_ID":        appID,
	}, shortcutTimeout)

	n.shortcutMu.Lock()
	if err == nil {
		n.shortcutOK = true
	}
	n.shortcutMu.Unlock()
	return err
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
$escapedUrl = [System.Security.SecurityElement]::Escape($openUrl)

$template = '<toast><visual><binding template="ToastGeneric"><text>' + $title + '</text><text>' + $message + '</text></binding></visual></toast>'
if ($openUrl) {
    $template = '<toast activationType="protocol" launch="' + $escapedUrl + '"><visual><binding template="ToastGeneric"><text>' + $title + '</text><text>' + $message + '</text></binding></visual><actions><action content="Open Control Center" activationType="protocol" arguments="' + $escapedUrl + '" /></actions></toast>'
}

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
$notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appID)
$notifier.Show($toast)
`)

	return runPowerShellWithTimeout(script, map[string]string{
		"STAND_APP_ID":   appID,
		"STAND_TITLE":    title,
		"STAND_MESSAGE":  message,
		"STAND_OPEN_URL": n.openURL,
	}, showToastTimeout)
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
