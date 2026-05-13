//go:build windows

package deeplink

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const classesKeyPath = `Software\Classes\` + Scheme

func RegisterCurrentExecutable() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	root, _, err := registry.CreateKey(registry.CURRENT_USER, classesKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create protocol root: %w", err)
	}
	defer root.Close()

	if err := root.SetStringValue("", "URL:Stand Reminder Protocol"); err != nil {
		return fmt.Errorf("set protocol description: %w", err)
	}
	if err := root.SetStringValue("URL Protocol", ""); err != nil {
		return fmt.Errorf("set protocol marker: %w", err)
	}

	commandKey, _, err := registry.CreateKey(registry.CURRENT_USER, classesKeyPath+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create protocol command: %w", err)
	}
	defer commandKey.Close()

	command := fmt.Sprintf(`"%s" "%%1"`, exePath)
	if err := commandKey.SetStringValue("", command); err != nil {
		return fmt.Errorf("set protocol command: %w", err)
	}

	return nil
}
