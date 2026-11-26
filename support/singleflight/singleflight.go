package singleflight

import (
	"fmt"
	"runtime/debug"
	"sync"
)

type call[V any] struct {
	wg  sync.WaitGroup
	val V
	err error
}

// Group suppresses duplicate concurrent function calls for the same key K,
// preventing cache-stampede / thundering-herd storms (panic-safe & generic).
type Group[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*call[V]
}

// Do executes fn for key, coalescing any concurrent callers with the same key
// so fn runs only once and all callers share the resulting (V, error).
// Panics in fn are recovered and converted into an error delivered to all waiters.
func (g *Group[K, V]) Do(key K, fn func() (V, error)) (V, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[K]*call[V])
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	c := &call[V]{}
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			c.err = fmt.Errorf("singleflight: panicked: %v\n%s", r, string(debug.Stack()))
		}
		g.mu.Lock()
		delete(g.calls, key)
		g.mu.Unlock()
		c.wg.Done()
	}()

	c.val, c.err = fn()
	return c.val, c.err
}
