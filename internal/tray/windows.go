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
	"stand-reminder/internal/deeplink"
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
	procDestroyIcon            = user32.NewProc("DestroyIcon")
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
	procSetTimer               = user32.NewProc("SetTimer")
	procKillTimer              = user32.NewProc("KillTimer")
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
	wmTimer         = 0x0113
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
	nimModify  = 0x00000001
	nimDelete  = 0x00000002
	nimSetVer  = 0x00000004
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nifShowTip = 0x00000080

	notifVersion4 = 4

	mfString    = 0x00000000
	mfGrayed    = 0x00000001
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
	menuPause     = 1004
	menuResume    = 1005
	menuBreak     = 1006

	trayRefreshTimerID = 1
	trayRefreshMS      = 1000
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
	trayState             func() StateInfo
	trayIcon              uintptr
	trayIcons             map[string]uintptr
	trayStatusCached      string
	trayTooltipCached     string
	taskbarCreatedMessage uint32
	trayProc              = syscall.NewCallback(wndProc)
	classNamePtr          = syscall.StringToUTF16Ptr("StandReminderTrayWindow")
)

type StateInfo struct {
	Status             string
	IdleSeconds        int64
	AccumulatedSeconds int64
	RemainingSeconds   int64
	OnBreak            bool
	Paused             bool
}

func Run(url string, localeProvider func() string, stateProvider func() StateInfo) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	trayURL = url
	trayLocale = localeProvider
	trayState = stateProvider
	trayIcons = loadTrayIcons()
	state := currentState()
	trayStatusCached = state.Status
	trayIcon = iconForStatus(trayStatusCached)
	trayTooltipCached = currentTooltip(state)
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

	if err := addTrayIcon(hwnd, trayIcon, trayTooltipCached); err != nil {
		return err
	}
	defer deleteTrayIcon(hwnd)
	defer destroyTrayIcons()

	procSetTimer.Call(hwnd, trayRefreshTimerID, trayRefreshMS, 0)
	defer procKillTimer.Call(hwnd, trayRefreshTimerID)

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
		state := currentState()
		trayIcon = iconForStatus(state.Status)
		trayTooltipCached = currentTooltip(state)
		_ = addTrayIcon(hwnd, trayIcon, trayTooltipCached)
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
		case menuPause:
			_ = invokeTrayAction("pause")
		case menuResume:
			_ = invokeTrayAction("resume")
		case menuBreak:
			_ = invokeTrayAction("break")
		case menuAutoStart:
			_ = toggleAutoStart()
		case menuExit:
			deleteTrayIcon(hwnd)
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case wmTimer:
		if wParam == trayRefreshTimerID {
			refreshTrayIcon(hwnd)
			return 0
		}
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
		return ret
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
	return ret
}

func addTrayIcon(hwnd, icon uintptr, tip string) error {
	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip | nifShowTip
	nid.UCallbackMessage = wmTrayCallback
	nid.HIcon = icon
	copyTooltip(nid.SzTip[:], tip)

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

func updateTray(hwnd, icon uintptr, tip string) error {
	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = 1
	nid.UFlags = nifIcon | nifTip | nifShowTip
	nid.HIcon = icon
	copyTooltip(nid.SzTip[:], tip)
	ret, _, err := procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIconW modify failed: %w", err)
	}
	return nil
}

func showMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	locale := currentLocale()
	state := currentState()
	openLabel := webui.LocaleText(locale, "trayOpenConsole", "Open Control Center")
	pauseLabel := webui.LocaleText(locale, "btnPause", "Pause")
	resumeLabel := webui.LocaleText(locale, "btnResume", "Start / Resume")
	breakLabel := webui.LocaleText(locale, "btnBreak", "I Took a Break")
	autoStartLabel := webui.LocaleText(locale, "trayLaunchAtStartup", "Launch at Startup")
	exitLabel := webui.LocaleText(locale, "trayExit", "Exit")

	procAppendMenuW.Call(menu, mfString, menuOpen, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(openLabel))))
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	procAppendMenuW.Call(menu, actionMenuFlags(canPause(state)), menuPause, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(pauseLabel))))
	procAppendMenuW.Call(menu, actionMenuFlags(canResume(state)), menuResume, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(resumeLabel))))
	procAppendMenuW.Call(menu, actionMenuFlags(canBreak(state)), menuBreak, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(breakLabel))))
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)

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

func invokeTrayAction(action string) error {
	return deeplink.ForwardAction(trayURL, action)
}

func actionMenuFlags(enabled bool) uintptr {
	if enabled {
		return mfString
	}
	return mfString | mfGrayed
}

func canPause(state StateInfo) bool {
	return !state.Paused && !state.OnBreak
}

func canResume(state StateInfo) bool {
	return state.Paused || state.OnBreak
}

func canBreak(state StateInfo) bool {
	return !state.OnBreak
}

func loadTrayIcon() uintptr {
	iconPath, err := writeTempIconFile(appassets.StandReminderICO)
	if err == nil {
		defer os.Remove(iconPath)
		ptr := syscall.StringToUTF16Ptr(iconPath)
		icon, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(ptr)), imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
		if icon != 0 {
			return icon
		}
	}
	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	return icon
}

