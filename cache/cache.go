package cache

import (
	"sync"
	"time"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/support/manager"
	"github.com/rapide-go/rapide/support/singleflight"
)

var rememberFlight singleflight.Group[string, any]

// Store is the contract implemented by all cache drivers (Memory, SQLite, Redis, etc.).
type Store interface {
	Get(key string) (any, bool)
	Put(key string, value any, ttl time.Duration)
	Forget(key string)
	Flush()
}

// TouchableStore is optionally implemented by stores supporting in-place TTL extension.
type TouchableStore interface {
	Touch(key string, ttl time.Duration) bool
}

// Manager returns the multi-store *manager.Manager[Store] bound in the container.
func Manager() *manager.Manager[Store] {
	return container.Use[*manager.Manager[Store]]()
}

// Cache is the Rapide facade accessor for the default cache.Store.
func Cache() Store {
	if m, err := container.TryUse[*manager.Manager[Store]](); err == nil && m != nil {
		return m.Default()
	}
	return container.Use[Store]()
}

// UseStore resolves a specific named cache store (e.g. cache.UseStore("memory"), cache.UseStore("redis")).
func UseStore(name string) Store {
	return Manager().Must(name)
}

// Extend registers a custom named cache store factory.
func Extend(name string, fn manager.Factory[Store]) {
	Manager().Extend(name, fn)
}

// Get fetches a strongly-typed value of type T from the active cache Store.
func Get[T any](key string) (T, bool) {
	var zero T
	raw, ok := Cache().Get(key)
	if !ok {
		return zero, false
	}
	v, ok := raw.(T)
	return v, ok
}

// Remember returns the cached value for key if present; otherwise it invokes fn,
// stores the result for ttl, and returns it.
func Remember[T any](key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	if v, ok := Get[T](key); ok {
		return v, nil
	}
	val, err := fn()
	if err != nil {
		return val, err
	}
	Cache().Put(key, val, ttl)
	return val, nil
}

// Has reports whether key currently exists in the active cache Store.
func Has(key string) bool {
	_, ok := Cache().Get(key)
	return ok
}

// Put stores value under key for ttl in the active cache Store.
func Put(key string, value any, ttl time.Duration) {
	Cache().Put(key, value, ttl)
}

// Forget removes key from the active cache Store.
func Forget(key string) {
	Cache().Forget(key)
}

// Flush clears all entries from the active cache Store.
func Flush() {
	Cache().Flush()
}

// Touch extends key's TTL without fetching or decoding its payload.
// Returns true if the key existed and was updated.
func Touch(key string, ttl time.Duration) bool {
	st := Cache()
	if t, ok := st.(TouchableStore); ok {
		return t.Touch(key, ttl)
	}
	if v, ok := st.Get(key); ok {
		st.Put(key, v, ttl)
		return true
	}
	return false
}

// RememberCoalesced works like Remember, but uses Go singleflight so hundreds of
// concurrent cache misses for the same key execute fn only once (stampede-proof).
func RememberCoalesced[T any](key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	if v, ok := Get[T](key); ok {
		return v, nil
	}
	raw, err := rememberFlight.Do(key, func() (any, error) {
		if v, ok := Get[T](key); ok {
			return v, nil
		}
		val, err := fn()
		if err != nil {
			return nil, err
		}
		Cache().Put(key, val, ttl)
		return val, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return raw.(T), nil
}

// RememberForever caches the result of fn without expiration (ttl = 0).
func RememberForever[T any](key string, fn func() (T, error)) (T, error) {
	return Remember(key, 0, fn)
}

var counterMu sync.Mutex

// Increment atomically adds delta (default 1) to an integer counter stored at key and returns the new value.
func Increment(key string, delta ...int64) int64 {
	step := int64(1)
	if len(delta) > 0 {
		step = delta[0]
	}
	counterMu.Lock()
	defer counterMu.Unlock()

	var cur int64
	if raw, ok := Cache().Get(key); ok {
		switch v := raw.(type) {
		case int64:
			cur = v
		case int:
			cur = int64(v)
		case float64:
			cur = int64(v)
		}
	}
	next := cur + step
	Cache().Put(key, next, 0)
	return next
}

// Decrement subtracts delta (default 1) from an integer counter stored at key.
func Decrement(key string, delta ...int64) int64 {
	step := int64(1)
	if len(delta) > 0 {
		step = delta[0]
	}
	return Increment(key, -step)
}
