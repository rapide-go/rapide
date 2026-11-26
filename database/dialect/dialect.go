package dialect

import (
	"fmt"
	"strings"
)

// Dialect abstracts SQL differences between SQLite, PostgreSQL, and MySQL.
type Dialect interface {
	Name() string
	Placeholder(index int) string
	Quote(ident string) string
	AutoIncrement() string
}

// SQLite dialect (default for Rapide's single-binary stack).
type SQLite struct{}

func (SQLite) Name() string           { return "sqlite" }
func (SQLite) Placeholder(int) string { return "?" }
func (SQLite) Quote(id string) string { return `"` + id + `"` }
func (SQLite) AutoIncrement() string  { return "INTEGER PRIMARY KEY AUTOINCREMENT" }

// Postgres dialect ($1, $2 placeholders, BIGSERIAL primary keys).
type Postgres struct{}

func (Postgres) Name() string             { return "postgres" }
func (Postgres) Placeholder(i int) string { return fmt.Sprintf("$%d", i) }
func (Postgres) Quote(id string) string   { return `"` + id + `"` }
func (Postgres) AutoIncrement() string    { return "BIGSERIAL PRIMARY KEY" }

// MySQL dialect (? placeholders, backtick quoting, AUTO_INCREMENT).
type MySQL struct{}

func (MySQL) Name() string           { return "mysql" }
func (MySQL) Placeholder(int) string { return "?" }
func (MySQL) Quote(id string) string { return "`" + id + "`" }
func (MySQL) AutoIncrement() string  { return "BIGINT AUTO_INCREMENT PRIMARY KEY" }

// ValidateIdentifier ensures a SQL table or column identifier contains only safe characters
// (letters, digits, underscore, and optional dot for table.column), preventing SQL injection.
func ValidateIdentifier(ident string) error {
	if ident == "" {
		return fmt.Errorf("orm: empty SQL identifier")
	}
	for i, r := range ident {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if i == 0 && !(isLetter || r == '_') {
			return fmt.Errorf("orm: invalid SQL identifier %q", ident)
		}
		if !(isLetter || isDigit || r == '_' || r == '.') {
			return fmt.Errorf("orm: invalid SQL identifier %q", ident)
		}
	}
	return nil
}

var allowedOperators = map[string]string{
	"=":        "=",
	"!=":       "!=",
	"<>":       "<>",
	"<":        "<",
	"<=":       "<=",
	">":        ">",
	">=":       ">=",
	"LIKE":     "LIKE",
	"NOT LIKE": "NOT LIKE",
	"ILIKE":    "ILIKE",
	"IS":       "IS",
	"IS NOT":   "IS NOT",
}

// ValidateOperator normalizes and allowlists SQL comparison operators.
func ValidateOperator(op string) (string, error) {
	norm := strings.ToUpper(strings.TrimSpace(op))
	if safe, ok := allowedOperators[norm]; ok {
		return safe, nil
	}
	return "", fmt.Errorf("orm: disallowed SQL operator %q", op)
}
