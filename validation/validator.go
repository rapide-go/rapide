package validation

import (
	"fmt"
	"net/mail"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Validate inspects struct `validate` tags (required, email, min=N, max=N, oneof=a b c)
// and invokes Validatable.Validate() if implemented.
func Validate(v any) error {
	errs := make(Errors)

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Struct {
		rt := rv.Type()
		for i := range rt.NumField() {
			field := rt.Field(i)
			tag := field.Tag.Get("validate")
			if tag == "" || tag == "-" {
				continue
			}

			name := fieldDisplayName(field)
			fv := rv.Field(i)
			for rule := range strings.SplitSeq(tag, ",") {
				rule = strings.TrimSpace(rule)
				if rule == "" {
					continue
				}
				applyRule(errs, name, fv, rule)
			}
		}
	}

	if custom, ok := v.(Validatable); ok {
		for k, msgs := range custom.Validate() {
			for _, m := range msgs {
				errs.Add(k, m)
			}
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

func fieldDisplayName(f reflect.StructField) string {
	if jt := f.Tag.Get("json"); jt != "" {
		if name, _, _ := strings.Cut(jt, ","); name != "" && name != "-" {
			return name
		}
	}
	return strings.ToLower(f.Name)
}

func applyRule(errs Errors, name string, fv reflect.Value, rule string) {
	key, param, _ := strings.Cut(rule, "=")

	switch key {
	case "required":
		if isEmpty(fv) {
			errs.Add(name, fmt.Sprintf("The %s field is required.", name))
		}
	case "email":
		if fv.Kind() == reflect.String && fv.String() != "" {
			if _, err := mail.ParseAddress(fv.String()); err != nil {
				errs.Add(name, fmt.Sprintf("The %s field must be a valid email address.", name))
			}
		}
	case "min":
		limit, err := strconv.Atoi(param)
		if err != nil {
			return
		}
		switch fv.Kind() {
		case reflect.String:
			if len([]rune(fv.String())) < limit {
				errs.Add(name, fmt.Sprintf("The %s field must be at least %d characters.", name, limit))
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if fv.Int() < int64(limit) {
				errs.Add(name, fmt.Sprintf("The %s field must be at least %d.", name, limit))
			}
		case reflect.Slice, reflect.Array, reflect.Map:
			if fv.Len() < limit {
				errs.Add(name, fmt.Sprintf("The %s field must have at least %d items.", name, limit))
			}
		}
	case "max":
		limit, err := strconv.Atoi(param)
		if err != nil {
			return
		}
		switch fv.Kind() {
		case reflect.String:
			if len([]rune(fv.String())) > limit {
				errs.Add(name, fmt.Sprintf("The %s field must not exceed %d characters.", name, limit))
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if fv.Int() > int64(limit) {
				errs.Add(name, fmt.Sprintf("The %s field must not be greater than %d.", name, limit))
			}
		case reflect.Slice, reflect.Array, reflect.Map:
			if fv.Len() > limit {
				errs.Add(name, fmt.Sprintf("The %s field must not have more than %d items.", name, limit))
			}
		}
	case "oneof":
		if fv.Kind() == reflect.String && fv.String() != "" {
			allowed := strings.Fields(param)
			val := fv.String()
			matched := slices.Contains(allowed, val)
			if !matched {
				errs.Add(name, fmt.Sprintf("The selected %s is invalid.", name))
			}
		}
	default:
		if custom, ok := lookupRule(key); ok {
			if msg := custom(name, fv, param); msg != "" {
				errs.Add(name, msg)
			}
		}
	}
}

func isEmpty(fv reflect.Value) bool {
	switch fv.Kind() {
	case reflect.String:
		return strings.TrimSpace(fv.String()) == ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return fv.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return fv.IsNil()
	default:
		return fv.IsZero()
	}
}
