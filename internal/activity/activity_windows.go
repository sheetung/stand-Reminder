//go:build windows

package activity

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetTickCount64   = kernel32.NewProc("GetTickCount64")
)

type lastInputInfo struct {
	CbSize uint32
	DwTime uint32
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
