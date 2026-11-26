package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func coerceValue[T any](raw any) (T, bool) {
	var zero T
	targetType := reflect.TypeFor[T]()
	s := fmt.Sprint(raw)

	if targetType == reflect.TypeFor[time.Duration]() {
		if d, err := time.ParseDuration(s); err == nil {
			return any(d).(T), true
		}
		return zero, false
	}

	out := reflect.New(targetType).Elem()
	switch out.Kind() {
	case reflect.String:
		out.SetString(s)
		return out.Interface().(T), true
	case reflect.Bool:
		if b, err := strconv.ParseBool(s); err == nil {
			out.SetBool(b)
			return out.Interface().(T), true
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if f, ok := raw.(float64); ok {
			out.SetInt(int64(f))
			return out.Interface().(T), true
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			out.SetInt(n)
			return out.Interface().(T), true
		}
	case reflect.Float32, reflect.Float64:
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			out.SetFloat(f)
			return out.Interface().(T), true
		}
	}
	return zero, false
}

func toSnakeKey(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			sb.WriteByte('_')
		}
		sb.WriteRune(unicode.ToLower(r))
	}
	return sb.String()
}
