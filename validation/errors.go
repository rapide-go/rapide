package validation

import (
	"fmt"
	"strings"
)

// Errors maps field names to one or more human-readable validation error messages,
// matching Rapide's 422 Unprocessable Entity JSON payload shape.
type Errors map[string][]string

func (e Errors) Error() string {
	var parts []string
	for field, msgs := range e {
		parts = append(parts, fmt.Sprintf("%s: %s", field, strings.Join(msgs, ", ")))
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Add appends a validation message for a field.
func (e Errors) Add(field, message string) {
	e[field] = append(e[field], message)
}

// HasErrors reports whether any validation errors were recorded.
func (e Errors) HasErrors() bool {
	return len(e) > 0
}

// Validatable can be implemented by request DTOs to add custom cross-field validation rules
// .
type Validatable interface {
	Validate() Errors
}
