//go:build !windows

package tray

type StateInfo struct {
	Status             string
	IdleSeconds        int64
	AccumulatedSeconds int64
	RemainingSeconds   int64
	OnBreak            bool
	Paused             bool
}

func Run(_ string, _ func() string, _ func() StateInfo) error {
	select {}
}
