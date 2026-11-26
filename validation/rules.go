package validation

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/rapide-go/rapide/database/dialect"
	"github.com/rapide-go/rapide/database/query"
)

// Rule is a composable validation check.
type Rule interface {
	Validate(field string, value any) string
}

// RuleFunc adapts a plain function into a Rule.
type RuleFunc func(field string, value any) string

func (f RuleFunc) Validate(field string, value any) string { return f(field, value) }

// Check runs one or more Rules against field/value and records any failure into Errors.
func Check(errs Errors, field string, value any, rules ...Rule) Errors {
	if errs == nil {
		errs = make(Errors)
	}
	for _, r := range rules {
		if r == nil {
			continue
		}
		if msg := r.Validate(field, value); msg != "" {
			errs[field] = append(errs[field], msg)
		}
	}
	return errs
}

// PasswordRule provides a clean, idiomatic Go API.
type PasswordRule struct {
	min       int
	mixedCase bool
	numbers   bool
	symbols   bool
}

// Password creates a fluent password strength rule with minimum length minLen.
func Password(minLen int) PasswordRule {
	return PasswordRule{min: minLen}
}

func (p PasswordRule) MixedCase() PasswordRule { p.mixedCase = true; return p }
func (p PasswordRule) Numbers() PasswordRule   { p.numbers = true; return p }
func (p PasswordRule) Symbols() PasswordRule   { p.symbols = true; return p }

func (p PasswordRule) Validate(field string, value any) string {
	s := fmt.Sprint(value)
	if len([]rune(s)) < p.min {
		return fmt.Sprintf("%s must be at least %d characters", field, p.min)
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	if p.mixedCase && (!hasUpper || !hasLower) {
		return fmt.Sprintf("%s must contain both uppercase and lowercase letters", field)
	}
	if p.numbers && !hasDigit {
		return fmt.Sprintf("%s must contain at least one number", field)
	}
	if p.symbols && !hasSymbol {
		return fmt.Sprintf("%s must contain at least one symbol", field)
	}
	return ""
}

// InRule validates that a value is contained in an allowed generic set.
type InRule[T comparable] struct {
	Allowed []T
}

// In constructs a generic InRule[T] allowing any of the provided values.
func In[T comparable](values ...T) InRule[T] {
	return InRule[T]{Allowed: values}
}

func (r InRule[T]) Validate(field string, value any) string {
	v, ok := value.(T)
	if !ok || !slices.Contains(r.Allowed, v) {
		return fmt.Sprintf("selected %s is invalid", field)
	}
	return ""
}

// UniqueRule verifies that a column value does not already exist in table.
type UniqueRule struct {
	table     string
	column    string
	ignoreID  any
	ignoreCol string
}

// Unique constructs a UniqueRule for table.column.
func Unique(table, column string) UniqueRule {
	return UniqueRule{table: table, column: column, ignoreCol: "id"}
}

// Ignore excludes a specific record ID (useful on UPDATE forms).
func (u UniqueRule) Ignore(id any, idColumn ...string) UniqueRule {
	u.ignoreID = id
	if len(idColumn) > 0 && idColumn[0] != "" {
		u.ignoreCol = idColumn[0]
	}
	return u
}

func (u UniqueRule) Validate(field string, value any) string {
	if err := dialect.ValidateIdentifier(u.table); err != nil {
		return "invalid validation table"
	}
	if err := dialect.ValidateIdentifier(u.column); err != nil {
		return "invalid validation column"
	}
	d := query.DefaultDialect
	q := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s",
		d.Quote(u.table), d.Quote(u.column), d.Placeholder(1))
	args := []any{value}
	if u.ignoreID != nil {
		if err := dialect.ValidateIdentifier(u.ignoreCol); err == nil {
			q += fmt.Sprintf(" AND %s != %s", d.Quote(u.ignoreCol), d.Placeholder(2))
			args = append(args, u.ignoreID)
		}
	}
	var count int
	if err := query.DB().QueryRow(q, args...).Scan(&count); err == nil && count > 0 {
		return fmt.Sprintf("%s has already been taken", field)
	}
	return ""
}

// ExistsRule verifies that a column value exists in table.
type ExistsRule struct {
	table  string
	column string
}

// Exists constructs an ExistsRule for table.column.
func Exists(table, column string) ExistsRule {
	return ExistsRule{table: table, column: column}
}

func (e ExistsRule) Validate(field string, value any) string {
	if err := dialect.ValidateIdentifier(e.table); err != nil {
		return "invalid validation table"
	}
	if err := dialect.ValidateIdentifier(e.column); err != nil {
		return "invalid validation column"
	}
	d := query.DefaultDialect
	q := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s",
		d.Quote(e.table), d.Quote(e.column), d.Placeholder(1))
	var count int
	if err := query.DB().QueryRow(q, value).Scan(&count); err != nil || count == 0 {
		return fmt.Sprintf("selected %s does not exist", strings.TrimSpace(field))
	}
	return ""
}
