//go:build !windows

package tray

func Run(_ string, _ func() string) error {
	select {}
}
