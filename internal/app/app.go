package app

import (
	"log"
	"sync"
	"time"

	"stand-reminder/internal/activity"
	"stand-reminder/internal/config"
	"stand-reminder/internal/notify"
	"stand-reminder/internal/reminder"
)

type Snapshot struct {
	Status               string `json:"status"`
	IdleSeconds          int64  `json:"idle_seconds"`
	AccumulatedSeconds   int64  `json:"accumulated_seconds"`
	RemainingSeconds     int64  `json:"remaining_seconds"`
	RemindAfterMinutes   int    `json:"remind_after_minutes"`
	IdleResetMinutes     int    `json:"idle_reset_minutes"`
	CheckIntervalSeconds int    `json:"check_interval_seconds"`
	NotificationTitle    string `json:"notification_title"`
	NotificationMessage  string `json:"notification_message"`
	UpdatedAt            string `json:"updated_at"`
}

type App struct {
	mu         sync.RWMutex
	configPath string
	cfg        config.Config
	detector   activity.Detector
	notifier   notify.WindowsNotifier
	engine     *reminder.Engine
	state      Snapshot
}

func New(configPath string) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	app := &App{
		configPath: configPath,
		cfg:        cfg,
		detector:   activity.NewDetector(),
		notifier:   notify.NewWindowsNotifier(),
	}
	app.rebuildLocked(cfg)
	app.state.Status = string(reminder.StateIdle)
	app.state.UpdatedAt = time.Now().Format(time.RFC3339)
	return app, nil
}

func (a *App) Run() {
	for {
		a.mu.RLock()
		interval := time.Duration(a.cfg.CheckIntervalSeconds) * time.Second
		engine := a.engine
		cfg := a.cfg
		a.mu.RUnlock()

		idle, err := a.detector.IdleDuration()
		if err != nil {
			log.Printf("failed to read input state: %v", err)
			time.Sleep(interval)
			continue
		}

		result := engine.Update(idle)

		a.mu.Lock()
		a.state.Status = string(result.State)
		a.state.IdleSeconds = int64(idle / time.Second)
		a.state.AccumulatedSeconds = int64(result.Accumulated / time.Second)
		a.state.RemainingSeconds = int64(result.Remaining / time.Second)
		a.state.UpdatedAt = time.Now().Format(time.RFC3339)
		if result.State == reminder.StateIdleReset {
			a.state.AccumulatedSeconds = 0
			a.state.RemainingSeconds = int64(time.Duration(cfg.RemindAfterMinutes) * time.Minute / time.Second)
		}
		if result.State == reminder.StateReminderTriggered {
			a.state.AccumulatedSeconds = 0
			a.state.RemainingSeconds = int64(time.Duration(cfg.RemindAfterMinutes) * time.Minute / time.Second)
		}
		a.mu.Unlock()

		switch result.State {
		case reminder.StateReminderTriggered:
			log.Printf("reminder triggered: active_duration=%s", result.PreviousAccumulated.Round(time.Second))
			if err := a.notifier.Notify(cfg.NotificationTitle, cfg.NotificationMessage); err != nil {
				log.Printf("failed to send notification: %v", err)
			}
		case reminder.StateIdleReset:
			log.Printf("idle reset: idle=%s previous_accumulated=%s", idle.Round(time.Second), result.PreviousAccumulated.Round(time.Second))
		}

		time.Sleep(interval)
	}
}

func (a *App) Snapshot() Snapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

func (a *App) UpdateConfig(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := config.Save(a.configPath, cfg); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = cfg
	a.rebuildLocked(cfg)
	a.state.Status = string(reminder.StateIdle)
	a.state.IdleSeconds = 0
	a.state.AccumulatedSeconds = 0
	a.state.RemainingSeconds = int64(time.Duration(cfg.RemindAfterMinutes) * time.Minute / time.Second)
	a.state.UpdatedAt = time.Now().Format(time.RFC3339)
	log.Printf("config updated: remind_after=%dm idle_reset=%dm check_interval=%ds", cfg.RemindAfterMinutes, cfg.IdleResetMinutes, cfg.CheckIntervalSeconds)
	return nil
}

func (a *App) TestNotification() error {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	return a.notifier.Notify(cfg.NotificationTitle, cfg.NotificationMessage)
}

func (a *App) NotifyStarted(controlCenterURL string) error {
	return a.notifier.Notify(
		"Stand Reminder 已启动",
		"程序已在系统托盘运行。单击托盘图标可打开控制中心："+controlCenterURL,
	)
}

func (a *App) rebuildLocked(cfg config.Config) {
	a.engine = reminder.NewEngine(reminder.Config{
		RemindAfter:   time.Duration(cfg.RemindAfterMinutes) * time.Minute,
		IdleReset:     time.Duration(cfg.IdleResetMinutes) * time.Minute,
		CheckInterval: time.Duration(cfg.CheckIntervalSeconds) * time.Second,
	})
	a.state.RemindAfterMinutes = cfg.RemindAfterMinutes
	a.state.IdleResetMinutes = cfg.IdleResetMinutes
	a.state.CheckIntervalSeconds = cfg.CheckIntervalSeconds
	a.state.NotificationTitle = cfg.NotificationTitle
	a.state.NotificationMessage = cfg.NotificationMessage
	a.state.RemainingSeconds = int64(time.Duration(cfg.RemindAfterMinutes) * time.Minute / time.Second)
}
