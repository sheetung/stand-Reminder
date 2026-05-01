//go:build !windows

package notify

import "fmt"

type WindowsNotifier struct{}

func NewWindowsNotifier() WindowsNotifier {
	return WindowsNotifier{}
}

func (WindowsNotifier) SetOpenURL(string) {}

func (WindowsNotifier) SetLocale(string) {}

func (WindowsNotifier) Notify(title, message string) error {
	return fmt.Errorf("notifications are only supported on Windows")
}
