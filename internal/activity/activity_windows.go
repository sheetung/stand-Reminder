//go:build windows

package activity

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetForegroundWnd = user32.NewProc("GetForegroundWindow")
	procGetWindowRect    = user32.NewProc("GetWindowRect")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procGetWindowPID     = user32.NewProc("GetWindowThreadProcessId")
	procIsZoomed         = user32.NewProc("IsZoomed")
	procGetTickCount64   = kernel32.NewProc("GetTickCount64")
	procOpenProcess      = kernel32.NewProc("OpenProcess")
	procQueryFullImage   = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
)

const (
	processQueryLimitedInformation = 0x1000
	smCxScreen                     = 0
	smCyScreen                     = 1
)

type lastInputInfo struct {
	CbSize uint32
	DwTime uint32
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type Detector struct{}

func NewDetector() Detector {
	return Detector{}
}

func (Detector) IdleDuration() (time.Duration, error) {
	info := lastInputInfo{CbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}

	ret, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		if err != syscall.Errno(0) {
			return 0, fmt.Errorf("GetLastInputInfo: %w", err)
		}

		return 0, fmt.Errorf("GetLastInputInfo failed")
	}

	ticks, _, err := procGetTickCount64.Call()
	if err != syscall.Errno(0) {
		return 0, fmt.Errorf("GetTickCount64: %w", err)
	}

	now := uint64(ticks)
	last := uint64(info.DwTime)
	if now < last {
		return 0, nil
	}

	return time.Duration(now-last) * time.Millisecond, nil
}

func (Detector) MediaPlaying() (bool, error) {
	playing, err := queryMediaSessionPlaying()
	if err == nil && playing {
		return true, nil
	}

	fullscreenPlaying, fullscreenErr := foregroundFullscreenMediaApp()
	if fullscreenErr == nil && fullscreenPlaying {
		return true, nil
	}

	if err == nil {
		return false, fullscreenErr
	}
	if fullscreenErr == nil {
		return false, err
	}

	return false, fmt.Errorf("media session query failed: %v; fullscreen fallback failed: %w", err, fullscreenErr)
}

func queryMediaSessionPlaying() (bool, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", mediaPlaybackScript,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("media playback query failed: %w", err)
	}

	return strings.TrimSpace(string(output)) == "1", nil
}

func foregroundFullscreenMediaApp() (bool, error) {
	window, _, err := procGetForegroundWnd.Call()
	if window == 0 {
		if err != syscall.Errno(0) {
			return false, fmt.Errorf("GetForegroundWindow: %w", err)
		}
		return false, fmt.Errorf("GetForegroundWindow failed")
	}

	var bounds rect
	ret, _, err := procGetWindowRect.Call(window, uintptr(unsafe.Pointer(&bounds)))
	if ret == 0 {
		if err != syscall.Errno(0) {
			return false, fmt.Errorf("GetWindowRect: %w", err)
		}
		return false, fmt.Errorf("GetWindowRect failed")
	}

	screenWidth, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenHeight, _, _ := procGetSystemMetrics.Call(smCyScreen)
	if screenWidth == 0 || screenHeight == 0 {
		return false, fmt.Errorf("GetSystemMetrics returned zero size")
	}

	windowWidth := maxInt32(0, bounds.Right-bounds.Left)
	windowHeight := maxInt32(0, bounds.Bottom-bounds.Top)
	if windowWidth == 0 || windowHeight == 0 {
		return false, nil
	}

	screenArea := int64(screenWidth) * int64(screenHeight)
	windowArea := int64(windowWidth) * int64(windowHeight)
	maximized, _, _ := procIsZoomed.Call(window)
	if maximized == 0 && windowArea*100 < screenArea*70 {
		return false, nil
	}

	pid, err := windowProcessID(window)
	if err != nil {
		return false, err
	}

	exeName, err := processBaseName(pid)
	if err != nil {
		return false, err
	}

	_, ok := fullscreenMediaProcesses[exeName]
	return ok, nil
}

func windowProcessID(window uintptr) (uint32, error) {
	var pid uint32
	ret, _, err := procGetWindowPID.Call(window, uintptr(unsafe.Pointer(&pid)))
	if ret == 0 && pid == 0 {
		if err != syscall.Errno(0) {
			return 0, fmt.Errorf("GetWindowThreadProcessId: %w", err)
		}
		return 0, fmt.Errorf("GetWindowThreadProcessId failed")
	}
	return pid, nil
}

func processBaseName(pid uint32) (string, error) {
	handle, _, err := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		if err != syscall.Errno(0) {
			return "", fmt.Errorf("OpenProcess: %w", err)
		}
		return "", fmt.Errorf("OpenProcess failed")
	}
	defer procCloseHandle.Call(handle)

	buffer := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buffer))
	ret, _, err := procQueryFullImage.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		if err != syscall.Errno(0) {
			return "", fmt.Errorf("QueryFullProcessImageNameW: %w", err)
		}
		return "", fmt.Errorf("QueryFullProcessImageNameW failed")
	}

	fullPath := syscall.UTF16ToString(buffer[:size])
	return strings.ToLower(filepath.Base(fullPath)), nil
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

var fullscreenMediaProcesses = map[string]struct{}{
	"chrome.exe":          {},
	"msedge.exe":          {},
	"firefox.exe":         {},
	"brave.exe":           {},
	"opera.exe":           {},
	"vivaldi.exe":         {},
	"iexplore.exe":        {},
	"browser.exe":         {},
	"quark.exe":           {},
	"qqbrowser.exe":       {},
	"360chromex.exe":      {},
	"360se.exe":           {},
	"sogouexplorer.exe":   {},
	"thorium.exe":         {},
	"vlc.exe":             {},
	"potplayer64.exe":     {},
	"potplayermini64.exe": {},
	"potplayer.exe":       {},
	"potplayermini.exe":   {},
	"mpv.exe":             {},
	"wmplayer.exe":        {},
	"mpc-hc64.exe":        {},
	"mpc-hc.exe":          {},
	"mpc-be64.exe":        {},
	"mpc-be.exe":          {},
	"kmplayer.exe":        {},
	"kmplayer64x.exe":     {},
	"qqlive.exe":          {},
	"qqlivebrowser.exe":   {},
	"qiyi.exe":            {},
	"qyclient.exe":        {},
	"youkuclient.exe":     {},
	"bilibili.exe":        {},
	"cloudmusic.exe":      {},
	"spotify.exe":         {},
	"foobar2000.exe":      {},
}

const mediaPlaybackScript = `
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager, Windows.Media.Control, ContentType=WindowsRuntime]
$managerAsync = [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager]::RequestAsync()
$manager = [System.WindowsRuntimeSystemExtensions]::AsTask($managerAsync).GetAwaiter().GetResult()

if ($null -eq $manager) {
  Write-Output '0'
  exit
}

foreach ($session in $manager.GetSessions()) {
  try {
    $playbackInfo = $session.GetPlaybackInfo()
    if ($null -ne $playbackInfo -and $playbackInfo.PlaybackStatus.ToString() -eq 'Playing') {
      Write-Output '1'
      exit
    }
  } catch {
  }
}

Write-Output '0'
`
