package schema

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rapide-go/rapide/database/dialect"
)

// Builder executes dialect-aware DDL statements against a *sql.DB.
type Builder struct {
	db      *sql.DB
	ctx     context.Context
	dialect dialect.Dialect
}

// NewBuilder wraps a *sql.DB for running schema migrations (defaults to SQLite dialect).
func NewBuilder(db *sql.DB, d ...dialect.Dialect) *Builder {
	dial := dialect.Dialect(dialect.SQLite{})
	if len(d) > 0 && d[0] != nil {
		dial = d[0]
	}
	return &Builder{db: db, ctx: context.Background(), dialect: dial}
}

// Create builds and executes a CREATE TABLE statement.
func (b *Builder) Create(name string, fn func(t *Table)) error {
	if err := dialect.ValidateIdentifier(name); err != nil {
		return err
	}
	tbl := &Table{name: name}
	fn(tbl)

	defs := make([]string, len(tbl.columns))
	for i, c := range tbl.columns {
		if err := dialect.ValidateIdentifier(c.name); err != nil {
			return err
		}
		defs[i] = c.toSQL(b.dialect)
	}
	q := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)", b.dialect.Quote(name), strings.Join(defs, ",\n  "))
	_, err := b.db.ExecContext(b.ctx, q)
	return err
}

// Drop removes a table if it exists.
func (b *Builder) Drop(name string) error {
	if err := dialect.ValidateIdentifier(name); err != nil {
		return err
	}
	_, err := b.db.ExecContext(b.ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", b.dialect.Quote(name)))
	return err
}

// AddColumn adds a new column to an existing table via ALTER TABLE.
func (b *Builder) AddColumn(table string, fn func(t *Table) *Column) error {
	if err := dialect.ValidateIdentifier(table); err != nil {
		return err
	}
	tbl := &Table{name: table}
	col := fn(tbl)
	if col == nil {
		return nil
	}
	if err := dialect.ValidateIdentifier(col.name); err != nil {
		return err
	}
	q := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", b.dialect.Quote(table), col.toSQL(b.dialect))
	_, err := b.db.ExecContext(b.ctx, q)
	return err
}

// DropColumn removes a column from an existing table.
func (b *Builder) DropColumn(table, column string) error {
	if err := dialect.ValidateIdentifier(table); err != nil {
		return err
	}
	if err := dialect.ValidateIdentifier(column); err != nil {
		return err
	}
	q := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", b.dialect.Quote(table), b.dialect.Quote(column))
	_, err := b.db.ExecContext(b.ctx, q)
	return err
}

// AddIndex creates an index (or unique index) on one or more columns of table.
func (b *Builder) AddIndex(table, indexName string, unique bool, cols ...string) error {
	if err := dialect.ValidateIdentifier(table); err != nil {
		return err
	}
	if err := dialect.ValidateIdentifier(indexName); err != nil {
		return err
	}
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		if err := dialect.ValidateIdentifier(c); err != nil {
			return err
		}
		quotedCols[i] = b.dialect.Quote(c)
	}
	kind := "INDEX"
	if unique {
		kind = "UNIQUE INDEX"
	}
	q := fmt.Sprintf("CREATE %s IF NOT EXISTS %s ON %s (%s)",
		kind, b.dialect.Quote(indexName), b.dialect.Quote(table), strings.Join(quotedCols, ", "))
	_, err := b.db.ExecContext(b.ctx, q)
	return err
}

// DropIndex drops an index by name if it exists.
func (b *Builder) DropIndex(indexName string) error {
	if err := dialect.ValidateIdentifier(indexName); err != nil {
		return err
	}
	_, err := b.db.ExecContext(b.ctx, fmt.Sprintf("DROP INDEX IF EXISTS %s", b.dialect.Quote(indexName)))
	return err
}

// RenameTable renames a table from oldName to newName.
func (b *Builder) RenameTable(oldName, newName string) error {
	if err := dialect.ValidateIdentifier(oldName); err != nil {
		return err
	}
	if err := dialect.ValidateIdentifier(newName); err != nil {
		return err
	}
	q := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", b.dialect.Quote(oldName), b.dialect.Quote(newName))
	_, err := b.db.ExecContext(b.ctx, q)
	return err
}
