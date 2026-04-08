package stats

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"stand-reminder/internal/config"
)

const (
	CategoryWork   = "work"
	CategoryBreak  = "break"
	CategoryIdle   = "idle"
	CategoryPaused = "paused"

	slotsPerDay  = 288
	slotDuration = 5 * time.Minute

	eventReminderTriggered = "reminder_triggered"
	eventBreakStarted      = "break_started"
	eventIdleReset         = "idle_reset"
)

type HourBucket struct {
	Hour          int   `json:"hour"`
	WorkSeconds   int64 `json:"work_seconds"`
	BreakSeconds  int64 `json:"break_seconds"`
	IdleSeconds   int64 `json:"idle_seconds"`
	PausedSeconds int64 `json:"paused_seconds"`
}

type SlotBucket struct {
	Index         int   `json:"index"`
	WorkSeconds   int64 `json:"work_seconds"`
	BreakSeconds  int64 `json:"break_seconds"`
	IdleSeconds   int64 `json:"idle_seconds"`
	PausedSeconds int64 `json:"paused_seconds"`
}

type DayRecord struct {
	Date          string       `json:"date"`
	WorkSeconds   int64        `json:"work_seconds"`
	BreakSeconds  int64        `json:"break_seconds"`
	IdleSeconds   int64        `json:"idle_seconds"`
	PausedSeconds int64        `json:"paused_seconds"`
	Reminders     int          `json:"reminders"`
	Breaks        int          `json:"breaks"`
	IdleResets    int          `json:"idle_resets"`
	Buckets       []HourBucket `json:"buckets"`
	Slots         []SlotBucket `json:"slots"`
}

type fileData struct {
	Days []DayRecord `json:"days"`
}

type Bar struct {
	Key           string `json:"key"`
	Label         string `json:"label"`
	WorkSeconds   int64  `json:"work_seconds"`
	BreakSeconds  int64  `json:"break_seconds"`
	IdleSeconds   int64  `json:"idle_seconds"`
	PausedSeconds int64  `json:"paused_seconds"`
	TotalSeconds  int64  `json:"total_seconds"`
	Dominant      string `json:"dominant"`
}

type Summary struct {
	RangeKey           string `json:"range_key"`
	TotalWorkSeconds   int64  `json:"total_work_seconds"`
	TotalBreakSeconds  int64  `json:"total_break_seconds"`
	TotalIdleSeconds   int64  `json:"total_idle_seconds"`
	TotalPausedSeconds int64  `json:"total_paused_seconds"`
	TotalReminders     int    `json:"total_reminders"`
	TotalBreaks        int    `json:"total_breaks"`
	TotalIdleResets    int    `json:"total_idle_resets"`
	Bars               []Bar  `json:"bars"`
}

type Store struct {
	mu sync.Mutex
	db *sql.DB
}

type sessionRow struct {
	ID       int64
	State    string
	Source   string
	Start    time.Time
	End      time.Time
	Duration int64
}

func Open(dbPath string) (*Store, config.Config, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, config.Config{}, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, config.Config{}, fmt.Errorf("open sqlite: %w", err)
	}

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, config.Config{}, err
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		_ = db.Close()
		return nil, config.Config{}, err
	}

	return store, cfg, nil
}

