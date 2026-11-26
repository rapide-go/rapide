package ctxkey

import "context"

// Key is a collision-free, type-safe context.Context key using Go generics.
//
// Example:
//
//	var TenantKey = ctxkey.New[Tenant]()
//	ctx = TenantKey.With(ctx, tenant)
//	tenant, ok := TenantKey.Get(ctx)
type Key[T any] struct {
	id *int
}

// New allocates a unique, type-safe context Key[T].
func New[T any]() Key[T] {
	return Key[T]{id: new(int)}
}

// With returns a derived context carrying value under this typed key.
func (k Key[T]) With(ctx context.Context, value T) context.Context {
	return context.WithValue(ctx, k.id, value)
}

// Get retrieves the typed value from ctx without unsafe type assertions.
func (k Key[T]) Get(ctx context.Context) (T, bool) {
	var zero T
	if ctx == nil {
		return zero, false
	}
	v, ok := ctx.Value(k.id).(T)
	return v, ok
}

// MustGet retrieves the typed value from ctx or returns fallback if absent.
func (k Key[T]) MustGet(ctx context.Context, fallback T) T {
	if v, ok := k.Get(ctx); ok {
		return v
	}
	return fallback
}
