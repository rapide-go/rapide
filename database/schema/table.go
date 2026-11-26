package schema

import (
	"strings"

	"github.com/rapide-go/rapide/database/dialect"
)

// Column represents a single table column definition in a migration.
type Column struct {
	name     string
	sqlType  string
	nullable bool
	unique   bool
	defVal   string
	primary  bool
}

func (c *Column) Nullable() *Column {
	c.nullable = true
	return c
}

func (c *Column) Unique() *Column {
	c.unique = true
	return c
}

func (c *Column) Default(expr string) *Column {
	c.defVal = expr
	return c
}

func (c *Column) toSQL(d dialect.Dialect) string {
	if c.primary {
		return d.Quote(c.name) + " " + d.AutoIncrement()
	}
	var parts []string
	parts = append(parts, d.Quote(c.name), c.sqlType)
	if !c.nullable {
		parts = append(parts, "NOT NULL")
	}
	if c.unique {
		parts = append(parts, "UNIQUE")
	}
	if c.defVal != "" {
		parts = append(parts, "DEFAULT "+c.defVal)
	}
	return strings.Join(parts, " ")
}

// Table is a Rapide Blueprint-style fluent table schema definition.
type Table struct {
	name    string
	columns []*Column
}

func (t *Table) add(name, sqlType string) *Column {
	col := &Column{name: name, sqlType: sqlType}
	t.columns = append(t.columns, col)
	return col
}

func (t *Table) ID() *Column {
	c := t.add("id", "INTEGER")
	c.primary = true
	return c
}

func (t *Table) String(name string) *Column    { return t.add(name, "VARCHAR(255)") }
func (t *Table) Text(name string) *Column      { return t.add(name, "TEXT") }
func (t *Table) Integer(name string) *Column   { return t.add(name, "INTEGER") }
func (t *Table) ForeignID(name string) *Column { return t.add(name, "INTEGER") }
func (t *Table) Boolean(name string) *Column   { return t.add(name, "BOOLEAN") }

func (t *Table) Timestamps() {
	t.add("created_at", "TIMESTAMP").Default("CURRENT_TIMESTAMP")
	t.add("updated_at", "TIMESTAMP").Default("CURRENT_TIMESTAMP")
}

func (t *Table) SoftDeletes() {
	t.add("deleted_at", "TIMESTAMP").Nullable()
}
