package reminder

import "time"

type Config struct {
	RemindAfter   time.Duration
	IdleReset     time.Duration
	CheckInterval time.Duration
}

type State string

const (
	StateActive            State = "active"
	StatePaused            State = "paused"
	StateIdle              State = "idle"
	StateIdleReset         State = "idle_reset"
	StateReminderTriggered State = "reminder_triggered"
)

type UpdateResult struct {
	State               State
	Accumulated         time.Duration
	PreviousAccumulated time.Duration
	Remaining           time.Duration
}

type Engine struct {
	remindAfter   time.Duration
	idleReset     time.Duration
	checkInterval time.Duration
	accumulated   time.Duration
}

func NewEngine(cfg Config) *Engine {
	return &Engine{
		remindAfter:   cfg.RemindAfter,
		idleReset:     cfg.IdleReset,
		checkInterval: cfg.CheckInterval,
	}
}

func (e *Engine) CheckInterval() time.Duration {
	return e.checkInterval
}

func (e *Engine) Update(idle time.Duration) UpdateResult {
	if idle >= e.idleReset {
		previous := e.accumulated
		e.accumulated = 0
		if previous > 0 {
			return UpdateResult{
				State:               StateIdleReset,
				PreviousAccumulated: previous,
			}
		}

		return UpdateResult{State: StateIdle}
	}

	if idle >= e.checkInterval {
		remaining := e.remindAfter - e.accumulated
		if remaining < 0 {
			remaining = 0
		}

		return UpdateResult{
			State:       StatePaused,
			Accumulated: e.accumulated,
			Remaining:   remaining,
		}
	}

	e.accumulated += e.checkInterval
	if e.accumulated >= e.remindAfter {
		previous := e.accumulated
		e.accumulated = 0
		return UpdateResult{
			State:               StateReminderTriggered,
			PreviousAccumulated: previous,
		}
	}

	remaining := e.remindAfter - e.accumulated
	if remaining < 0 {
		remaining = 0
	}

	return UpdateResult{
		State:       StateActive,
		Accumulated: e.accumulated,
		Remaining:   remaining,
	}
}
