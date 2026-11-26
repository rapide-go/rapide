package manager

import "github.com/rapide-go/rapide/support/option"

// WithDefault sets the default instance name returned by Manager[T].Default().
func WithDefault[T any](name string) option.Option[Manager[T]] {
	return func(m *Manager[T]) {
		m.SetDefault(name)
	}
}

// WithInstance registers a pre-built named instance on a Manager[T].
func WithInstance[T any](name string, instance T) option.Option[Manager[T]] {
	return func(m *Manager[T]) {
		m.Set(name, instance)
	}
}

// WithFactory registers a lazy named factory on a Manager[T].
func WithFactory[T any](name string, fn Factory[T]) option.Option[Manager[T]] {
	return func(m *Manager[T]) {
		m.Register(name, fn)
	}
}
