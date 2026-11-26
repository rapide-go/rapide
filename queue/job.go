package queue

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrQueueClosed is returned when pushing to a stopped/draining pool.
	ErrQueueClosed = errors.New("queue: pool is closed")
	// ErrQueueFull is returned by TryPush when the job buffer is at capacity.
	ErrQueueFull = errors.New("queue: pool buffer is full")
)

// Job is a typed background job handled by the in-process goroutine worker pool.
type Job interface {
	Handle(ctx context.Context) error
}

// JobFunc adapts a plain function into a Job.
type JobFunc func(ctx context.Context) error

func (f JobFunc) Handle(ctx context.Context) error { return f(ctx) }

// TriesJob optionally declares the total number of attempts.
type TriesJob interface {
	MaxTries() int
}

// BackoffJob optionally declares the base retry backoff duration.
type BackoffJob interface {
	Backoff() time.Duration
}

// TimeoutJob optionally declares a per-attempt execution timeout.
type TimeoutJob interface {
	Timeout() time.Duration
}
