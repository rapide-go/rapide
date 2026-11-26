package channel

import (
	"context"
	"errors"
	"iter"
	"sync"
)

// ErrClosed is returned when sending to a closed Channel.
var ErrClosed = errors.New("channel: closed")

// Channel is a panic-safe, generic wrapper around a Go chan T with idempotent Close,
// context-aware Send/Receive, and Go 1.23+ iter.Seq[T] iteration.
type Channel[T any] struct {
	ch     chan T
	mu     sync.RWMutex
	closed bool
	once   sync.Once
}

// New creates a buffered (or unbuffered when size <= 0) generic Channel[T].
func New[T any](size int) *Channel[T] {
	if size < 0 {
		size = 0
	}
	return &Channel[T]{ch: make(chan T, size)}
}

// Send delivers val to the channel or aborts if ctx is cancelled or the channel is closed.
func (c *Channel[T]) Send(ctx context.Context, val T) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ErrClosed
	}
	select {
	case c.ch <- val:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TrySend attempts a non-blocking send, returning false if the buffer is full or closed.
func (c *Channel[T]) TrySend(val T) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return false
	}
	select {
	case c.ch <- val:
		return true
	default:
		return false
	}
}

// Receive waits for the next value or returns (zero, false) if closed or ctx is cancelled.
func (c *Channel[T]) Receive(ctx context.Context) (T, bool) {
	var zero T
	select {
	case v, ok := <-c.ch:
		return v, ok
	case <-ctx.Done():
		return zero, false
	}
}

// Close closes the channel safely (idempotent — calling Close multiple times never panics).
func (c *Channel[T]) Close() {
	c.once.Do(func() {
		c.mu.Lock()
		c.closed = true
		close(c.ch)
		c.mu.Unlock()
	})
}

// Raw exposes the underlying receive-only Go channel for use in custom `select` statements.
func (c *Channel[T]) Raw() <-chan T {
	return c.ch
}

// All returns a Go 1.23+ iter.Seq[T] draining the channel until it is closed.
func (c *Channel[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range c.ch {
			if !yield(v) {
				return
			}
		}
	}
}

// Pipe transforms values from in into a new Channel[U] in a background goroutine.
func Pipe[T, U any](ctx context.Context, in *Channel[T], fn func(T) U) *Channel[U] {
	out := New[U](cap(in.ch))
	go func() {
		defer out.Close()
		for {
			v, ok := in.Receive(ctx)
			if !ok {
				return
			}
			if err := out.Send(ctx, fn(v)); err != nil {
				return
			}
		}
	}()
	return out
}

// Merge fans-in multiple input channels into a single output Channel[T].
func Merge[T any](ctx context.Context, inputs ...*Channel[T]) *Channel[T] {
	out := New[T](len(inputs) * 8)
	var wg sync.WaitGroup
	for _, in := range inputs {
		ch := in
		wg.Go(func() {
			for {
				v, ok := ch.Receive(ctx)
				if !ok {
					return
				}
				if err := out.Send(ctx, v); err != nil {
					return
				}
			}
		})
	}
	go func() {
		wg.Wait()
		out.Close()
	}()
	return out
}

// FanOut broadcasts every item from in to n independent output channels.
func FanOut[T any](ctx context.Context, in *Channel[T], n int) []*Channel[T] {
	if n < 1 {
		n = 1
	}
	outs := make([]*Channel[T], n)
	for i := range n {
		outs[i] = New[T](cap(in.ch))
	}
	go func() {
		defer func() {
			for _, o := range outs {
				o.Close()
			}
		}()
		for {
			v, ok := in.Receive(ctx)
			if !ok {
				return
			}
			for _, o := range outs {
				if err := o.Send(ctx, v); err != nil {
					return
				}
			}
		}
	}()
	return outs
}
