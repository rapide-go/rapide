package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"reflect"
	"strings"
	"sync"
)

// Repository is Rapide's thread-safe, generic configuration repository
// .
type Repository struct {
	mu      sync.RWMutex
	values  map[string]any
	structs map[reflect.Type]any
}

// NewRepository creates an empty configuration Repository.
func NewRepository() *Repository {
	return &Repository{
		values:  make(map[string]any),
		structs: make(map[reflect.Type]any),
	}
}

// Default is the global configuration Repository backing config.Get / config.Set / config.Use.
var Default = NewRepository()

// Set stores a value at a dot-notation key (e.g. "app.port", "database.dsn").
func (r *Repository) Set(key string, value any) *Repository {
	r.mu.Lock()
	r.values[strings.ToLower(key)] = value
	r.mu.Unlock()
	return r
}

// Has reports whether key is present in the repository.
func (r *Repository) Has(key string) bool {
	r.mu.RLock()
	_, ok := r.values[strings.ToLower(key)]
	r.mu.RUnlock()
	return ok
}

// All returns a shallow copy of all flat dot-notation keys and values.
func (r *Repository) All() map[string]any {
	r.mu.RLock()
	out := make(map[string]any, len(r.values))
	maps.Copy(out, r.values)
	r.mu.RUnlock()
	return out
}

// LoadJSONFile loads a JSON config file and flattens nested objects under prefix (e.g. "app").
// Missing files are ignored so optional config files don't break startup.
func (r *Repository) LoadJSONFile(path, prefix string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("config: parse %s: %w", path, err)
	}
	r.flattenInto(prefix, m)
	return nil
}

func (r *Repository) flattenInto(prefix string, m map[string]any) {
	for k, v := range m {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		if sub, ok := v.(map[string]any); ok {
			r.flattenInto(full, sub)
			continue
		}
		r.Set(full, v)
	}
}

// --- Global Facade Helpers (Generic, Type-Inferred) ---

// Set writes key=value into the Default global Repository.
func Set(key string, value any) { Default.Set(key, value) }

// Has reports whether key exists in the Default global Repository.
func Has(key string) bool { return Default.Has(key) }

// Get retrieves key as type T from the Default Repository.
// Fallback is optional; when provided, Go infers T automatically:
//
//	port := config.Get("app.port", 8080)       // T inferred as int
//	name := config.Get("app.name", "Alphyx")   // T inferred as string
//	port := config.Get[int]("app.port")        // fallback optional -> 0 if missing
func Get[T any](key string, fallback ...T) T {
	return GetOn[T](Default, key, fallback...)
}

// GetOn retrieves key as type T from a specific Repository.
func GetOn[T any](r *Repository, key string, fallback ...T) T {
	var def T
	if len(fallback) > 0 {
		def = fallback[0]
	}

	r.mu.RLock()
	raw, ok := r.values[strings.ToLower(key)]
	r.mu.RUnlock()
	if !ok || raw == nil {
		return def
	}
	if v, ok := raw.(T); ok {
		return v
	}
	if coerced, ok := coerceValue[T](raw); ok {
		return coerced
	}
	return def
}

// BindStruct loads typed config struct T from defaults + env, caches it on r,
// and indexes every field under `<prefix>.<snake_field>` for dot-key access via config.Get.
func BindStruct[T any](r *Repository, prefix string) (T, error) {
	cfg, err := Load[T]()
	if err != nil {
		return cfg, err
	}
	SetStruct(r, prefix, cfg)
	return cfg, nil
}

// SetStruct caches an already-built config struct T on r and indexes its fields under prefix.
func SetStruct[T any](r *Repository, prefix string, cfg T) {
	t := reflect.TypeFor[T]()
	r.mu.Lock()
	r.structs[t] = cfg
	r.mu.Unlock()

	rv := reflect.ValueOf(cfg)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}
	rt := rv.Type()
	for i := range rt.NumField() {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		key := toSnakeKey(f.Name)
		if prefix != "" {
			key = prefix + "." + key
		}
		r.Set(key, rv.Field(i).Interface())
	}
}

// Use returns the cached typed config struct T from Default (auto-loading via Load[T] if not yet bound).
func Use[T any]() T {
	return UseOn[T](Default)
}

// UseOn returns the cached typed config struct T from r (auto-loading via Load[T] if not yet bound).
func UseOn[T any](r *Repository) T {
	t := reflect.TypeFor[T]()
	r.mu.RLock()
	v, ok := r.structs[t]
	r.mu.RUnlock()
	if ok {
		return v.(T)
	}
	cfg, err := Load[T]()
	if err == nil {
		SetStruct(r, "", cfg)
	}
	return cfg
}
