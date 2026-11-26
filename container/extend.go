package container

import (
	"fmt"
	"reflect"
)

// Extend wraps an existing binding for type T with decorator.
// Works for both pre-bound instances and lazy/transient factories.
func Extend[T any](c *Container, decorator func(T) T) error {
	t := reflect.TypeFor[T]()

	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.bindings[t]
	if !ok {
		return fmt.Errorf("container: cannot extend unbound type %v", t)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.resolved && e.singleton {
		e.instance = decorator(e.instance.(T))
		return nil
	}

	prevFactory := e.factory
	e.factory = func(ct *Container) (any, error) {
		val, err := prevFactory(ct)
		if err != nil {
			return nil, err
		}
		return decorator(val.(T)), nil
	}
	return nil
}
