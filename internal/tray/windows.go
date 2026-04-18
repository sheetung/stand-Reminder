//go:build windows

package tray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	appassets "stand-reminder/assets"
	webui "stand-reminder/internal/web"

	"golang.org/x/sys/windows/registry"
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	shell32                    = syscall.NewLazyDLL("shell32.dll")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procLoadIconW              = user32.NewProc("LoadIconW")
	procLoadCursorW            = user32.NewProc("LoadCursorW")
	procLoadImageW             = user32.NewProc("LoadImageW")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wmDestroy       = 0x0002
	wmCommand       = 0x0111
	wmApp           = 0x8000
	wmTrayCallback  = wmApp + 1
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmContextMenu   = 0x007B
	ninSelect       = 0x0400
	ninKeySelect    = 0x0401

	csHRedraw = 0x0002
	csVRedraw = 0x0001

	idiApplication = 32512
	idcArrow       = 32512
	imageIcon      = 1
	lrLoadFromFile = 0x0010
	lrDefaultSize  = 0x0040

	cwUseDefault = 0x80000000

	nimAdd     = 0x00000000
	nimDelete  = 0x00000002
	nimSetVer  = 0x00000004
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	notifVersion4 = 4

	mfString    = 0x00000000
	mfChecked   = 0x00000008
	mfSeparator = 0x00000800

	tpmBottomAlign = 0x0020
	tpmLeftAlign   = 0x0000
	tpmRightButton = 0x0002
)

const (
	menuOpen      = 1001
	menuAutoStart = 1002
	menuExit      = 1003
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "StandReminder"
)

type point struct {
	X int32
	Y int32
}

type msg struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	State            uint32
	StateMask        uint32
	SzInfo           [256]uint16
	UnionTimeout     uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         syscall.GUID
	HBalloonIcon     uintptr
}

var (
	trayURL               string
	trayLocale            func() string
	trayIcon              uintptr
	taskbarCreatedMessage uint32
	trayProc              = syscall.NewCallback(wndProc)
	classNamePtr          = syscall.StringToUTF16Ptr("StandReminderTrayWindow")
)

func Run(url string, localeProvider func() string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	trayURL = url
	trayLocale = localeProvider
	trayIcon = loadTrayIcon()
	taskbarCreatedMessage = registerTaskbarCreatedMessage()

	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		Style:     csHRedraw | csVRedraw,
		WndProc:   trayProc,
		Icon:      trayIcon,
		Cursor:    cursor,
		ClassName: classNamePtr,
		IconSm:    trayIcon,
	}

	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("RegisterClassExW failed: %w", err)
	}

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Stand Reminder"))),
		0,
		cwUseDefault,
		cwUseDefault,
		cwUseDefault,
		cwUseDefault,
		0,
		0,
		0,
		0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed: %w", err)
	}

	if err := addTrayIcon(hwnd, trayIcon); err != nil {
		return err
	}
	defer deleteTrayIcon(hwnd)

	var message msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) == -1 {
			return fmt.Errorf("GetMessageW failed")
		}
		if ret == 0 {
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func wndProc(hwnd, msgID, wParam, lParam uintptr) uintptr {
	if taskbarCreatedMessage != 0 && uint32(msgID) == taskbarCreatedMessage {
		_ = addTrayIcon(hwnd, trayIcon)
		return 0
	}

	switch uint32(msgID) {
	case wmTrayCallback:
		eventCode := uint32(lParam & 0xffff)
		switch eventCode {
		case wmLButtonDblClk, ninSelect, ninKeySelect:
			openBrowser(trayURL)
		case wmRButtonUp, wmContextMenu:
			showMenu(hwnd)
		}
		return 0
	case wmCommand:
		switch uint32(wParam & 0xffff) {
		case menuOpen:
			openBrowser(trayURL)
		case menuAutoStart:
			_ = toggleAutoStart()
		case menuExit:
			deleteTrayIcon(hwnd)
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
		return ret
	}
}

func addTrayIcon(hwnd, icon uintptr) error {
	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip
	nid.UCallbackMessage = wmTrayCallback
	nid.HIcon = icon
	copy(nid.SzTip[:], syscall.StringToUTF16("Stand Reminder"))

	ret, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIconW add failed: %w", err)
	}

	nid.UnionTimeout = notifVersion4
	procShellNotifyIconW.Call(nimSetVer, uintptr(unsafe.Pointer(&nid)))
	return nil
}

func deleteTrayIcon(hwnd uintptr) {
	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = 1
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
}

func showMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	locale := currentLocale()
	openLabel := webui.LocaleText(locale, "trayOpenConsole", "Open Control Center")
	autoStartLabel := webui.LocaleText(locale, "trayLaunchAtStartup", "Launch at Startup")
	exitLabel := webui.LocaleText(locale, "trayExit", "Exit")

	procAppendMenuW.Call(menu, mfString, menuOpen, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(openLabel))))

	autoStartFlags := uintptr(mfString)
	enabled, err := isAutoStartEnabled()
	if err == nil && enabled {
		autoStartFlags |= mfChecked
	}
	procAppendMenuW.Call(menu, autoStartFlags, menuAutoStart, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(autoStartLabel))))
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	procAppendMenuW.Call(menu, mfString, menuExit, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(exitLabel))))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(hwnd)
	procTrackPopupMenu.Call(menu, tpmBottomAlign|tpmLeftAlign|tpmRightButton, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
}

func currentLocale() string {
	if trayLocale == nil {
		return "zh-CN"
	}
	locale := trayLocale()
	if locale == "en" || locale == "en-US" {
		return "en-US"
	}
	return "zh-CN"
}

func toggleAutoStart() error {
	enabled, err := isAutoStartEnabled()
	if err != nil {
		return err
	}
	if enabled {
		return disableAutoStart()
	}
	return enableAutoStart()
}

func isAutoStartEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer key.Close()

	value, _, err := key.GetStringValue(runValueName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}

	current, currentErr := currentExecutablePath()
	if currentErr != nil {
		return false, currentErr
	}
	stored := normalizeRunValue(value)
	if stored == "" {
		return false, nil
	}
	return stored == current, nil
}

func enableAutoStart() error {
	exePath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(runValueName, fmt.Sprintf("\"%s\"", exePath))
}

func currentExecutablePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exePath)
}

func normalizeRunValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"")
	value = strings.ReplaceAll(value, "\\\\", "\\")
	return value
}

func disableAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	defer key.Close()

	if err := key.DeleteValue(runValueName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}

func openBrowser(url string) {
	_ = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Start()
}

func loadTrayIcon() uintptr {
	iconPath, err := writeTempIconFile(appassets.StandReminderICO)
	if err == nil {
		ptr := syscall.StringToUTF16Ptr(iconPath)
		icon, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(ptr)), imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
		if icon != 0 {
			return icon
		}
	}
	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	return icon
}

func writeTempIconFile(data []byte) (string, error) {
	file, err := os.CreateTemp("", "stand-reminder-*.ico")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func registerTaskbarCreatedMessage() uint32 {
	msg, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("TaskbarCreated"))))
	return uint32(msg)
}
