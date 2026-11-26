package conc

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// Group runs goroutines concurrently with shared context cancellation and panic recovery
// (Go's errgroup flavour, zero external dependencies).
type Group struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
	err    error
}

// NewGroup creates a panic-safe concurrency Group derived from parent.
func NewGroup(parent context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(parent)
	return &Group{ctx: ctx, cancel: cancel}, ctx
}

// Go launches fn in a goroutine; on error or panic, cancels sibling goroutines.
func (g *Group) Go(fn func(ctx context.Context) error) {
	g.wg.Go(func() {
		if err := callSafe(g.ctx, fn); err != nil {
			g.once.Do(func() {
				g.err = err
				g.cancel()
			})
		}
	})
}

// Wait blocks until all goroutines complete and returns the first error, if any.
func (g *Group) Wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

// All runs all functions concurrently, preserves output order, and cancels siblings on first error.
func All[T any](ctx context.Context, fns ...func(context.Context) (T, error)) ([]T, error) {
	results := make([]T, len(fns))
	g, gctx := NewGroup(ctx)
	for i, fn := range fns {
		idx, f := i, fn
		g.Go(func(c context.Context) error {
			val, err := f(c)
			if err != nil {
				return err
			}
			results[idx] = val
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	_ = gctx
	return results, nil
}

// Map transforms items concurrently with at most concurrency workers, preserving order.
func Map[T, U any](ctx context.Context, items []T, concurrency int, fn func(context.Context, T) (U, error)) ([]U, error) {
	if concurrency < 1 {
		concurrency = 4
	}
	out := make([]U, len(items))
	sem := make(chan struct{}, concurrency)
	g, gctx := NewGroup(ctx)

	for i, item := range items {
		idx, it := i, item
		g.Go(func(c context.Context) error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-c.Done():
				return c.Err()
			}
			val, err := fn(c, it)
			if err != nil {
				return err
			}
			out[idx] = val
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	_ = gctx
	return out, nil
}

// Race runs fns concurrently and returns the first successful result, cancelling the rest.
func Race[T any](ctx context.Context, fns ...func(context.Context) (T, error)) (T, error) {
	var zero T
	if len(fns) == 0 {
		return zero, nil
	}
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type res struct {
		val T
		err error
	}
	ch := make(chan res, len(fns))
	for _, fn := range fns {
		f := fn
		go func() {
			var v T
			err := callSafe(rctx, func(c context.Context) error {
				var e error
				v, e = f(c)
				return e
			})
			ch <- res{val: v, err: err}
		}()
	}

	var lastErr error
	for range fns {
		r := <-ch
		if r.err == nil {
			cancel()
			return r.val, nil
		}
		lastErr = r.err
	}
	return zero, lastErr
}

func callSafe(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("conc: goroutine panicked: %v\n%s", r, string(debug.Stack()))
		}
	}()
	return fn(ctx)
}
