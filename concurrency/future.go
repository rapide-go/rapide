package conc

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Future represents the eventual result of an asynchronous goroutine computation.
type Future[T any] struct {
	done chan struct{}
	val  T
	err  error
}

// Async launches fn in a panic-safe goroutine and immediately returns a typed *Future[T].
func Async[T any](ctx context.Context, fn func(context.Context) (T, error)) *Future[T] {
	f := &Future[T]{done: make(chan struct{})}
	go func() {
		defer close(f.done)
		f.err = callSafe(ctx, func(c context.Context) error {
			var err error
			f.val, err = fn(c)
			return err
		})
	}()
	return f
}

// Await blocks until the Future completes and returns its typed value and error.
func (f *Future[T]) Await() (T, error) {
	<-f.done
	return f.val, f.err
}

// AwaitTimeout waits for the Future up to timeout, returning context.DeadlineExceeded if it times out.
func (f *Future[T]) AwaitTimeout(timeout time.Duration) (T, error) {
	var zero T
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-f.done:
		return f.val, f.err
	case <-timer.C:
		return zero, context.DeadlineExceeded
	}
}

// Done returns a channel closed when the Future finishes (for use in `select`).
func (f *Future[T]) Done() <-chan struct{} {
	return f.done
}

// --- Rapide `defer(fn)` Background Task Tracker ---

var deferredWG sync.WaitGroup

// Defer runs fn in a panic-safe background goroutine
// tracked by Drain(ctx) so in-flight deferred tasks finish cleanly on graceful shutdown.
func Defer(fn func(ctx context.Context) error) {
	deferredWG.Go(func() {
		if err := callSafe(context.Background(), fn); err != nil {
			slog.Error("conc.Defer: background task failed", "error", err)
		}
	})
}

// Drain waits for all running Defer tasks to finish until ctx expires (hook into app.OnShutdown).
func Drain(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		deferredWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
