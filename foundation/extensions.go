package foundation

import (
	"reflect"
	"sync"
)

// LifecycleHook runs during App.Boot() (before or after providers boot).
type LifecycleHook func(app *App) error

type extensionStore struct {
	mu    sync.RWMutex
	items map[reflect.Type][]any
}

func newExtensionStore() *extensionStore {
	return &extensionStore{items: make(map[reflect.Type][]any)}
}

// AddExtension appends one or more typed extension items (e.g. migrations, route registrars,
// console commands) contributed by any service provider without creating circular imports.
func AddExtension[T any](app *App, items ...T) {
	t := reflect.TypeFor[T]()
	app.extensions.mu.Lock()
	for _, it := range items {
		app.extensions.items[t] = append(app.extensions.items[t], it)
	}
	app.extensions.mu.Unlock()
}

// Extensions returns all typed extension items of type T registered across all providers.
func Extensions[T any](app *App) []T {
	t := reflect.TypeFor[T]()
	app.extensions.mu.RLock()
	raw := app.extensions.items[t]
	out := make([]T, len(raw))
	for i, v := range raw {
		out[i] = v.(T)
	}
	app.extensions.mu.RUnlock()
	return out
}
