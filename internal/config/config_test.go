package config

import (
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestValidateRejectsZeroRemindAfter(t *testing.T) {
	cfg := Default()
	cfg.RemindAfterMinutes = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for remind_after_minutes")
	}
}

func TestValidateRejectsZeroIdleReset(t *testing.T) {
	cfg := Default()
	cfg.IdleResetMinutes = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for idle_reset_minutes")
	}
}

func TestValidateRejectsZeroCheckInterval(t *testing.T) {
	cfg := Default()
	cfg.CheckIntervalSeconds = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for check_interval_seconds")
	}
}

func TestValidateRejectsEmptyTitle(t *testing.T) {
	cfg := Default()
	cfg.NotificationTitle = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty notification_title")
	}
}

func TestValidateRejectsEmptyMessage(t *testing.T) {
	cfg := Default()
	cfg.NotificationMessage = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty notification_message")
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	cfg := Config{
		RemindAfterMinutes:   30,
		IdleResetMinutes:     6,
		CheckIntervalSeconds: 4,
		NotificationTitle:    "Stand Reminder",
		NotificationMessage:  "Stretch",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}
