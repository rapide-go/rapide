package conc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Task is a single unit of work in a Batch or Chain.
type Task func(ctx context.Context) error

// BatchStats summarizes the outcome of a completed Batch.
type BatchStats struct {
	Total     int
	Completed int
	Failed    int
}

// BatchBuilder provides a clean, idiomatic Go API...])->then(...)->catch(...)->finally(...)
// powered natively by bounded Go goroutines.
type BatchBuilder struct {
	tasks         []Task
	concurrency   int
	allowFailures bool
	onThen        func()
	onCatch       func(error)
	onFinally     func(BatchStats)
}

// Batch creates a fluent goroutine batch for the provided tasks.
func Batch(tasks ...Task) *BatchBuilder {
	return &BatchBuilder{
		tasks:       tasks,
		concurrency: 4,
	}
}

func (b *BatchBuilder) Concurrency(n int) *BatchBuilder {
	if n > 0 {
		b.concurrency = n
	}
	return b
}

// AllowFailures continues executing remaining tasks even if one fails (collecting all errors).
func (b *BatchBuilder) AllowFailures() *BatchBuilder {
	b.allowFailures = true
	return b
}

// Then runs only if every task in the batch succeeds.
func (b *BatchBuilder) Then(fn func()) *BatchBuilder {
	b.onThen = fn
	return b
}

// Catch runs when the batch fails with an error.
func (b *BatchBuilder) Catch(fn func(error)) *BatchBuilder {
	b.onCatch = fn
	return b
}

// Finally always runs after the batch completes (success or failure) with BatchStats.
func (b *BatchBuilder) Finally(fn func(BatchStats)) *BatchBuilder {
	b.onFinally = fn
	return b
}

// Run executes the batch concurrently and triggers Then / Catch / Finally callbacks.
func (b *BatchBuilder) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		completed atomic.Int64
		failed    atomic.Int64
		mu        sync.Mutex
		errs      []error
		wg        sync.WaitGroup
		sem       = make(chan struct{}, b.concurrency)
	)

	for _, task := range b.tasks {
		t := task
		wg.Go(func() {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-runCtx.Done():
				failed.Add(1)
				return
			}

			if err := callSafe(runCtx, t); err != nil {
				failed.Add(1)
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				if !b.allowFailures {
					cancel()
				}
				return
			}
			completed.Add(1)
		})
	}

	wg.Wait()

	stats := BatchStats{
		Total:     len(b.tasks),
		Completed: int(completed.Load()),
		Failed:    int(failed.Load()),
	}
	finalErr := errors.Join(errs...)
	if finalErr == nil {
		if b.onThen != nil {
			b.onThen()
		}
	} else if b.onCatch != nil {
		b.onCatch(finalErr)
	}
	if b.onFinally != nil {
		b.onFinally(stats)
	}
	return finalErr
}

// ChainBuilder runs tasks sequentially, stopping on the first error.
type ChainBuilder struct {
	tasks []Task
}

// Chain creates a sequential task chain.
func Chain(tasks ...Task) *ChainBuilder {
	return &ChainBuilder{tasks: tasks}
}

// Run executes each task in order, aborting on error or context cancellation.
func (c *ChainBuilder) Run(ctx context.Context) error {
	for _, t := range c.tasks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := callSafe(ctx, t); err != nil {
			return err
		}
	}
	return nil
}
