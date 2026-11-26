package queue

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	applog "github.com/rapide-go/rapide/log"
	"github.com/rapide-go/rapide/support/option"
)

type envelope struct {
	job        Job
	maxRetries int
	backoff    time.Duration
}

// FailedHandler is invoked when a job exhausts all retry attempts (dead-letter hook).
type FailedHandler func(job Job, err error)

// Pool is an in-process goroutine queue worker pool with panic recovery and graceful draining.
type Pool struct {
	workers  int
	ch       chan envelope
	wg       sync.WaitGroup
	cancel   context.CancelFunc
	onFailed FailedHandler

	mu      sync.RWMutex
	started bool
	closed  bool
}

// OnFailed registers a dead-letter callback invoked whenever a job exhausts its retries.
func (p *Pool) OnFailed(fn FailedHandler) *Pool {
	p.mu.Lock()
	p.onFailed = fn
	p.mu.Unlock()
	return p
}

// PushAfter schedules a job to be enqueued after delay (non-blocking; no-op if pool is closed).
func (p *Pool) PushAfter(delay time.Duration, job Job, retries ...int) {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()
	if closed {
		return
	}
	if delay <= 0 {
		_ = p.Push(job, retries...)
		return
	}
	time.AfterFunc(delay, func() {
		_ = p.Push(job, retries...)
	})
}

// PoolConfig configures a worker Pool via generic option.Option[PoolConfig].
type PoolConfig struct {
	Workers int
	Buffer  int
}

func WithWorkers(n int) option.Option[PoolConfig] { return func(c *PoolConfig) { c.Workers = n } }
func WithBuffer(n int) option.Option[PoolConfig]  { return func(c *PoolConfig) { c.Buffer = n } }

// NewPoolWithOptions creates a Pool configured via generic option.Option[PoolConfig].
func NewPoolWithOptions(opts ...option.Option[PoolConfig]) *Pool {
	cfg := PoolConfig{Workers: 4, Buffer: 256}
	option.Apply(&cfg, opts...)
	return NewPool(cfg.Workers, cfg.Buffer)
}

// NewPool creates a worker pool with the given concurrency and buffer size.
func NewPool(workers, buffer int) *Pool {
	if workers < 1 {
		workers = 4
	}
	if buffer < 1 {
		buffer = 256
	}
	return &Pool{
		workers: workers,
		ch:      make(chan envelope, buffer),
	}
}

// Start launches the background worker goroutines.
func (p *Pool) Start(parent context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started || p.closed {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	p.started = true

	for range p.workers {
		p.wg.Add(1)
		go p.worker(ctx)
	}
}

func (p *Pool) worker(ctx context.Context) {
	defer p.wg.Done()
	for env := range p.ch {
		p.execute(ctx, env)
	}
}

func (p *Pool) execute(ctx context.Context, env envelope) {
	attempts := env.maxRetries + 1
	if tj, ok := env.job.(TriesJob); ok && tj.MaxTries() > 0 {
		attempts = tj.MaxTries()
	}
	backoff := env.backoff
	if bj, ok := env.job.(BackoffJob); ok && bj.Backoff() > 0 {
		backoff = bj.Backoff()
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx := ctx
		var cancel context.CancelFunc
		if to, ok := env.job.(TimeoutJob); ok && to.Timeout() > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, to.Timeout())
		}
		err := p.safeHandle(attemptCtx, env.job)
		if cancel != nil {
			cancel()
		}
		if err == nil {
			return
		}
		lastErr = err
		applog.Error("queue: job failed", "attempt", attempt, "max_attempts", attempts, "error", err)
		if attempt < attempts {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff * time.Duration(attempt)):
			}
		}
	}
	p.mu.RLock()
	cb := p.onFailed
	p.mu.RUnlock()
	if cb != nil && lastErr != nil {
		cb(env.job, lastErr)
	}
}

// safeHandle invokes job.Handle with panic recovery so a crashing job never kills the server.
func (p *Pool) safeHandle(ctx context.Context, job Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			applog.Error("queue: job panicked", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
			err = fmt.Errorf("job panicked: %v", r)
		}
	}()
	return job.Handle(ctx)
}

// Push enqueues a job, blocking if the buffer is full. Returns ErrQueueClosed if shutting down.
func (p *Pool) Push(job Job, retries ...int) error {
	maxRetries := 3
	if len(retries) > 0 {
		maxRetries = retries[0]
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrQueueClosed
	}
	p.ch <- envelope{
		job:        job,
		maxRetries: maxRetries,
		backoff:    500 * time.Millisecond,
	}
	return nil
}

// TryPush enqueues a job without blocking; returns ErrQueueFull if the buffer is saturated.
func (p *Pool) TryPush(job Job, retries ...int) error {
	maxRetries := 3
	if len(retries) > 0 {
		maxRetries = retries[0]
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrQueueClosed
	}
	select {
	case p.ch <- envelope{
		job:        job,
		maxRetries: maxRetries,
		backoff:    500 * time.Millisecond,
	}:
		return nil
	default:
		return ErrQueueFull
	}
}

// Stop closes job intake and drains queued jobs gracefully until ctx expires.
func (p *Pool) Stop(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	started := p.started
	cancel := p.cancel
	close(p.ch)
	p.mu.Unlock()

	if !started {
		return nil
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		cancel()
		return nil
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
}
