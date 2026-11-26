package stream

import "iter"

// Filter lazily yields only elements of seq that satisfy pred (zero intermediate allocations).
func Filter[T any](seq iter.Seq[T], pred func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if pred(v) && !yield(v) {
				return
			}
		}
	}
}

// Map lazily transforms each element of seq from T to U.
func Map[T, U any](seq iter.Seq[T], fn func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(fn(v)) {
				return
			}
		}
	}
}

// Take yields at most the first n elements from seq.
func Take[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		count := 0
		for v := range seq {
			if !yield(v) {
				return
			}
			count++
			if count >= n {
				return
			}
		}
	}
}

// Chunk groups elements of seq into slices of at most size elements (ideal for batch DB inserts).
func Chunk[T any](seq iter.Seq[T], size int) iter.Seq[[]T] {
	if size < 1 {
		size = 1
	}
	return func(yield func([]T) bool) {
		batch := make([]T, 0, size)
		for v := range seq {
			batch = append(batch, v)
			if len(batch) == size {
				if !yield(batch) {
					return
				}
				batch = make([]T, 0, size)
			}
		}
		if len(batch) > 0 {
			yield(batch)
		}
	}
}

// Collect materializes a lazy iter.Seq[T] into a concrete slice []T.
func Collect[T any](seq iter.Seq[T]) []T {
	var out []T
	for v := range seq {
		out = append(out, v)
	}
	return out
}