func (s *Store) initSchema() error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS user_profile (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			username TEXT NOT NULL DEFAULT 'default',
			display_name TEXT NOT NULL DEFAULT 'Default User',
			locale TEXT NOT NULL DEFAULT 'zh-CN',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			remind_after_minutes INTEGER NOT NULL DEFAULT 45,
			idle_reset_minutes INTEGER NOT NULL DEFAULT 5,
			check_interval_seconds INTEGER NOT NULL DEFAULT 5,
			notification_title TEXT NOT NULL DEFAULT 'Stand Reminder',
			notification_message TEXT NOT NULL DEFAULT 'You''ve been active for a while. Time to stand up and stretch.',
			break_duration_minutes INTEGER NOT NULL DEFAULT 10,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS activity_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL DEFAULT 1,
			state TEXT NOT NULL CHECK (state IN ('work', 'break', 'idle', 'paused')),
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			duration_seconds INTEGER NOT NULL,
			source TEXT NOT NULL DEFAULT 'system' CHECK (source IN ('system', 'manual', 'derived', 'import')),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES user_profile(id)
		);`,
		`CREATE TABLE IF NOT EXISTS app_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL DEFAULT 1,
			event_type TEXT NOT NULL CHECK (event_type IN ('reminder_triggered', 'break_started', 'break_finished', 'idle_reset', 'paused', 'resumed', 'app_started')),
			event_time TEXT NOT NULL,
			metadata_json TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES user_profile(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_activity_sessions_time ON activity_sessions(start_time, end_time);`,
		`CREATE INDEX IF NOT EXISTS idx_activity_sessions_state_time ON activity_sessions(state, start_time);`,
		`CREATE INDEX IF NOT EXISTS idx_app_events_type_time ON app_events(event_type, event_time);`,
		`INSERT INTO user_profile (id) VALUES (1) ON CONFLICT(id) DO NOTHING;`,
	}

	defaults := config.Default()
	insertSettings := fmt.Sprintf(`INSERT INTO app_settings (
		id,
		remind_after_minutes,
		idle_reset_minutes,
		check_interval_seconds,
		notification_title,
		notification_message,
		break_duration_minutes
	) VALUES (1, %d, %d, %d, %q, %q, 10)
	ON CONFLICT(id) DO NOTHING;`, defaults.RemindAfterMinutes, defaults.IdleResetMinutes, defaults.CheckIntervalSeconds, defaults.NotificationTitle, defaults.NotificationMessage)
	statements = append(statements, insertSettings)

	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}

	return nil
}

func (s *Store) LoadConfig() (config.Config, error) {
	row := s.db.QueryRow(`
		SELECT remind_after_minutes, idle_reset_minutes, check_interval_seconds, notification_title, notification_message
		FROM app_settings
		WHERE id = 1
	`)

	cfg := config.Config{}
	if err := row.Scan(&cfg.RemindAfterMinutes, &cfg.IdleResetMinutes, &cfg.CheckIntervalSeconds, &cfg.NotificationTitle, &cfg.NotificationMessage); err != nil {
		return config.Config{}, fmt.Errorf("load config from sqlite: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return config.Config{}, err
	}

	return cfg, nil
}

func (s *Store) SaveConfig(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	_, err := s.db.Exec(`
		UPDATE app_settings
		SET remind_after_minutes = ?,
			idle_reset_minutes = ?,
			check_interval_seconds = ?,
			notification_title = ?,
			notification_message = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, cfg.RemindAfterMinutes, cfg.IdleResetMinutes, cfg.CheckIntervalSeconds, cfg.NotificationTitle, cfg.NotificationMessage)
	if err != nil {
		return fmt.Errorf("save config to sqlite: %w", err)
	}

	return nil
}

func (s *Store) AddDuration(at time.Time, category string, duration time.Duration) error {
	seconds := int64(math.Round(duration.Seconds()))
	if seconds <= 0 {
		return nil
	}

	start := at.Add(-time.Duration(seconds) * time.Second)
	end := start.Add(time.Duration(seconds) * time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin add duration: %w", err)
	}
	defer rollback(tx)

	if err := s.appendOrMergeSessionTx(tx, category, "system", start, end); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit add duration: %w", err)
	}
	return nil
}

func (s *Store) AddReminder(at time.Time) error {
	return s.addEvent(eventReminderTriggered, at, nil)
}

func (s *Store) AddBreakEvent(at time.Time) error {
	return s.addEvent(eventBreakStarted, at, nil)
}

func (s *Store) AddIdleReset(at time.Time) error {
	return s.addEvent(eventIdleReset, at, nil)
}

func (s *Store) Summary(rangeKey string, now time.Time) (Summary, error) {
	mode := normalizeRange(rangeKey)
	start := startOfDay(now)
	end := start.Add(24 * time.Hour)
	if mode == "7d" {
		start = start.AddDate(0, 0, -6)
	} else if mode == "20d" {
		start = start.AddDate(0, 0, -19)
	}

	sessions, err := s.loadSessionsBetween(start, end)
	if err != nil {
		return Summary{}, err
	}
	counts, err := s.loadEventCounts(start, end)
	if err != nil {
		return Summary{}, err
	}

	result := Summary{
		RangeKey:        mode,
		TotalReminders:  counts[eventReminderTriggered],
		TotalBreaks:     counts[eventBreakStarted],
		TotalIdleResets: counts[eventIdleReset],
	}

	switch mode {
	case "today":
		result.Bars = buildTodayBars(start, sessions)
	default:
		daysCount := 7
		if mode == "20d" {
			daysCount = 20
		}
		result.Bars = buildDayBars(start, daysCount, sessions)
	}

	for _, bar := range result.Bars {
		result.TotalWorkSeconds += bar.WorkSeconds
		result.TotalBreakSeconds += bar.BreakSeconds
		result.TotalIdleSeconds += bar.IdleSeconds
		result.TotalPausedSeconds += bar.PausedSeconds
	}

	return result, nil
}

