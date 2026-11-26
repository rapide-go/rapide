package manager

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/rapide-go/rapide/support/option"
)

// Factory lazily constructs a named driver instance of type T on first access.
type Factory[T any] func() (T, error)

type slot[T any] struct {
	mu       sync.Mutex
	factory  Factory[T]
	instance T
	resolved bool
}

// Manager is a thread-safe, generics-powered multi-instance driver manager
// .
type Manager[T any] struct {
	mu          sync.RWMutex
	defaultName string
	slots       map[string]*slot[T]
}

// New creates a Manager[T] and applies any generic option.Option[Manager[T]] configurators.
func New[T any](opts ...option.Option[Manager[T]]) *Manager[T] {
	m := &Manager[T]{
		slots: make(map[string]*slot[T]),
	}
	return option.Apply(m, opts...)
}

// SetDefault sets the active default driver name (e.g. from FILESYSTEM_DISK / CACHE_STORE).
func (m *Manager[T]) SetDefault(name string) *Manager[T] {
	m.mu.Lock()
	m.defaultName = name
	m.mu.Unlock()
	return m
}

// DefaultName returns the configured default instance name.
func (m *Manager[T]) DefaultName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultName
}

// Set registers an already-initialized named instance.
func (m *Manager[T]) Set(name string, instance T) *Manager[T] {
	m.mu.Lock()
	m.slots[name] = &slot[T]{
		instance: instance,
		resolved: true,
	}
	if m.defaultName == "" {
		m.defaultName = name
	}
	m.mu.Unlock()
	return m
}

// Register registers a lazy factory for a named instance (also aliased as Extend).
// The factory runs at most once when Get(name) is first called.
func (m *Manager[T]) Register(name string, fn Factory[T]) *Manager[T] {
	m.mu.Lock()
	m.slots[name] = &slot[T]{factory: fn}
	if m.defaultName == "" {
		m.defaultName = name
	}
	m.mu.Unlock()
	return m
}

// Extend registers a custom driver factory under name.
func (m *Manager[T]) Extend(name string, fn Factory[T]) *Manager[T] {
	return m.Register(name, fn)
}

// Get resolves and caches the named instance (uses per-slot locking to avoid deadlocks).
func (m *Manager[T]) Get(name string) (T, error) {
	var zero T
	if name == "" {
		name = m.DefaultName()
	}

	m.mu.RLock()
	s, ok := m.slots[name]
	m.mu.RUnlock()
	if !ok {
		return zero, fmt.Errorf("manager: driver %q is not registered", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resolved {
		return s.instance, nil
	}
	if s.factory == nil {
		return zero, fmt.Errorf("manager: driver %q has no factory", name)
	}

	inst, err := s.factory()
	if err != nil {
		return zero, fmt.Errorf("manager: failed to initialize driver %q: %w", name, err)
	}
	s.instance = inst
	s.resolved = true
	return inst, nil
}

// Must resolves the named instance or panics if not registered or initialization fails.
func (m *Manager[T]) Must(name string) T {
	v, err := m.Get(name)
	if err != nil {
		panic(err)
	}
	return v
}

// Default resolves the default instance or panics if unavailable.
func (m *Manager[T]) Default() T {
	return m.Must(m.DefaultName())
}

// Fake swaps the named instance with fake and returns a restore closure.
func (m *Manager[T]) Fake(name string, fake T) (restore func()) {
	if name == "" {
		name = m.DefaultName()
	}
	m.mu.Lock()
	prev, hadPrev := m.slots[name]
	m.slots[name] = &slot[T]{instance: fake, resolved: true}
	m.mu.Unlock()

	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if hadPrev {
			m.slots[name] = prev
		} else {
			delete(m.slots, name)
		}
	}
}

// Names returns a sorted list of all registered driver names.
func (m *Manager[T]) Names() []string {
	m.mu.RLock()
	names := make([]string, 0, len(m.slots))
	for k := range m.slots {
		names = append(names, k)
	}
	m.mu.RUnlock()
	slices.Sort(names)
	return names
}

// Forget removes a named instance from the manager.
func (m *Manager[T]) Forget(name string) {
	m.mu.Lock()
	delete(m.slots, name)
	m.mu.Unlock()
}

// CloseAll closes every resolved instance implementing io.Closer (hook into app.OnShutdown).
func (m *Manager[T]) CloseAll() error {
	m.mu.RLock()
	slots := make([]*slot[T], 0, len(m.slots))
	for _, s := range m.slots {
		slots = append(slots, s)
	}
	m.mu.RUnlock()

	var errs []error
	for _, s := range slots {
		s.mu.Lock()
		if s.resolved {
			if closer, ok := any(s.instance).(io.Closer); ok {
				if err := closer.Close(); err != nil {
					errs = append(errs, err)
				}
			}
		}
		s.mu.Unlock()
	}
	return errors.Join(errs...)
}
