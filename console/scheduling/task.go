package schedule

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Task represents a scheduled recurring job.
type Task struct {
	name          string
	interval      time.Duration
	fn            func(ctx context.Context) error
	nextRun       time.Time
	noOverlap     bool
	running       atomic.Bool
	dailyHour     int
	dailyMin      int
	hasDailyClock bool
}

// Name assigns a human-readable name to the task for logging.
func (t *Task) Name(name string) *Task {
	t.name = name
	return t
}

// WithoutOverlapping prevents the task from starting a new run while the previous run is still active.
func (t *Task) WithoutOverlapping() *Task {
	t.noOverlap = true
	return t
}

// Every schedules the task to run at a custom interval.
func (t *Task) Every(d time.Duration) *Task {
	if d <= 0 {
		d = time.Minute
	}
	t.interval = d
	t.hasDailyClock = false
	t.nextRun = time.Now().Add(d)
	return t
}

// DailyAt schedules the task to run once per day at "HH:MM" UTC (e.g. DailyAt("03:00")).
func (t *Task) DailyAt(hhmm string) *Task {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
		t.dailyHour = h
		t.dailyMin = m
		t.hasDailyClock = true
		t.interval = 24 * time.Hour
		t.nextRun = nextDailyTime(time.Now().UTC(), h, m)
	}
	return t
}

func (t *Task) EverySecond() *Task      { return t.Every(time.Second) }
func (t *Task) EveryMinute() *Task      { return t.Every(time.Minute) }
func (t *Task) EveryFiveMinutes() *Task { return t.Every(5 * time.Minute) }
func (t *Task) Hourly() *Task           { return t.Every(time.Hour) }
func (t *Task) Daily() *Task            { return t.Every(24 * time.Hour) }

func (t *Task) advance(now time.Time) {
	if t.hasDailyClock {
		t.nextRun = nextDailyTime(now.UTC().Add(time.Second), t.dailyHour, t.dailyMin)
		return
	}
	t.nextRun = now.Add(t.interval)
}

func nextDailyTime(now time.Time, hour, minute int) time.Time {
	cand := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if !cand.After(now) {
		cand = cand.Add(24 * time.Hour)
	}
	return cand
}
