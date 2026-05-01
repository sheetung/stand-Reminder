package app

import (
	"log"
	"sync"
	"time"

	"stand-reminder/internal/activity"
	"stand-reminder/internal/config"
	"stand-reminder/internal/notify"
	"stand-reminder/internal/reminder"
	"stand-reminder/internal/stats"
	"stand-reminder/internal/version"
)

const (
	statusManualPaused = "manual_paused"
	statusBreakMode    = "break_mode"
	breakDuration      = 10 * time.Minute
	autoPauseThreshold = 60 * time.Second
)

type Snapshot struct {
	Version                string `json:"version"`
	HasUpdate              bool   `json:"has_update"`
	LatestVersion          string `json:"latest_version"`
	ReleaseURL             string `json:"release_url"`
	Status                 string `json:"status"`
	IdleSeconds            int64  `json:"idle_seconds"`
	IdleAccumulatedSeconds int64  `json:"idle_accumulated_seconds"`
	AccumulatedSeconds     int64  `json:"accumulated_seconds"`
	RemainingSeconds       int64  `json:"remaining_seconds"`
	RemindAfterMinutes     int    `json:"remind_after_minutes"`
	IdleResetMinutes       int    `json:"idle_reset_minutes"`
	CheckIntervalSeconds   int    `json:"check_interval_seconds"`
	NotificationTitle      string `json:"notification_title"`
	NotificationMessage    string `json:"notification_message"`
	NotificationError      string `json:"notification_error"`
	Locale                 string `json:"locale"`
	Paused                 bool   `json:"paused"`
	OnBreak                bool   `json:"on_break"`
	BreakEndsAt            string `json:"break_ends_at"`
	UpdatedAt              string `json:"updated_at"`
}

type App struct {
	mu         sync.RWMutex
	cfg        config.Config
	detector   activity.Detector
	notifier   *notify.WindowsNotifier
	store      *stats.Store
	engine     *reminder.Engine
	locale     string
	state      Snapshot
	paused     bool
	breakUntil time.Time
	updateInfo version.UpdateInfo // Latest version information
}

func New(dbPath string, currentVersion string) (*App, error) {
	store, cfg, err := stats.Open(dbPath)
	if err != nil {
		return nil, err
	}

	locale, err := store.LoadLocale()
	if err != nil {
		return nil, err
	}

	n := notify.NewWindowsNotifier()
	app := &App{
		cfg:      cfg,
		detector: activity.NewDetector(),
		notifier: &n,
		store:    store,
		locale:   locale,
	}
	app.rebuildLocked(cfg)
	app.resetStateLocked(string(reminder.StateIdle))

	// Initialize version info (async check for updates)
	go func() {
		updateInfo := version.CheckUpdate(currentVersion)
		app.mu.Lock()
		app.updateInfo = updateInfo
		app.state.Version = currentVersion
		app.state.HasUpdate = updateInfo.HasUpdate
		app.state.LatestVersion = updateInfo.LatestVersion
		app.state.ReleaseURL = updateInfo.ReleaseURL
		app.mu.Unlock()
	}()

	return app, nil
}

func (a *App) SetControlCenterURL(url string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.notifier.SetOpenURL(url)
}

func (a *App) Stats(rangeKey string) (stats.Summary, error) {
	return a.store.Summary(rangeKey, time.Now())
}

func (a *App) Shutdown() {
	log.Println("app shutting down...")
	if err := a.store.Close(); err != nil {
		log.Printf("store close: %v", err)
	}
}

