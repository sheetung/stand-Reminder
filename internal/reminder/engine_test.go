package reminder

import (
	"testing"
	"time"
)

func TestEngineAccumulatesWhileActive(t *testing.T) {
	engine := NewEngine(Config{
		RemindAfter:   15 * time.Second,
		IdleReset:     10 * time.Second,
		CheckInterval: 5 * time.Second,
	})

	result := engine.Update(2 * time.Second)
	if result.State != StateActive {
		t.Fatalf("expected active state, got %s", result.State)
	}

	if result.Accumulated != 5*time.Second {
		t.Fatalf("unexpected accumulated: %s", result.Accumulated)
	}
}

func TestEngineResetsAfterIdleThreshold(t *testing.T) {
	engine := NewEngine(Config{
		RemindAfter:   30 * time.Second,
		IdleReset:     10 * time.Second,
		CheckInterval: 5 * time.Second,
	})

	engine.Update(1 * time.Second)
	engine.Update(1 * time.Second)
	result := engine.Update(10 * time.Second)

	if result.State != StateIdleReset {
		t.Fatalf("expected idle reset, got %s", result.State)
	}

	if result.PreviousAccumulated != 10*time.Second {
		t.Fatalf("unexpected previous accumulated: %s", result.PreviousAccumulated)
	}
}

func TestEnginePausesWhenUserStopsInputBriefly(t *testing.T) {
	engine := NewEngine(Config{
		RemindAfter:   30 * time.Second,
		IdleReset:     20 * time.Second,
		CheckInterval: 5 * time.Second,
	})

	engine.Update(1 * time.Second)
	result := engine.Update(7 * time.Second)

	if result.State != StatePaused {
		t.Fatalf("expected paused state, got %s", result.State)
	}

	if result.Accumulated != 5*time.Second {
		t.Fatalf("unexpected accumulated: %s", result.Accumulated)
	}
}

func TestEngineTriggersReminderAndRestartsCycle(t *testing.T) {
	engine := NewEngine(Config{
		RemindAfter:   10 * time.Second,
		IdleReset:     20 * time.Second,
		CheckInterval: 5 * time.Second,
	})

	engine.Update(1 * time.Second)
	result := engine.Update(1 * time.Second)

	if result.State != StateReminderTriggered {
		t.Fatalf("expected reminder state, got %s", result.State)
	}

	if result.PreviousAccumulated != 10*time.Second {
		t.Fatalf("unexpected previous accumulated: %s", result.PreviousAccumulated)
	}

	next := engine.Update(1 * time.Second)
	if next.Accumulated != 5*time.Second {
		t.Fatalf("expected new cycle to restart, got %s", next.Accumulated)
	}
}
