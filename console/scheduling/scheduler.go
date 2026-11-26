package schedule

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	applog "github.com/rapide-go/rapide/log"
)

// Scheduler runs recurring tasks in-process inside the single Go binary (no system crontab needed).
type Scheduler struct {
	mu      sync.Mutex
	tasks   []*Task
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
}

// New creates a new Scheduler.
func New() *Scheduler {
	return &Scheduler{}
}

// Call registers a recurring task function. Default interval is 1 minute unless chained.
func (s *Scheduler) Call(fn func(ctx context.Context) error) *Task {
	t := &Task{
		name:     "unnamed-task",
		interval: time.Minute,
		fn:       fn,
		nextRun:  time.Now().Add(time.Minute),
	}
	s.mu.Lock()
	s.tasks = append(s.tasks, t)
	s.mu.Unlock()
	return t
}

// TaskInfo summarizes a scheduled task for `schedule:list`.
type TaskInfo struct {
	Name      string
	Interval  time.Duration
	NextRun   time.Time
	NoOverlap bool
}

// Tasks returns a snapshot of all registered scheduled tasks.
func (s *Scheduler) Tasks() []TaskInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TaskInfo, len(s.tasks))
	for i, t := range s.tasks {
		out[i] = TaskInfo{
			Name:      t.name,
			Interval:  t.interval,
			NextRun:   t.nextRun,
			NoOverlap: t.noOverlap,
		}
	}
	return out
}

// Start launches the scheduler ticker loop in a background goroutine.
func (s *Scheduler) Start(parent context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.started = true

	s.wg.Add(1)
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.tick(ctx, now)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	s.mu.Lock()
	var due []*Task
	for _, t := range s.tasks {
		if !now.Before(t.nextRun) {
			t.advance(now)
			if t.noOverlap && !t.running.CompareAndSwap(false, true) {
				applog.Warn("schedule: skipping overlapping run", "task", t.name)
				continue
			}
			due = append(due, t)
		}
	}
	s.mu.Unlock()

	for _, t := range due {
		s.wg.Add(1)
		go func(task *Task) {
			defer s.wg.Done()
			if task.noOverlap {
				defer task.running.Store(false)
			}
			s.runSafe(ctx, task)
		}(t)
	}
}

func (s *Scheduler) runSafe(ctx context.Context, task *Task) {
	defer func() {
		if r := recover(); r != nil {
			applog.Error("schedule: task panicked", "task", task.name, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
		}
	}()
	if err := task.fn(ctx); err != nil {
		applog.Error("schedule: task failed", "task", task.name, "error", err)
	}
}

// Stop halts the scheduler and waits for running tasks until ctx expires.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	s.mu.Unlock()

	cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Schedule is the facade accessor returning the active *Scheduler from the container.
func Schedule() *Scheduler {
	return container.Use[*Scheduler]()
}

// Provider binds and runs the in-process Scheduler tied to the App lifecycle.
type Provider struct {
	foundation.BaseProvider
	Configure func(s *Scheduler)
}

func (p Provider) Register(app *foundation.App) error {
	container.Bind[*Scheduler](app.Container, New())
	return nil
}

func (p Provider) Boot(app *foundation.App) error {
	s := container.Resolve[*Scheduler](app.Container)
	if p.Configure != nil {
		p.Configure(s)
	}
	s.Start(context.Background())
	app.OnShutdown(s.Stop)
	return nil
}
