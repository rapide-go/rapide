package config

// Get reads key from r as type T, returning fallback if missing (Go 1.27 generic method).
func (r *Repository) Get[T any](key string, fallback ...T) T {
	return GetOn[T](r, key, fallback...)
}

// BindStruct loads T from env/defaults and binds it under prefix on r.
func (r *Repository) BindStruct[T any](prefix string) (T, error) {
	return BindStruct[T](r, prefix)
}

// Use retrieves the bound struct T from r.
func (r *Repository) Use[T any]() T {
	return UseOn[T](r)
}
