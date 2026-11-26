package container

// Bind registers value under type T directly on c (Go 1.27 generic method).
func (c *Container) Bind[T any](value T) {
	Bind[T](c, value)
}

// BindFactory registers a transient factory for T directly on c.
func (c *Container) BindFactory[T any](fn func(*Container) (T, error)) {
	BindFactory[T](c, fn)
}

// Singleton registers a lazy singleton factory for T directly on c.
func (c *Container) Singleton[T any](fn func(*Container) (T, error)) {
	Singleton[T](c, fn)
}

// TryResolve resolves T from c without panicking.
func (c *Container) TryResolve[T any]() (T, error) {
	return TryResolve[T](c)
}

// Resolve resolves T from c, panicking if no binding is registered.
func (c *Container) Resolve[T any]() T {
	return Resolve[T](c)
}
