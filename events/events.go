package events

import (
	"reflect"
	"sync"
)

// Dispatcher is a type-safe, generics-powered event bus.
type Dispatcher struct {
	mu        sync.RWMutex
	listeners map[reflect.Type][]func(any)
}

// NewDispatcher creates a new isolated event dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		listeners: make(map[reflect.Type][]func(any)),
	}
}

// Default is the global application event dispatcher.
var Default = NewDispatcher()

// ListenOn registers a typed listener for event E on a specific Dispatcher.
func ListenOn[E any](d *Dispatcher, fn func(E)) {
	t := reflect.TypeFor[E]()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listeners[t] = append(d.listeners[t], func(v any) {
		fn(v.(E))
	})
}

// DispatchOn synchronously invokes all listeners registered for event E on d.
func DispatchOn[E any](d *Dispatcher, event E) {
	t := reflect.TypeFor[E]()
	d.mu.RLock()
	fns := append(([]func(any))(nil), d.listeners[t]...)
	d.mu.RUnlock()

	for _, fn := range fns {
		fn(event)
	}
}

// DispatchAsyncOn invokes all listeners for event E concurrently in goroutines.
func DispatchAsyncOn[E any](d *Dispatcher, event E) {
	t := reflect.TypeFor[E]()
	d.mu.RLock()
	fns := append(([]func(any))(nil), d.listeners[t]...)
	d.mu.RUnlock()

	for _, fn := range fns {
		go fn(event)
	}
}

// Listen registers a typed listener on the Default global event dispatcher.
func Listen[E any](fn func(E)) {
	ListenOn[E](Default, fn)
}

// Dispatch synchronously broadcasts event E on the Default global event dispatcher.
func Dispatch[E any](event E) {
	DispatchOn[E](Default, event)
}

// DispatchAsync broadcasts event E asynchronously on the Default global event dispatcher.
func DispatchAsync[E any](event E) {
	DispatchAsyncOn[E](Default, event)
}
