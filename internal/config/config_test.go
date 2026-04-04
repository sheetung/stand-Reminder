package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	err := os.WriteFile(path, []byte(`{
  "remind_after_minutes": 45,
  "idle_reset_minutes": 5,
  "check_interval_seconds": 5,
  "notification_title": "Stand Reminder",
  "notification_message": "Stretch"
}`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.RemindAfterMinutes != 45 {
		t.Fatalf("unexpected remind_after_minutes: %d", cfg.RemindAfterMinutes)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	err := os.WriteFile(path, []byte(`{invalid json`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected parse error")
	}

	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsMissingField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	err := os.WriteFile(path, []byte(`{
  "remind_after_minutes": 45,
  "idle_reset_minutes": 5,
  "check_interval_seconds": 5,
  "notification_title": "",
  "notification_message": "Stretch"
}`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "notification_title") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	want := Config{
		RemindAfterMinutes:   30,
		IdleResetMinutes:     6,
		CheckIntervalSeconds: 4,
		NotificationTitle:    "Stand Reminder",
		NotificationMessage:  "Stretch",
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("save config: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got != want {
		t.Fatalf("unexpected round trip config: %#v", got)
	}
}