func (a *App) Run() {
	for {
		a.mu.Lock()
		interval := time.Duration(a.cfg.CheckIntervalSeconds) * time.Second
		engine := a.engine
		cfg := a.cfg
		paused := a.paused
		breakUntil := a.breakUntil
		now := time.Now()

		if !breakUntil.IsZero() {
			if now.Before(breakUntil) {
				a.state.Status = statusBreakMode
				a.state.Paused = false
				a.state.OnBreak = true
				a.state.BreakEndsAt = breakUntil.Format(time.RFC3339)
				a.state.UpdatedAt = now.Format(time.RFC3339)
				a.mu.Unlock()
				if err := a.store.AddDuration(now, stats.CategoryBreak, interval); err != nil {
					log.Printf("failed to update break stats: %v", err)
				}
				time.Sleep(interval)
				continue
			}

			a.breakUntil = time.Time{}
			a.paused = true
			a.rebuildLocked(cfg)
			a.resetStateLocked(statusManualPaused)
			a.mu.Unlock()
			time.Sleep(interval)
			continue
		}

		if paused {
			a.state.Status = statusManualPaused
			a.state.Paused = true
			a.state.OnBreak = false
			a.state.BreakEndsAt = ""
			a.state.UpdatedAt = now.Format(time.RFC3339)
			a.mu.Unlock()
			if err := a.store.AddDuration(now, stats.CategoryPaused, interval); err != nil {
				log.Printf("failed to update paused stats: %v", err)
			}
			time.Sleep(interval)
			continue
		}
		a.mu.Unlock()

		idle, err := a.detector.IdleDuration()
		if err != nil {
			log.Printf("failed to read input state: %v", err)
			time.Sleep(interval)
			continue
		}

		result := engine.Update(idle)

		a.mu.Lock()
		a.state.Status = string(result.State)
		a.state.Paused = false
		a.state.OnBreak = false
		a.state.BreakEndsAt = ""
		a.state.IdleSeconds = int64(idle / time.Second)
		a.state.AccumulatedSeconds = int64(result.Accumulated / time.Second)
		if result.State == reminder.StateIdle || result.State == reminder.StateIdleReset {
			a.state.IdleAccumulatedSeconds += int64(interval / time.Second)
		}
		a.state.RemainingSeconds = int64(result.Remaining / time.Second)
		a.state.UpdatedAt = now.Format(time.RFC3339)
		if result.State == reminder.StateIdleReset || result.State == reminder.StateReminderTriggered {
			a.state.AccumulatedSeconds = 0
			a.state.RemainingSeconds = int64(time.Duration(cfg.RemindAfterMinutes) * time.Minute / time.Second)
		}
		a.mu.Unlock()

		switch result.State {
		case reminder.StateActive:
			if err := a.store.AddDuration(now, stats.CategoryWork, interval); err != nil {
				log.Printf("failed to update active stats: %v", err)
			}
		case reminder.StatePaused, reminder.StateIdle, reminder.StateIdleReset:
			if err := a.store.AddDuration(now, stats.CategoryIdle, interval); err != nil {
				log.Printf("failed to update idle stats: %v", err)
			}
		}

		switch result.State {
		case reminder.StateReminderTriggered:
			log.Printf("reminder triggered: active_duration=%s", result.PreviousAccumulated.Round(time.Second))
			if err := a.store.AddReminder(now); err != nil {
				log.Printf("failed to update reminder stats: %v", err)
			}
			if err := a.notifier.Notify(cfg.NotificationTitle, cfg.NotificationMessage); err != nil {
				log.Printf("failed to send notification: %v", err)
				a.mu.Lock()
				a.state.NotificationError = err.Error()
				a.mu.Unlock()
			} else {
				a.mu.Lock()
				a.state.NotificationError = ""
				a.mu.Unlock()
			}
		case reminder.StateIdleReset:
			log.Printf("idle reset: idle=%s previous_accumulated=%s", idle.Round(time.Second), result.PreviousAccumulated.Round(time.Second))
			if err := a.store.AddIdleReset(now); err != nil {
				log.Printf("failed to update idle reset stats: %v", err)
			}
		}

		time.Sleep(interval)
	}
}

func (a *App) Locale() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.locale
}

func (a *App) Snapshot() Snapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

func (a *App) SetLocale(locale string) error {
	if err := a.store.SaveLocale(locale); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if locale == "en" || locale == "en-US" {
		a.locale = "en-US"
	} else {
		a.locale = "zh-CN"
	}
	a.state.Locale = a.locale
	return nil
}

