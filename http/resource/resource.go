package resource

import (
	"github.com/rapide-go/rapide/database/query"
)

// Transformer maps a domain model T into an API response DTO R.
type Transformer[T, R any] func(item T) R

// Item wraps a single transformed resource in a standard `{"data":...}` JSON envelope.
type Item[R any] struct {
	Data R              `json:"data"`
	Meta map[string]any `json:"meta,omitempty"`
}

// List wraps a slice of transformed resources in a standard `{"data": [...]}` JSON envelope.
type List[R any] struct {
	Data []R            `json:"data"`
	Meta map[string]any `json:"meta,omitempty"`
}

// PageMeta holds pagination metadata for Paginated[R].
type PageMeta struct {
	Total       int `json:"total"`
	PerPage     int `json:"per_page"`
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
}

// Paginated wraps a paginated slice of transformed resources in `{"data": [...], "meta": {...}}`.
type Paginated[R any] struct {
	Data []R      `json:"data"`
	Meta PageMeta `json:"meta"`
}

// Make transforms a single model T into Item[R].
func Make[T, R any](item T, fn Transformer[T, R]) Item[R] {
	return Item[R]{Data: fn(item)}
}

// Collection transforms a slice []T into List[R].
func Collection[T, R any](items []T, fn Transformer[T, R]) List[R] {
	out := make([]R, len(items))
	for i, it := range items {
		out[i] = fn(it)
	}
	return List[R]{Data: out}
}

// FromPage transforms a query.Page[T] into Paginated[R] while preserving pagination metadata.
func FromPage[T, R any](page query.Page[T], fn Transformer[T, R]) Paginated[R] {
	out := make([]R, len(page.Data))
	for i, it := range page.Data {
		out[i] = fn(it)
	}
	return Paginated[R]{
		Data: out,
		Meta: PageMeta{
			Total:       page.Total,
			PerPage:     page.PerPage,
			CurrentPage: page.CurrentPage,
			LastPage:    page.LastPage,
		},
	}
}

// When returns a pointer to val if cond is true, or nil otherwise (ideal with `json:",omitempty"`).
func When[V any](cond bool, val V) *V {
	if !cond {
		return nil
	}
	return new(val)
}

// WhenLazy evaluates fn only when cond is true, returning a pointer to its result or nil.
func WhenLazy[V any](cond bool, fn func() V) *V {
	if !cond {
		return nil
	}
	return new(fn())
}
