package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

const (
	CategoryWork   = "work"
	CategoryBreak  = "break"
	CategoryIdle   = "idle"
	CategoryPaused = "paused"

	slotsPerDay  = 288
	slotDuration = 5 * time.Minute
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
	mu   sync.Mutex
	path string
	days map[string]*DayRecord
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, days: map[string]*DayRecord{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read stats: %w", err)
	}

	var payload fileData
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse stats: %w", err)
	}

	for _, day := range payload.Days {
		copy := day
		normalizeRecord(&copy)
		s.days[day.Date] = &copy
	}

	return s, nil
}

func (s *Store) AddDuration(at time.Time, category string, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current := at.Add(-duration)
	remaining := duration
	for remaining > 0 {
		nextSlot := current.Truncate(slotDuration).Add(slotDuration)
		chunk := nextSlot.Sub(current)
		if chunk > remaining {
			chunk = remaining
		}

		record := s.dayRecordLocked(current)
		hourBucket := &record.Buckets[current.Hour()]
		slotIndex := int(current.Sub(startOfDay(current)) / slotDuration)
		if slotIndex < 0 {
			slotIndex = 0
		}
		if slotIndex >= slotsPerDay {
			slotIndex = slotsPerDay - 1
		}
		slotBucket := &record.Slots[slotIndex]
		seconds := int64(chunk / time.Second)
		applySeconds(record, hourBucket, slotBucket, category, seconds)

		current = current.Add(chunk)
		remaining -= chunk
	}

	return s.saveLocked()
}

func (s *Store) AddReminder(at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayRecordLocked(at).Reminders++
	return s.saveLocked()
}

func (s *Store) AddBreakEvent(at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayRecordLocked(at).Breaks++
	return s.saveLocked()
}

func (s *Store) AddIdleReset(at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayRecordLocked(at).IdleResets++
	return s.saveLocked()
}

func (s *Store) Summary(rangeKey string, now time.Time) (Summary, error) {
	mode := normalizeRange(rangeKey)

	s.mu.Lock()
	defer s.mu.Unlock()

	result := Summary{RangeKey: mode}
	start := startOfDay(now)

	switch mode {
	case "today":
		key := dayKey(start)
		record := s.days[key]
		for slot := 0; slot < slotsPerDay; slot++ {
			label := ""
			if slot%12 == 0 {
				label = start.Add(time.Duration(slot) * slotDuration).Format("15:04")
			}
			bar := Bar{Key: fmt.Sprintf("%03d", slot), Label: label}
			if record != nil && slot < len(record.Slots) {
				slotData := record.Slots[slot]
				bar.WorkSeconds = slotData.WorkSeconds
				bar.BreakSeconds = slotData.BreakSeconds
				bar.IdleSeconds = slotData.IdleSeconds
				bar.PausedSeconds = slotData.PausedSeconds
				bar.TotalSeconds = slotData.WorkSeconds + slotData.BreakSeconds + slotData.IdleSeconds + slotData.PausedSeconds
				bar.Dominant = dominantCategory(bar)
			}
			result.Bars = append(result.Bars, bar)
		}
		if record != nil {
			accumulateTotals(&result, record)
		}
	default:
		daysCount := 7
		if mode == "20d" {
			daysCount = 20
		}
		for i := daysCount - 1; i >= 0; i-- {
			day := start.AddDate(0, 0, -i)
			key := dayKey(day)
			bar := Bar{Key: key, Label: day.Format("01/02")}
			if record := s.days[key]; record != nil {
				bar.WorkSeconds = record.WorkSeconds
				bar.BreakSeconds = record.BreakSeconds
				bar.IdleSeconds = record.IdleSeconds
				bar.PausedSeconds = record.PausedSeconds
				bar.TotalSeconds = record.WorkSeconds + record.BreakSeconds + record.IdleSeconds + record.PausedSeconds
				bar.Dominant = dominantCategory(bar)
				accumulateTotals(&result, record)
			}
			result.Bars = append(result.Bars, bar)
		}
	}

	return result, nil
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

func (s *Store) dayRecordLocked(at time.Time) *DayRecord {
	key := dayKey(at)
	if record, ok := s.days[key]; ok {
		normalizeRecord(record)
		return record
	}

	record := &DayRecord{Date: key}
	normalizeRecord(record)
	s.days[key] = record
	return record
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

func applySeconds(record *DayRecord, hourBucket *HourBucket, slotBucket *SlotBucket, category string, seconds int64) {
	switch category {
	case CategoryWork:
		record.WorkSeconds += seconds
		hourBucket.WorkSeconds += seconds
		slotBucket.WorkSeconds += seconds
	case CategoryBreak:
		record.BreakSeconds += seconds
		hourBucket.BreakSeconds += seconds
		slotBucket.BreakSeconds += seconds
	case CategoryPaused:
		record.PausedSeconds += seconds
		hourBucket.PausedSeconds += seconds
		slotBucket.PausedSeconds += seconds
	default:
		record.IdleSeconds += seconds
		hourBucket.IdleSeconds += seconds
		slotBucket.IdleSeconds += seconds
	}
}

func accumulateTotals(summary *Summary, record *DayRecord) {
	summary.TotalWorkSeconds += record.WorkSeconds
	summary.TotalBreakSeconds += record.BreakSeconds
	summary.TotalIdleSeconds += record.IdleSeconds
	summary.TotalPausedSeconds += record.PausedSeconds
	summary.TotalReminders += record.Reminders
	summary.TotalBreaks += record.Breaks
	summary.TotalIdleResets += record.IdleResets
}

func (s *Store) saveLocked() error {
	payload := fileData{Days: make([]DayRecord, 0, len(s.days))}
	keys := make([]string, 0, len(s.days))
	for key := range s.days {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		payload.Days = append(payload.Days, *s.days[key])
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write stats: %w", err)
	}
	return nil
}

func dayKey(t time.Time) string {
	return startOfDay(t).Format("2006-01-02")
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}
