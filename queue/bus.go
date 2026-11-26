package queue

import (
	"context"
	"sync"
)

// Chain executes a sequence of Jobs strictly in order, stopping at the first error
// .
type Chain struct {
	jobs    []Job
	onCatch func(error)
}

// NewChain creates a sequential job chain.
func NewChain(jobs ...Job) *Chain {
	return &Chain{jobs: jobs}
}

// Catch registers a callback invoked if any job in the chain fails.
func (c *Chain) Catch(fn func(error)) *Chain {
	c.onCatch = fn
	return c
}

// Handle implements Job so a Chain can be dispatched onto any queue Pool.
func (c *Chain) Handle(ctx context.Context) error {
	for _, job := range c.jobs {
		if err := ctx.Err(); err != nil {
			if c.onCatch != nil {
				c.onCatch(err)
			}
			return err
		}
		if err := job.Handle(ctx); err != nil {
			if c.onCatch != nil {
				c.onCatch(err)
			}
			return err
		}
	}
	return nil
}

// Batch runs a group of Jobs concurrently and triggers Then / Catch / Finally lifecycle hooks
// .
type Batch struct {
	jobs      []Job
	onThen    func()
	onCatch   func(error)
	onFinally func()
}

// NewBatch creates a concurrent job batch.
func NewBatch(jobs ...Job) *Batch {
	return &Batch{jobs: jobs}
}

// Then registers a callback invoked only if every job in the batch succeeds.
func (b *Batch) Then(fn func()) *Batch {
	b.onThen = fn
	return b
}

// Catch registers a callback invoked on the first job failure.
func (b *Batch) Catch(fn func(error)) *Batch {
	b.onCatch = fn
	return b
}

// Finally registers a callback always invoked after all jobs finish.
func (b *Batch) Finally(fn func()) *Batch {
	b.onFinally = fn
	return b
}

// Handle implements Job so a Batch can be dispatched onto any queue Pool.
func (b *Batch) Handle(ctx context.Context) error {
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	for _, job := range b.jobs {
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			if err := j.Handle(ctx); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(job)
	}
	wg.Wait()

	if firstErr != nil {
		if b.onCatch != nil {
			b.onCatch(firstErr)
		}
	} else if b.onThen != nil {
		b.onThen()
	}
	if b.onFinally != nil {
		b.onFinally()
	}
	return firstErr
}
