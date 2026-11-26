package schema

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rapide-go/rapide/database/dialect"
)

// Migration defines a reversible database migration.
type Migration struct {
	Name string
	Up   func(*Builder) error
	Down func(*Builder) error
}

// Migrator tracks and runs migrations against the `_migrations` table.
type Migrator struct {
	db         *sql.DB
	dialect    dialect.Dialect
	migrations []Migration
}

func NewMigrator(db *sql.DB, migrations ...Migration) *Migrator {
	return &Migrator{
		db:         db,
		dialect:    dialect.SQLite{},
		migrations: migrations,
	}
}

// WithDialect configures the SQL dialect used by the migrator.
func (m *Migrator) WithDialect(d dialect.Dialect) *Migrator {
	m.dialect = d
	return m
}

func (m *Migrator) ensureTable(ctx context.Context) error {
	q := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			%s %s,
			%s VARCHAR(255) NOT NULL UNIQUE,
			%s INTEGER NOT NULL,
			%s TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		m.dialect.Quote("_migrations"),
		m.dialect.Quote("id"), m.dialect.AutoIncrement(),
		m.dialect.Quote("name"),
		m.dialect.Quote("batch"),
		m.dialect.Quote("ran_at"),
	)
	_, err := m.db.ExecContext(ctx, q)
	return err
}

// Up runs all pending migrations in a new batch.
func (m *Migrator) Up(ctx context.Context) ([]string, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}

	applied := make(map[string]bool)
	rows, err := m.db.QueryContext(ctx, "SELECT name FROM _migrations")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		applied[name] = true
	}
	rows.Close()

	var maxBatch int
	_ = m.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(batch), 0) FROM _migrations").Scan(&maxBatch)
	nextBatch := maxBatch + 1

	builder := &Builder{db: m.db, ctx: ctx, dialect: m.dialect}
	insertSQL := fmt.Sprintf(
		"INSERT INTO _migrations (name, batch) VALUES (%s, %s)",
		m.dialect.Placeholder(1), m.dialect.Placeholder(2),
	)

	var ran []string
	for _, mig := range m.migrations {
		if applied[mig.Name] {
			continue
		}
		if err := mig.Up(builder); err != nil {
			return ran, fmt.Errorf("migration %s failed: %w", mig.Name, err)
		}
		if _, err := m.db.ExecContext(ctx, insertSQL, mig.Name, nextBatch); err != nil {
			return ran, err
		}
		ran = append(ran, mig.Name)
	}
	return ran, nil
}

// Rollback reverts the most recent batch of migrations in reverse order.
func (m *Migrator) Rollback(ctx context.Context) ([]string, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}

	var maxBatch int
	if err := m.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(batch), 0) FROM _migrations").Scan(&maxBatch); err != nil || maxBatch == 0 {
		return nil, err
	}

	lookup := make(map[string]Migration, len(m.migrations))
	for _, mig := range m.migrations {
		lookup[mig.Name] = mig
	}

	selectSQL := fmt.Sprintf("SELECT name FROM _migrations WHERE batch = %s ORDER BY id DESC", m.dialect.Placeholder(1))
	rows, err := m.db.QueryContext(ctx, selectSQL, maxBatch)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	rows.Close()

	builder := &Builder{db: m.db, ctx: ctx, dialect: m.dialect}
	deleteSQL := fmt.Sprintf("DELETE FROM _migrations WHERE name = %s", m.dialect.Placeholder(1))

	var rolled []string
	for _, name := range names {
		if mig, ok := lookup[name]; ok && mig.Down != nil {
			if err := mig.Down(builder); err != nil {
				return rolled, fmt.Errorf("rollback %s failed: %w", name, err)
			}
		}
		if _, err := m.db.ExecContext(ctx, deleteSQL, name); err != nil {
			return rolled, err
		}
		rolled = append(rolled, name)
	}
	return rolled, nil
}

// MigrationStatus describes whether a registered migration has been applied and its batch number.
type MigrationStatus struct {
	Name    string
	Applied bool
	Batch   int
}

// Status reports the applied/pending state of all registered migrations.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}
	batches := make(map[string]int)
	rows, err := m.db.QueryContext(ctx, "SELECT name, batch FROM _migrations")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			name  string
			batch int
		)
		if err := rows.Scan(&name, &batch); err != nil {
			rows.Close()
			return nil, err
		}
		batches[name] = batch
	}
	rows.Close()

	out := make([]MigrationStatus, len(m.migrations))
	for i, mig := range m.migrations {
		b, ok := batches[mig.Name]
		out[i] = MigrationStatus{Name: mig.Name, Applied: ok, Batch: b}
	}
	return out, nil
}

// Reset rolls back all applied migration batches in reverse order.
func (m *Migrator) Reset(ctx context.Context) ([]string, error) {
	var all []string
	for {
		rolled, err := m.Rollback(ctx)
		all = append(all, rolled...)
		if err != nil {
			return all, err
		}
		if len(rolled) == 0 {
			return all, nil
		}
	}
}

// Fresh resets all applied migrations and re-runs every migration from scratch.
func (m *Migrator) Fresh(ctx context.Context) ([]string, error) {
	if _, err := m.Reset(ctx); err != nil {
		return nil, err
	}
	return m.Up(ctx)
}
