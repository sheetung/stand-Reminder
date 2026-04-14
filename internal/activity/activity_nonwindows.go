//go:build !windows

package activity

import (
	"fmt"
	"time"
)

type Detector struct{}

func NewDetector() Detector {
	return Detector{}
}

func (Detector) IdleDuration() (time.Duration, error) {
	return 0, fmt.Errorf("activity detection is only supported on Windows")
}
