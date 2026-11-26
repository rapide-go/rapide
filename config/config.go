package config

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// LoadEnv reads a .env file and sets any unset environment variables.
// Missing .env files are silently ignored so production can use real env vars.
func LoadEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)

		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

// Env returns an environment variable or fallback if unset/empty.
func Env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Load populates a typed config struct T from environment variables using `env` and `default` tags.
//
// Example:
//
//	type AppConfig struct {
//	    Name  string        `env:"APP_NAME" default:"Rapide"`
//	    Port  int           `env:"APP_PORT" default:"8080"`
//	    Debug bool          `env:"APP_DEBUG" default:"true"`
//	    TTL   time.Duration `env:"CACHE_TTL" default:"5m"`
//	}
func Load[T any]() (T, error) {
	var cfg T
	v := reflect.ValueOf(&cfg).Elem()
	t := v.Type()

	if t.Kind() != reflect.Struct {
		return cfg, fmt.Errorf("config: Load[T] requires a struct type, got %s", t.Kind())
	}

	for i := range t.NumField() {
		field := t.Field(i)
		fv := v.Field(i)
		if !fv.CanSet() {
			continue
		}

		envKey := field.Tag.Get("env")
		defVal := field.Tag.Get("default")
		if envKey == "" && defVal == "" {
			continue
		}

		raw := os.Getenv(envKey)
		if raw == "" {
			raw = defVal
		}
		if raw == "" {
			continue
		}

		if err := setField(fv, raw); err != nil {
			return cfg, fmt.Errorf("config: field %s (%s): %w", field.Name, envKey, err)
		}
	}
	return cfg, nil
}

func setField(fv reflect.Value, raw string) error {
	if fv.Type() == reflect.TypeFor[time.Duration]() {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		fv.SetInt(int64(d))
		return nil
	}

	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(f)
	default:
		return fmt.Errorf("unsupported config field kind: %s", fv.Kind())
	}
	return nil
}
