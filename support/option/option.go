package option

// Option is a reusable generic functional option for any configurable type T.
type Option[T any] func(*T)

// Apply invokes all non-nil options against target and returns target.
func Apply[T any](target *T, opts ...Option[T]) *T {
	for _, opt := range opts {
		if opt != nil {
			opt(target)
		}
	}
	return target
}
