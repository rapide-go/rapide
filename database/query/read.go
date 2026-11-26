package query

import (
	"errors"
	"fmt"
	"iter"

	"github.com/rapide-go/rapide/support/errs"
)

// Find fetches a single record by primary key id.
func (b *Builder[T]) Find(id any) (T, error) {
	return b.Where("id", "=", id).First()
}

// FindOrFail fetches a single record by primary key id, wrapping ErrNotFound in an HTTP 404 errs.NotFound error
// .
func (b *Builder[T]) FindOrFail(id any) (T, error) {
	return b.Where("id", "=", id).FirstOrFail()
}

// First executes the query with LIMIT 1 and returns the first matching record.
func (b *Builder[T]) First() (T, error) {
	var zero T
	items, err := b.Limit(1).Get()
	if err != nil {
		return zero, err
	}
	if len(items) == 0 {
		return zero, ErrNotFound
	}
	return items[0], nil
}

// FirstOrFail executes the query with LIMIT 1 and returns an HTTP 404 errs.NotFound error
// (wrapping ErrNotFound) when no row matches.
func (b *Builder[T]) FirstOrFail() (T, error) {
	item, err := b.First()
	if errors.Is(err, ErrNotFound) {
		return item, errs.NotFound(b.table+" not found", ErrNotFound)
	}
	return item, err
}

// Get executes the SELECT query and scans all matching rows into []T.
func (b *Builder[T]) Get() ([]T, error) {
	var results []T
	for item, err := range b.Cursor() {
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}

// Cursor streams rows lazily using Go's iter.Seq2[T, error], using cached struct metadata for fast scanning.
func (b *Builder[T]) Cursor() iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		if b.err != nil {
			var zero T
			yield(zero, b.err)
			return
		}

		querySQL, args := b.buildSelect()
		rows, err := b.exec.QueryContext(b.ctx, querySQL, args...)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}

		for rows.Next() {
			var item T
			if err := b.meta.scan(rows, cols, &item); err != nil {
				var zero T
				yield(zero, err)
				return
			}
			if !yield(item, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			var zero T
			yield(zero, err)
		}
	}
}

// Count returns the total number of rows matching the current WHERE clauses.
func (b *Builder[T]) Count() (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	whereSQL, args := b.buildWhere(1)
	q := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", b.dialect.Quote(b.table), whereSQL)
	var count int
	if err := b.exec.QueryRowContext(b.ctx, q, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Exists reports whether at least one row matches the current query.
func (b *Builder[T]) Exists() (bool, error) {
	n, err := b.Clone().Count()
	return n > 0, err
}

// FirstOrCreate returns the first record matching the current WHERE clauses,
// or inserts and returns item if no record exists.
func (b *Builder[T]) FirstOrCreate(item T) (T, error) {
	found, err := b.Clone().First()
	if err == nil {
		return found, nil
	}
	if err != ErrNotFound {
		return found, err
	}
	return b.Clone().Create(item)
}

// Paginate returns a Page[T] for the given 1-based page number and perPage size.
func (b *Builder[T]) Paginate(page, perPage int) (Page[T], error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	total, err := b.Clone().Count()
	if err != nil {
		return Page[T]{}, err
	}
	items, err := b.Clone().Limit(perPage).Offset((page - 1) * perPage).Get()
	if err != nil {
		return Page[T]{}, err
	}
	lastPage := max((total+perPage-1)/perPage, 1)
	return Page[T]{
		Data:        items,
		Total:       total,
		PerPage:     perPage,
		CurrentPage: page,
		LastPage:    lastPage,
	}, nil
}