func buildTodayBars(dayStart time.Time, sessions []sessionRow) []Bar {
	bars := make([]Bar, slotsPerDay)
	for slot := 0; slot < slotsPerDay; slot++ {
		label := ""
		if slot%12 == 0 {
			label = dayStart.Add(time.Duration(slot) * slotDuration).Format("15:04")
		}
		bars[slot] = Bar{Key: fmt.Sprintf("%03d", slot), Label: label}
	}

	for _, session := range sessions {
		current := maxTime(session.Start, dayStart)
		dayEnd := dayStart.Add(24 * time.Hour)
		if current.After(dayEnd) {
			continue
		}
		finish := minTime(session.End, dayEnd)
		for current.Before(finish) {
			slotStart := current.Truncate(slotDuration)
			if slotStart.Before(dayStart) {
				slotStart = dayStart
			}
			slotIndex := int(slotStart.Sub(dayStart) / slotDuration)
			if slotIndex < 0 || slotIndex >= slotsPerDay {
				break
			}
			slotEnd := slotStart.Add(slotDuration)
			chunkEnd := minTime(finish, slotEnd)
			seconds := int64(chunkEnd.Sub(current) / time.Second)
			if seconds > 0 {
				applyBarSeconds(&bars[slotIndex], session.State, seconds)
			}
			current = chunkEnd
		}
	}

	for i := range bars {
		bars[i].TotalSeconds = bars[i].WorkSeconds + bars[i].BreakSeconds + bars[i].IdleSeconds + bars[i].PausedSeconds
		bars[i].Dominant = dominantCategory(bars[i])
	}

	return bars
}

func buildDayBars(start time.Time, daysCount int, sessions []sessionRow) []Bar {
	bars := make([]Bar, daysCount)
	for i := 0; i < daysCount; i++ {
		day := start.AddDate(0, 0, i)
		bars[i] = Bar{Key: dayKey(day), Label: day.Format("01/02")}
	}

	for _, session := range sessions {
		current := maxTime(session.Start, start)
		finish := session.End
		for current.Before(finish) {
			dayStart := startOfDay(current)
			index := int(dayStart.Sub(start).Hours() / 24)
			if index < 0 || index >= daysCount {
				break
			}
			dayEnd := dayStart.Add(24 * time.Hour)
			chunkEnd := minTime(finish, dayEnd)
			seconds := int64(chunkEnd.Sub(current) / time.Second)
			if seconds > 0 {
				applyBarSeconds(&bars[index], session.State, seconds)
			}
			current = chunkEnd
		}
	}

	for i := range bars {
		bars[i].TotalSeconds = bars[i].WorkSeconds + bars[i].BreakSeconds + bars[i].IdleSeconds + bars[i].PausedSeconds
		bars[i].Dominant = dominantCategory(bars[i])
	}

	return bars
}

func applyBarSeconds(bar *Bar, category string, seconds int64) {
	switch category {
	case CategoryWork:
		bar.WorkSeconds += seconds
	case CategoryBreak:
		bar.BreakSeconds += seconds
	case CategoryPaused:
		bar.PausedSeconds += seconds
	default:
		bar.IdleSeconds += seconds
	}
}

func dominantCategory(bar Bar) string {
	top := int64(0)
	name := ""
	candidates := []struct {
		name  string
		value int64
	}{
		{name: CategoryPaused, value: bar.PausedSeconds},
		{name: CategoryBreak, value: bar.BreakSeconds},
		{name: CategoryIdle, value: bar.IdleSeconds},
		{name: CategoryWork, value: bar.WorkSeconds},
	}
	for _, candidate := range candidates {
		if candidate.value > top {
			top = candidate.value
			name = candidate.name
		}
	}
	return name
}

func normalizeRange(key string) string {
	switch key {
	case "today":
		return "today"
	case "20d":
		return "20d"
	case "7d":
		fallthrough
	default:
		return "7d"
	}
}

func (s *Store) addEvent(eventType string, at time.Time, metadata any) error {
	var payload any
	if metadata != nil {
		payload = metadata
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin add event: %w", err)
	}
	defer rollback(tx)

	if err := s.insertEventTx(tx, eventType, at, payload); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit add event: %w", err)
	}
	return nil
}

