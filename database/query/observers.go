package query

import (
	"reflect"
	"sync"
)

// Observer defines lifecycle hooks for model type T.
type Observer[T any] struct {
	Creating func(item *T) error
	Created  func(item *T)
	Updating func(values map[string]any) error
	Updated  func(affected int64)
	Deleting func() error
	Deleted  func(affected int64)
}

var (
	observersMu sync.RWMutex
	observers   = make(map[reflect.Type][]any)
)

// Observe registers a model Observer[T] for lifecycle events on T.
func Observe[T any](obs Observer[T]) {
	t := reflect.TypeFor[T]()
	observersMu.Lock()
	observers[t] = append(observers[t], obs)
	observersMu.Unlock()
}

// ClearObservers removes all registered observers for T (useful in tests).
func ClearObservers[T any]() {
	t := reflect.TypeFor[T]()
	observersMu.Lock()
	delete(observers, t)
	observersMu.Unlock()
}

func getObservers[T any]() []Observer[T] {
	t := reflect.TypeFor[T]()
	observersMu.RLock()
	raw := observers[t]
	observersMu.RUnlock()
	if len(raw) == 0 {
		return nil
	}
	out := make([]Observer[T], 0, len(raw))
	for _, r := range raw {
		if obs, ok := r.(Observer[T]); ok {
			out = append(out, obs)
		}
	}
	return out
}
