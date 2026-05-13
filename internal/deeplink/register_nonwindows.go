//go:build !windows

package deeplink

func RegisterCurrentExecutable() error {
	return nil
}
