package container

import (
	"fmt"
	"reflect"
)

// Bind registers a concrete value or interface implementation under type T.
// Specify T explicitly when binding an interface: Bind[cache.Store](c, mem).
func Bind[T any](c *Container, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[reflect.TypeFor[T]()] = &entry{
		instance:  value,
		singleton: true,
		resolved:  true,
	}
}

// BindFactory registers a transient factory invoked on every resolution of T.
func BindFactory[T any](c *Container, fn func(*Container) (T, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[reflect.TypeFor[T]()] = &entry{
		factory: func(ct *Container) (any, error) {
			return fn(ct)
		},
		singleton: false,
	}
}

// Singleton registers a lazy factory resolved once and cached for subsequent calls.
// Factories may safely call Resolve/TryResolve for other dependencies without deadlocking.
func Singleton[T any](c *Container, fn func(*Container) (T, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[reflect.TypeFor[T]()] = &entry{
		factory: func(ct *Container) (any, error) {
			return fn(ct)
		},
		singleton: true,
	}
}

// TryResolve looks up type T in the container (or its parent chain).
// Uses per-entry locking so nested singleton factories never deadlock the container.
func TryResolve[T any](c *Container) (T, error) {
	var zero T
	t := reflect.TypeFor[T]()

	c.mu.RLock()
	e, ok := c.bindings[t]
	parent := c.parent
	c.mu.RUnlock()

	if !ok {
		if parent != nil {
			return TryResolve[T](parent)
		}
		return zero, fmt.Errorf("container: no binding registered for %v", t)
	}

	if !e.singleton {
		if e.factory == nil {
			return e.instance.(T), nil
		}
		val, err := e.factory(c)
		if err != nil {
			return zero, err
		}
		return val.(T), nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.resolved || e.factory == nil {
		return e.instance.(T), nil
	}

	val, err := e.factory(c)
	if err != nil {
		return zero, err
	}
	e.instance = val
	e.resolved = true
	return val.(T), nil
}

// Resolve returns the binding for T or panics if not registered.
func Resolve[T any](c *Container) T {
	v, err := TryResolve[T](c)
	if err != nil {
		panic(err)
	}
	return v
}

// Has reports whether type T is bound in c or any parent container.
func Has[T any](c *Container) bool {
	t := reflect.TypeFor[T]()
	for cur := c; cur != nil; cur = cur.parent {
		cur.mu.RLock()
		_, ok := cur.bindings[t]
		cur.mu.RUnlock()
		if ok {
			return true
		}
	}
	return false
}

// Fake swaps the binding for T with fake and returns a restore function.
// Usage: defer container.Fake[cache.Store](container.Default, mockStore)()
func Fake[T any](c *Container, fake T) (restore func()) {
	t := reflect.TypeFor[T]()
	c.mu.Lock()
	prev, hadPrev := c.bindings[t]
	c.bindings[t] = &entry{
		instance:  fake,
		singleton: true,
		resolved:  true,
	}
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if hadPrev {
			c.bindings[t] = prev
		} else {
			delete(c.bindings, t)
		}
	}
}
