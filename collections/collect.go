package collect

import (
	"iter"
	"slices"
)

// Collection is a modern, type-safe generic slice wrapper.
// Same-type transformations chain fluently as methods; type-changing transforms
// (Map, GroupBy, Pluck, Reduce) are free functions due to Go's method-generic rules.
type Collection[T any] struct {
	items []T
}

// From wraps a slice in a Collection[T].
func From[T any](items []T) Collection[T] {
	return Collection[T]{items: items}
}

// Of creates a Collection[T] from variadic arguments.
func Of[T any](items ...T) Collection[T] {
	return Collection[T]{items: items}
}

// Items returns the underlying slice.
func (c Collection[T]) Items() []T {
	return c.items
}

// Count returns the number of items in the collection.
func (c Collection[T]) Count() int {
	return len(c.items)
}

// IsEmpty reports whether the collection has zero elements.
func (c Collection[T]) IsEmpty() bool {
	return len(c.items) == 0
}

// First returns the first item and true, or the zero value and false if empty.
func (c Collection[T]) First() (T, bool) {
	if len(c.items) == 0 {
		var zero T
		return zero, false
	}
	return c.items[0], true
}

// Last returns the last item and true, or the zero value and false if empty.
func (c Collection[T]) Last() (T, bool) {
	if len(c.items) == 0 {
		var zero T
		return zero, false
	}
	return c.items[len(c.items)-1], true
}

// Filter returns a new Collection containing only items matching pred.
func (c Collection[T]) Filter(pred func(T) bool) Collection[T] {
	out := make([]T, 0, len(c.items))
	for _, item := range c.items {
		if pred(item) {
			out = append(out, item)
		}
	}
	return Collection[T]{items: out}
}

// Take returns a new Collection with at most the first n items.
func (c Collection[T]) Take(n int) Collection[T] {
	if n >= len(c.items) {
		return c
	}
	if n < 0 {
		n = 0
	}
	return Collection[T]{items: c.items[:n]}
}

// Sort returns a new sorted Collection using cmp (like slices.SortFunc).
func (c Collection[T]) Sort(cmp func(a, b T) int) Collection[T] {
	cp := slices.Clone(c.items)
	slices.SortFunc(cp, cmp)
	return Collection[T]{items: cp}
}

// Each invokes fn for every item and returns the same Collection for chaining.
func (c Collection[T]) Each(fn func(T)) Collection[T] {
	for _, item := range c.items {
		fn(item)
	}
	return c
}

// All returns a Go 1.23+ iter.Seq[T] so you can range directly over the collection.
func (c Collection[T]) All() iter.Seq[T] {
	return slices.Values(c.items)
}

// --- Type-changing free functions ---

// Map transforms Collection[T] into Collection[U].
func Map[T, U any](c Collection[T], fn func(T) U) Collection[U] {
	out := make([]U, len(c.items))
	for i, item := range c.items {
		out[i] = fn(item)
	}
	return Collection[U]{items: out}
}

// GroupBy groups items in Collection[T] by a comparable key K.
func GroupBy[T any, K comparable](c Collection[T], keyFn func(T) K) map[K]Collection[T] {
	groups := make(map[K][]T)
	for _, item := range c.items {
		k := keyFn(item)
		groups[k] = append(groups[k], item)
	}
	out := make(map[K]Collection[T], len(groups))
	for k, v := range groups {
		out[k] = Collection[T]{items: v}
	}
	return out
}

// Reduce folds Collection[T] into a single value of type U.
func Reduce[T, U any](c Collection[T], initial U, fn func(acc U, item T) U) U {
	acc := initial
	for _, item := range c.items {
		acc = fn(acc, item)
	}
	return acc
}
