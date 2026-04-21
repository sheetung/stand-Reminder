package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	RemindAfterMinutes   int    `json:"remind_after_minutes"`
	IdleResetMinutes     int    `json:"idle_reset_minutes"`
	CheckIntervalSeconds int    `json:"check_interval_seconds"`
	NotificationTitle    string `json:"notification_title"`
	NotificationMessage  string `json:"notification_message"`
}

func Default() Config {
	return Config{
		RemindAfterMinutes:   45,
		IdleResetMinutes:     5,
		CheckIntervalSeconds: 5,
		NotificationTitle:    "Stand Reminder",
		NotificationMessage:  "You've been active for a while. Time to stand up and stretch.",
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	if c.RemindAfterMinutes <= 0 {
		return fmt.Errorf("remind_after_minutes must be greater than 0")
	}

	if c.IdleResetMinutes <= 0 {
		return fmt.Errorf("idle_reset_minutes must be greater than 0")
	}

	if c.CheckIntervalSeconds <= 0 {
		return fmt.Errorf("check_interval_seconds must be greater than 0")
	}

	if c.NotificationTitle == "" {
		return fmt.Errorf("notification_title must not be empty")
	}

	if c.NotificationMessage == "" {
		return fmt.Errorf("notification_message must not be empty")
	}

	return nil
}

func ExampleJSON() string {
	defaults := Default()
	return strings.TrimSpace(fmt.Sprintf(`{
  "remind_after_minutes": %d,
  "idle_reset_minutes": %d,
  "check_interval_seconds": %d,
  "notification_title": %q,
  "notification_message": %q
}`,
		defaults.RemindAfterMinutes,
		defaults.IdleResetMinutes,
		defaults.CheckIntervalSeconds,
		defaults.NotificationTitle,
		defaults.NotificationMessage,
	))
}
