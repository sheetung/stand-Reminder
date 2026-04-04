//go:build !windows

package tray

func Run(_ string) error {
	select {}
}
