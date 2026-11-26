package seed

import (
	"context"

	"github.com/rapide-go/rapide/database/query"
)

// Seeder populates database tables with initial or demo data.
type Seeder interface {
	Run(ctx context.Context) error
}

// SeederFunc adapts a plain function into a Seeder.
type SeederFunc func(ctx context.Context) error

func (f SeederFunc) Run(ctx context.Context) error { return f(ctx) }

// Run executes one or more seeders sequentially.
func Run(ctx context.Context, seeders ...Seeder) error {
	for _, s := range seeders {
		if err := s.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Factory generates model instances of type T for seeding and development.
type Factory[T any] struct {
	define func(index int) T
}

// NewFactory creates a typed Factory[T] from a generator function.
func NewFactory[T any](fn func(index int) T) *Factory[T] {
	return &Factory[T]{define: fn}
}

// Make builds a single in-memory instance of T without touching the database.
func (f *Factory[T]) Make(overrides ...func(*T)) T {
	item := f.define(1)
	for _, fn := range overrides {
		fn(&item)
	}
	return item
}

// MakeMany builds n in-memory instances of T.
func (f *Factory[T]) MakeMany(n int, overrides ...func(*T)) []T {
	out := make([]T, 0, n)
	for i := 1; i <= n; i++ {
		item := f.define(i)
		for _, fn := range overrides {
			fn(&item)
		}
		out = append(out, item)
	}
	return out
}

// Create builds and inserts a single instance of T via orm/query.
func (f *Factory[T]) Create(ctx context.Context, overrides ...func(*T)) (T, error) {
	item := f.Make(overrides...)
	return query.Query[T]().WithContext(ctx).Create(item)
}

// CreateMany builds and inserts n instances of T via orm/query.
func (f *Factory[T]) CreateMany(ctx context.Context, n int, overrides ...func(*T)) ([]T, error) {
	items := f.MakeMany(n, overrides...)
	created := make([]T, 0, len(items))
	for _, item := range items {
		c, err := query.Query[T]().WithContext(ctx).Create(item)
		if err != nil {
			return created, err
		}
		created = append(created, c)
	}
	return created, nil
}
