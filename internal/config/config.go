package config

import "fmt"

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
