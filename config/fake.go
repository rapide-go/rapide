package config

import (
	"reflect"
	"strings"
)

// Fake temporarily overrides a dot-key in Default and returns a restore function.
func Fake(key string, value any) (restore func()) {
	lk := strings.ToLower(key)
	Default.mu.Lock()
	prev, had := Default.values[lk]
	Default.values[lk] = value
	Default.mu.Unlock()
	return func() {
		Default.mu.Lock()
		if had {
			Default.values[lk] = prev
		} else {
			delete(Default.values, lk)
		}
		Default.mu.Unlock()
	}
}

// FakeStruct temporarily overrides a typed config struct T in Default and returns a restore function.
func FakeStruct[T any](fake T) (restore func()) {
	t := reflect.TypeFor[T]()
	Default.mu.Lock()
	prev, had := Default.structs[t]
	Default.structs[t] = fake
	Default.mu.Unlock()
	return func() {
		Default.mu.Lock()
		if had {
			Default.structs[t] = prev
		} else {
			delete(Default.structs, t)
		}
		Default.mu.Unlock()
	}
}