func loadTrayIcons() map[string]uintptr {
	icons := map[string]uintptr{
		"":                   loadTrayIconFromBytes(appassets.StandReminderBlackICO),
		"active":             loadTrayIconFromBytes(appassets.StandReminderBlackICO),
		"paused":             loadTrayIconFromBytes(appassets.StandReminderGrayICO),
		"manual_paused":      loadTrayIconFromBytes(appassets.StandReminderGrayICO),
		"break_mode":         loadTrayIconFromBytes(appassets.StandReminderBlueICO),
		"idle":               loadTrayIconFromBytes(appassets.StandReminderDarkGrayICO),
		"idle_reset":         loadTrayIconFromBytes(appassets.StandReminderGreenICO),
		"reminder_triggered": loadTrayIconFromBytes(appassets.StandReminderRedICO),
	}
	for key, icon := range icons {
		if icon == 0 {
			icons[key] = loadTrayIcon()
		}
	}
	return icons
}

func iconForStatus(status string) uintptr {
	if trayIcons == nil {
		return loadTrayIcon()
	}
	status = strings.TrimSpace(status)
	if icon, ok := trayIcons[status]; ok && icon != 0 {
		return icon
	}
	if icon, ok := trayIcons["active"]; ok && icon != 0 {
		return icon
	}
	return loadTrayIcon()
}

func currentState() StateInfo {
	if trayState == nil {
		return StateInfo{Status: "active"}
	}
	state := trayState()
	state.Status = strings.TrimSpace(state.Status)
	if state.Status == "" {
		state.Status = "active"
	}
	return state
}

func refreshTrayIcon(hwnd uintptr) {
	state := currentState()
	status := state.Status
	tip := currentTooltip(state)
	if status == trayStatusCached && tip == trayTooltipCached {
		return
	}
	icon := iconForStatus(status)
	if icon == 0 {
		return
	}
	if err := updateTray(hwnd, icon, tip); err == nil {
		trayStatusCached = status
		trayTooltipCached = tip
		trayIcon = icon
	}
}

func copyTooltip(dst []uint16, tip string) {
	copy(dst, syscall.StringToUTF16(truncateTooltip(tip)))
}

func truncateTooltip(tip string) string {
	runes := []rune(strings.TrimSpace(tip))
	if len(runes) > 63 {
		return string(runes[:63])
	}
	return string(runes)
}

func currentTooltip(state StateInfo) string {
	locale := currentLocale()
	label := localizedStatus(locale, state.Status)
	switch state.Status {
	case "break_mode":
		return fmt.Sprintf("Stand Reminder | %s %s", label, formatClockShort(state.RemainingSeconds))
	case "reminder_triggered":
		return fmt.Sprintf("Stand Reminder | %s %s", label, formatAccumulated(locale, state.AccumulatedSeconds))
	case "manual_paused", "paused":
		return fmt.Sprintf("Stand Reminder | %s %s", label, formatAccumulated(locale, state.AccumulatedSeconds))
	case "idle", "idle_reset":
		return fmt.Sprintf("Stand Reminder | %s %s", label, formatIdle(locale, state.IdleSeconds))
	default:
		return fmt.Sprintf("Stand Reminder | %s %s %s", label, formatAccumulated(locale, state.AccumulatedSeconds), formatRemaining(locale, state.RemainingSeconds))
	}
}

func localizedStatus(locale, status string) string {
	isEN := locale == "en" || locale == "en-US"
	switch status {
	case "active":
		if isEN {
			return "Active"
		}
		return "正常"
	case "paused":
		if isEN {
			return "Auto Paused"
		}
		return "自动暂停"
	case "manual_paused":
		if isEN {
			return "Paused"
		}
		return "已暂停"
	case "break_mode":
		if isEN {
			return "On Break"
		}
		return "休息中"
	case "idle":
		if isEN {
			return "Idle"
		}
		return "空闲中"
	case "idle_reset":
		if isEN {
			return "Idle Reset"
		}
		return "空闲重置"
	case "reminder_triggered":
		if isEN {
			return "Time to Rest"
		}
		return "该休息了"
	default:
		if isEN {
			return "Active"
		}
		return "正常"
	}
}

func formatMinutes(locale string, seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	minutes := seconds / 60
	if locale == "en" || locale == "en-US" {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}

func formatAccumulated(locale string, seconds int64) string {
	if locale == "en" || locale == "en-US" {
		return "Elapsed " + formatMinutes(locale, seconds)
	}
	return "已计时" + formatMinutes(locale, seconds)
}

func formatRemaining(locale string, seconds int64) string {
	if locale == "en" || locale == "en-US" {
		return "Left " + formatMinutes(locale, seconds)
	}
	return "剩余" + formatMinutes(locale, seconds)
}

func formatIdle(locale string, seconds int64) string {
	if locale == "en" || locale == "en-US" {
		return "Idle " + formatClockShort(seconds)
	}
	return "空闲" + formatClockShort(seconds)
}

func formatClockShort(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	minutes := seconds / 60
	remainSeconds := seconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, remainSeconds)
}

func destroyTrayIcons() {
	for _, icon := range trayIcons {
		if icon != 0 {
			procDestroyIcon.Call(icon)
		}
	}
	trayIcons = nil
}

func loadTrayIconFromBytes(data []byte) uintptr {
	if len(data) == 0 {
		return 0
	}
	iconPath, err := writeTempIconFile(data)
	if err != nil {
		return 0
	}
	defer os.Remove(iconPath)
	ptr := syscall.StringToUTF16Ptr(iconPath)
	icon, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(ptr)), imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
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
