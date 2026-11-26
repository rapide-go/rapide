package query

import (
	"context"
	"database/sql"
	"errors"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/database/dialect"
	"github.com/rapide-go/rapide/support/manager"
)

// ErrNotFound is returned by First / Find when no row matches.
var ErrNotFound = errors.New("orm: record not found")

// Executor is satisfied by both *sql.DB and *sql.Tx so queries work identically inside transactions.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DefaultDialect controls placeholder style and identifier quoting (defaults to SQLite).
var DefaultDialect dialect.Dialect = dialect.SQLite{}

// SetDialect configures the global SQL dialect (e.g. dialect.Postgres{}, dialect.MySQL{}).
func SetDialect(d dialect.Dialect) {
	DefaultDialect = d
}

// Connections returns the multi-database *manager.Manager[*sql.DB] bound in the container,
// creating and registering one on demand if not yet present.
func Connections() *manager.Manager[*sql.DB] {
	if m, err := container.TryUse[*manager.Manager[*sql.DB]](); err == nil && m != nil {
		return m
	}
	m := manager.New[*sql.DB]()
	container.Register[*manager.Manager[*sql.DB]](m)
	return m
}

// RegisterDB registers a lazy named database connection (e.g. "default", "analytics", "replica").
func RegisterDB(name string, fn manager.Factory[*sql.DB]) {
	Connections().Register(name, fn)
}

// UseDB resolves a named *sql.DB connection (Rapide DB::connection("analytics") equivalent).
func UseDB(name string) *sql.DB {
	return Connections().Must(name)
}

// QueryConnection starts a generic query builder for model T on a specific named DB connection.
func QueryConnection[T any](connName string) *Builder[T] {
	return QueryOn[T](UseDB(connName))
}

// DB returns the default *sql.DB bound in the Rapide container.
func DB() *sql.DB {
	if m, err := container.TryUse[*manager.Manager[*sql.DB]](); err == nil && m != nil && m.DefaultName() != "" {
		return m.Default()
	}
	return container.Use[*sql.DB]()
}

// SetDB binds the default *sql.DB into the Rapide container and connection manager.
func SetDB(db *sql.DB) {
	Connections().Set("default", db).SetDefault("default")
	container.Register[*sql.DB](db)
}

// Transaction executes fn inside a database transaction, rolling back automatically on error or panic.
func Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return TransactionOn(ctx, DB(), fn)
}

// TransactionOn executes fn inside a transaction on an explicit *sql.DB.
func TransactionOn(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