func (a *App) UpdateConfig(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := a.store.SaveConfig(cfg); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = cfg
	a.paused = false
	a.breakUntil = time.Time{}
	a.rebuildLocked(cfg)
	a.resetStateLocked(string(reminder.StateIdle))
	log.Printf("config updated: remind_after=%dm idle_reset=%dm check_interval=%ds", cfg.RemindAfterMinutes, cfg.IdleResetMinutes, cfg.CheckIntervalSeconds)
	return nil
}

func (a *App) TestNotification() error {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	err := a.notifier.Notify(cfg.NotificationTitle, cfg.NotificationMessage)
	if err != nil {
		a.mu.Lock()
		a.state.NotificationError = err.Error()
		a.mu.Unlock()
	} else {
		a.mu.Lock()
		a.state.NotificationError = ""
		a.mu.Unlock()
	}
	return err
}

func (a *App) NotifyStarted(controlCenterURL string) error {
	err := a.notifier.Notify(
		"Stand Reminder Started",
		"Running in the system tray. Click the tray icon to open Control Center: "+controlCenterURL,
	)
	if err != nil {
		a.mu.Lock()
		a.state.NotificationError = err.Error()
		a.mu.Unlock()
	}
	return err
}

func (a *App) Pause() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.paused = true
	a.breakUntil = time.Time{}
	a.state.Paused = true
	a.state.OnBreak = false
	a.state.BreakEndsAt = ""
	a.state.Status = statusManualPaused
	a.state.UpdatedAt = time.Now().Format(time.RFC3339)
	return a.state
}

func (a *App) Resume() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.paused = false
	a.breakUntil = time.Time{}
	a.rebuildLocked(a.cfg)
	a.resetStateLocked(string(reminder.StateIdle))
	return a.state
}

func (a *App) StartBreak() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	a.paused = false
	a.breakUntil = now.Add(breakDuration)
	a.rebuildLocked(a.cfg)
	a.state.Status = statusBreakMode
	a.state.Paused = false
	a.state.OnBreak = true
	a.state.BreakEndsAt = a.breakUntil.Format(time.RFC3339)
	a.state.IdleSeconds = 0
	a.state.IdleAccumulatedSeconds = 0
	a.state.AccumulatedSeconds = 0
	a.state.RemainingSeconds = int64(time.Duration(a.cfg.RemindAfterMinutes) * time.Minute / time.Second)
	a.state.UpdatedAt = now.Format(time.RFC3339)
	if err := a.store.AddBreakEvent(now); err != nil {
		log.Printf("failed to update break stats: %v", err)
	}
	return a.state
}

func (a *App) resetStateLocked(status string) {
	a.state.Status = status
	a.state.Paused = a.paused
	a.state.OnBreak = false
	a.state.BreakEndsAt = ""
	a.state.IdleSeconds = 0
	a.state.IdleAccumulatedSeconds = 0
	a.state.AccumulatedSeconds = 0
	a.state.RemainingSeconds = int64(time.Duration(a.cfg.RemindAfterMinutes) * time.Minute / time.Second)
	a.state.Locale = a.locale
	a.state.UpdatedAt = time.Now().Format(time.RFC3339)
}

func (a *App) rebuildLocked(cfg config.Config) {
	a.engine = reminder.NewEngine(reminder.Config{
		RemindAfter:   time.Duration(cfg.RemindAfterMinutes) * time.Minute,
		IdleReset:     time.Duration(cfg.IdleResetMinutes) * time.Minute,
		PauseAfter:    autoPauseThreshold,
		CheckInterval: time.Duration(cfg.CheckIntervalSeconds) * time.Second,
	})
	a.state.RemindAfterMinutes = cfg.RemindAfterMinutes
	a.state.IdleResetMinutes = cfg.IdleResetMinutes
	a.state.CheckIntervalSeconds = cfg.CheckIntervalSeconds
	a.state.NotificationTitle = cfg.NotificationTitle
	a.state.NotificationMessage = cfg.NotificationMessage
	a.state.Locale = a.locale
}
