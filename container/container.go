package container

import (
	"reflect"
	"sync"
)

// Factory creates an instance of a service on demand.
type Factory func(*Container) (any, error)

type entry struct {
	mu        sync.Mutex
	instance  any
	factory   Factory
	singleton bool
	resolved  bool
}

// Container is a thread-safe, generics-powered IoC service container.
type Container struct {
	mu       sync.RWMutex
	bindings map[reflect.Type]*entry
	parent   *Container
}

// New creates a fresh Container instance.
func New() *Container {
	return &Container{
		bindings: make(map[reflect.Type]*entry),
	}
}

// Scope creates a child container that falls back to the parent for unbound types.
// Ideal for per-request scoped bindings.
func (c *Container) Scope() *Container {
	return &Container{
		bindings: make(map[reflect.Type]*entry),
		parent:   c,
	}
}

// Flush removes all bindings from this container.
func (c *Container) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings = make(map[reflect.Type]*entry)
}
