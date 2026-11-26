package query

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/rapide-go/rapide/database/dialect"
)

type whereClause struct {
	boolean string // "AND" or "OR"
	col     string
	op      string
	raw     string
	args    []any
}

// Page represents a fluent paginated query result.
type Page[T any] struct {
	Data        []T `json:"data"`
	Total       int `json:"total"`
	PerPage     int `json:"per_page"`
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
}

// Builder is a generic, Eloquent-style fluent SQL query builder for model T.
type Builder[T any] struct {
	exec        Executor
	dialect     dialect.Dialect
	ctx         context.Context
	meta        *typeMeta
	table       string
	columns     []string
	wheres      []whereClause
	orders      []string
	limit       *int
	offset      *int
	withTrashed bool
	err         error
}

// Query starts a new generic query builder for model T using the container's default DB.
func Query[T any]() *Builder[T] {
	return QueryOn[T](DB())
}

// QueryTx starts a new generic query builder bound to an active *sql.Tx transaction.
func QueryTx[T any](tx *sql.Tx) *Builder[T] {
	return QueryOn[T](tx)
}

// QueryOn starts a new generic query builder for model T on any Executor (*sql.DB or *sql.Tx).
func QueryOn[T any](exec Executor) *Builder[T] {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	meta := getTypeMeta[T](t)
	return &Builder[T]{
		exec:    exec,
		dialect: DefaultDialect,
		ctx:     context.Background(),
		meta:    meta,
		table:   meta.table,
	}
}

// Clone returns an independent copy of the Builder so base scopes can be reused safely.
func (b *Builder[T]) Clone() *Builder[T] {
	cp := *b
	cp.columns = slices.Clone(b.columns)
	cp.wheres = slices.Clone(b.wheres)
	cp.orders = slices.Clone(b.orders)
	if b.limit != nil {
		cp.limit = new(*b.limit)
	}
	if b.offset != nil {
		cp.offset = new(*b.offset)
	}
	return &cp
}

func (b *Builder[T]) WithContext(ctx context.Context) *Builder[T] {
	b.ctx = ctx
	return b
}

// Table overrides the target SQL table name for this query after validating the identifier.
func (b *Builder[T]) Table(name string) *Builder[T] {
	if err := dialect.ValidateIdentifier(name); err != nil {
		b.err = err
		return b
	}
	b.table = name
	return b
}

func (b *Builder[T]) WithDialect(d dialect.Dialect) *Builder[T] {
	b.dialect = d
	return b
}

// Select restricts the columns returned by SELECT.
func (b *Builder[T]) Select(cols ...string) *Builder[T] {
	for _, col := range cols {
		if err := dialect.ValidateIdentifier(col); err != nil {
			b.setErr(err)
			return b
		}
		b.columns = append(b.columns, col)
	}
	return b
}

// Where appends an AND condition. Supports either Where("col", "=", val) or Where("col", val).
func (b *Builder[T]) Where(col string, opOrVal any, val ...any) *Builder[T] {
	return b.addWhere("AND", col, opOrVal, val...)
}

// OrWhere appends an OR condition.
func (b *Builder[T]) OrWhere(col string, opOrVal any, val ...any) *Builder[T] {
	return b.addWhere("OR", col, opOrVal, val...)
}

// WhereIn appends a `col IN (?, ?,...)` condition.
func (b *Builder[T]) WhereIn(col string, values []any) *Builder[T] {
	if err := dialect.ValidateIdentifier(col); err != nil {
		b.setErr(err)
		return b
	}
	if len(values) == 0 {
		b.wheres = append(b.wheres, whereClause{boolean: "AND", raw: "1 = 0"})
		return b
	}
	b.wheres = append(b.wheres, whereClause{
		boolean: "AND",
		col:     col,
		op:      "IN",
		args:    values,
	})
	return b
}

// WhereNull appends a `col IS NULL` condition.
func (b *Builder[T]) WhereNull(col string) *Builder[T] {
	if err := dialect.ValidateIdentifier(col); err != nil {
		b.setErr(err)
		return b
	}
	b.wheres = append(b.wheres, whereClause{
		boolean: "AND",
		raw:     b.dialect.Quote(col) + " IS NULL",
	})
	return b
}

// WhereNotNull appends a `col IS NOT NULL` condition.
func (b *Builder[T]) WhereNotNull(col string) *Builder[T] {
	if err := dialect.ValidateIdentifier(col); err != nil {
		b.setErr(err)
		return b
	}
	b.wheres = append(b.wheres, whereClause{
		boolean: "AND",
		raw:     b.dialect.Quote(col) + " IS NOT NULL",
	})
	return b
}

func (b *Builder[T]) addWhere(boolean, col string, opOrVal any, val ...any) *Builder[T] {
	if err := dialect.ValidateIdentifier(col); err != nil {
		b.setErr(err)
		return b
	}

	op := "="
	arg := opOrVal
	if len(val) > 0 {
		op = fmt.Sprint(opOrVal)
		arg = val[0]
	}
	safeOp, err := dialect.ValidateOperator(op)
	if err != nil {
		b.setErr(err)
		return b
	}

	b.wheres = append(b.wheres, whereClause{
		boolean: boolean,
		col:     col,
		op:      safeOp,
		args:    []any{arg},
	})
	return b
}

// OrderBy appends an ORDER BY clause (e.g. OrderBy("created_at", "DESC")).
func (b *Builder[T]) OrderBy(col string, dir ...string) *Builder[T] {
	if err := dialect.ValidateIdentifier(col); err != nil {
		b.setErr(err)
		return b
	}
	direction := "ASC"
	if len(dir) > 0 && strings.EqualFold(dir[0], "DESC") {
		direction = "DESC"
	}
	b.orders = append(b.orders, b.dialect.Quote(col)+" "+direction)
	return b
}

// Latest orders descending by created_at (or custom column).
func (b *Builder[T]) Latest(col ...string) *Builder[T] {
	c := "created_at"
	if len(col) > 0 {
		c = col[0]
	}
	return b.OrderBy(c, "DESC")
}

func (b *Builder[T]) Limit(n int) *Builder[T] {
	b.limit = &n
	return b
}

func (b *Builder[T]) Offset(n int) *Builder[T] {
	b.offset = &n
	return b
}

func (b *Builder[T]) WithTrashed() *Builder[T] {
	b.withTrashed = true
	return b
}

func (b *Builder[T]) setErr(err error) {
	if b.err == nil {
		b.err = err
	}
}
