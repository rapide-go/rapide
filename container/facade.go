package container

// Default is the global application container backing Rapide facades.
var Default = New()

// Register binds a value into the global Default container.
func Register[T any](value T) {
	Bind[T](Default, value)
}

// RegisterSingleton registers a lazy singleton into the global Default container.
func RegisterSingleton[T any](fn func(*Container) (T, error)) {
	Singleton[T](Default, fn)
}

// Use resolves type T from the global Default container, panicking if missing.
func Use[T any]() T {
	return Resolve[T](Default)
}

// TryUse resolves type T from the global Default container without panicking.
func TryUse[T any]() (T, error) {
	return TryResolve[T](Default)
}
