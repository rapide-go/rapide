package ctx

import (
	"context"
	"fmt"
	"sync"

	applog "github.com/rapide-go/rapide/log"
)

const (
	deferCtxKey = "rapide.ctx.deferred"
	onceCtxKey  = "rapide.ctx.once"
)

var deferredWG sync.WaitGroup

// Defer schedules fn to run asynchronously AFTER the HTTP response has been sent
// (Rapide defer() equivalent). Panics are recovered and logged automatically.
func (c *Ctx) Defer(fn func(ctx context.Context)) {
	if fn == nil {
		return
	}
	var list []func(context.Context)
	if v, ok := c.Get(deferCtxKey); ok {
		list = v.([]func(context.Context))
	}
	c.Set(deferCtxKey, append(list, fn))
}

// RunDeferred executes all callbacks queued via c.Defer in a background goroutine
// tracked by deferredWG so graceful shutdown waits for them to finish.
func (c *Ctx) RunDeferred() {
	v, ok := c.Get(deferCtxKey)
	if !ok {
		return
	}
	list, _ := v.([]func(context.Context))
	if len(list) == 0 {
		return
	}
	c.Set(deferCtxKey, nil)
	attrs := c.ContextAttrs()

	deferredWG.Go(func() {
		bg := context.Background()
		for _, fn := range list {
			func() {
				defer func() {
					if r := recover(); r != nil {
						args := append([]any{"panic", fmt.Sprint(r)}, attrs...)
						applog.Error("deferred task panicked", args...)
					}
				}()
				fn(bg)
			}()
		}
	})
}

// DrainDeferred blocks until all in-flight deferred tasks complete or ctx expires.
func DrainDeferred(c context.Context) error {
	done := make(chan struct{})
	go func() {
		deferredWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-c.Done():
		return c.Err()
	}
}

// Once memoizes the result of fn under key for the lifetime of the current HTTP request
// (Rapide once() equivalent), preventing duplicate queries across middleware, controllers, and views.
func Once[T any](c *Ctx, key string, fn func() (T, error)) (T, error) {
	var zero T
	var store map[string]any
	if v, ok := c.Get(onceCtxKey); ok {
		store = v.(map[string]any)
	} else {
		store = make(map[string]any)
		c.Set(onceCtxKey, store)
	}
	if cached, ok := store[key]; ok {
		if val, ok := cached.(T); ok {
			return val, nil
		}
	}
	val, err := fn()
	if err != nil {
		return zero, err
	}
	store[key] = val
	return val, nil
}