func (s *Store) appendOrMergeSessionTx(tx *sql.Tx, state, source string, start, end time.Time) error {
	seconds := int64(end.Sub(start) / time.Second)
	if seconds <= 0 {
		return nil
	}

	last, err := loadLastSessionTx(tx)
	if err != nil {
		return err
	}

	if last != nil && last.State == state && last.Source == source && withinOneSecond(last.End, start) {
		_, err = tx.Exec(`
			UPDATE activity_sessions
			SET end_time = ?, duration_seconds = ?
			WHERE id = ?
		`, formatTime(end), last.Duration+seconds, last.ID)
		if err != nil {
			return fmt.Errorf("merge activity session: %w", err)
		}
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO activity_sessions (user_id, state, start_time, end_time, duration_seconds, source)
		VALUES (1, ?, ?, ?, ?, ?)
	`, state, formatTime(start), formatTime(end), seconds, source)
	if err != nil {
		return fmt.Errorf("insert activity session: %w", err)
	}
	return nil
}

func loadLastSessionTx(tx *sql.Tx) (*sessionRow, error) {
	row := tx.QueryRow(`
		SELECT id, state, source, start_time, end_time, duration_seconds
		FROM activity_sessions
		ORDER BY id DESC
		LIMIT 1
	`)

	var rawStart string
	var rawEnd string
	last := &sessionRow{}
	if err := row.Scan(&last.ID, &last.State, &last.Source, &rawStart, &rawEnd, &last.Duration); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load last session: %w", err)
	}

	start, err := parseTime(rawStart)
	if err != nil {
		return nil, err
	}
	end, err := parseTime(rawEnd)
	if err != nil {
		return nil, err
	}
	last.Start = start
	last.End = end
	return last, nil
}

func (s *Store) insertEventTx(tx *sql.Tx, eventType string, at time.Time, metadata any) error {
	metadataJSON := ""
	if metadata != nil {
		payload, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal event metadata: %w", err)
		}
		metadataJSON = string(payload)
	}

	_, err := tx.Exec(`
		INSERT INTO app_events (user_id, event_type, event_time, metadata_json)
		VALUES (1, ?, ?, ?)
	`, eventType, formatTime(at), metadataJSON)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (s *Store) loadSessionsBetween(start, end time.Time) ([]sessionRow, error) {
	rows, err := s.db.Query(`
		SELECT id, state, source, start_time, end_time, duration_seconds
		FROM activity_sessions
		WHERE end_time > ? AND start_time < ?
		ORDER BY start_time ASC, id ASC
	`, formatTime(start), formatTime(end))
	if err != nil {
		return nil, fmt.Errorf("query activity sessions: %w", err)
	}
	defer rows.Close()

	var sessions []sessionRow
	for rows.Next() {
		var rawStart string
		var rawEnd string
		var session sessionRow
		if err := rows.Scan(&session.ID, &session.State, &session.Source, &rawStart, &rawEnd, &session.Duration); err != nil {
			return nil, fmt.Errorf("scan activity session: %w", err)
		}
		startTime, err := parseTime(rawStart)
		if err != nil {
			return nil, err
		}
		endTime, err := parseTime(rawEnd)
		if err != nil {
			return nil, err
		}
		session.Start = startTime
		session.End = endTime
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity sessions: %w", err)
	}

	return sessions, nil
}

func (s *Store) loadEventCounts(start, end time.Time) (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT event_type, COUNT(*)
		FROM app_events
		WHERE event_time >= ? AND event_time < ?
		GROUP BY event_type
	`, formatTime(start), formatTime(end))
	if err != nil {
		return nil, fmt.Errorf("query event counts: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var eventType string
		var count int
		if err := rows.Scan(&eventType, &count); err != nil {
			return nil, fmt.Errorf("scan event counts: %w", err)
		}
		counts[eventType] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate event counts: %w", err)
	}
	return counts, nil
}

func normalizeRecord(record *DayRecord) {
	if len(record.Buckets) >= 24 {
		for hour := 0; hour < 24; hour++ {
			record.Buckets[hour].Hour = hour
		}
		record.Buckets = record.Buckets[:24]
	} else {
		buckets := make([]HourBucket, 24)
		for hour := 0; hour < 24; hour++ {
			buckets[hour].Hour = hour
			if hour < len(record.Buckets) {
				buckets[hour] = record.Buckets[hour]
				buckets[hour].Hour = hour
			}
		}
		record.Buckets = buckets
	}

	if len(record.Slots) >= slotsPerDay {
		for i := 0; i < slotsPerDay; i++ {
			record.Slots[i].Index = i
		}
		record.Slots = record.Slots[:slotsPerDay]
		return
	}

	slots := make([]SlotBucket, slotsPerDay)
	for i := 0; i < slotsPerDay; i++ {
		slots[i].Index = i
		if i < len(record.Slots) {
			slots[i] = record.Slots[i]
			slots[i].Index = i
		}
	}
	record.Slots = slots
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func parseTime(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse sqlite time %q: %w", raw, err)
	}
	return parsed, nil
}

func withinOneSecond(a, b time.Time) bool {
	delta := a.Sub(b)
	if delta < 0 {
		delta = -delta
	}
	return delta <= time.Second
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func dayKey(t time.Time) string {
	return startOfDay(t).Format("2006-01-02")
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}
