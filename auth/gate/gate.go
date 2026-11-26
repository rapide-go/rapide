package gate

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/rapide-go/rapide/http/ctx"
)

type abilityKey struct {
	name     string
	userType reflect.Type
	resType  reflect.Type
}

// Registry stores typed authorization abilities.
type Registry struct {
	mu        sync.RWMutex
	abilities map[abilityKey]func(user, resource any) bool
	befores   map[reflect.Type][]func(user any, ability string) (allowed bool, decided bool)
}

// NewRegistry creates an isolated authorization registry.
func NewRegistry() *Registry {
	return &Registry{
		abilities: make(map[abilityKey]func(any, any) bool),
		befores:   make(map[reflect.Type][]func(any, string) (bool, bool)),
	}
}

// Default is the global Gate registry.
var Default = NewRegistry()

// Before registers a super-admin / bypass hook evaluated before individual abilities for user type U.
// Return (true, true) to allow, (false, true) to deny, or (false, false) to defer to the ability.
func Before[U any](fn func(user U, ability string) (allowed bool, decided bool)) {
	BeforeOn[U](Default, fn)
}

// BeforeOn registers a Before hook on a specific Registry.
func BeforeOn[U any](r *Registry, fn func(user U, ability string) (bool, bool)) {
	ut := reflect.TypeFor[U]()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.befores[ut] = append(r.befores[ut], func(u any, ability string) (bool, bool) {
		return fn(u.(U), ability)
	})
}

// Define registers a type-safe authorization ability for user U and resource R.
func Define[U, R any](ability string, fn func(user U, resource R) bool) {
	DefineOn[U, R](Default, ability, fn)
}

// DefineOn registers an ability on a specific Registry.
func DefineOn[U, R any](r *Registry, ability string, fn func(user U, resource R) bool) {
	key := abilityKey{
		name:     ability,
		userType: reflect.TypeFor[U](),
		resType:  reflect.TypeFor[R](),
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.abilities[key] = func(u, res any) bool {
		return fn(u.(U), res.(R))
	}
}

// Allows reports whether user U is permitted to perform ability on resource R.
func Allows[U, R any](user U, ability string, resource R) bool {
	return AllowsOn[U, R](Default, user, ability, resource)
}

// AllowsOn evaluates an ability against a specific Registry.
func AllowsOn[U, R any](r *Registry, user U, ability string, resource R) bool {
	ut := reflect.TypeFor[U]()
	key := abilityKey{
		name:     ability,
		userType: ut,
		resType:  reflect.TypeFor[R](),
	}

	r.mu.RLock()
	befores := append([]func(any, string) (bool, bool)(nil), r.befores[ut]...)
	fn, ok := r.abilities[key]
	r.mu.RUnlock()

	for _, b := range befores {
		if allowed, decided := b(user, ability); decided {
			return allowed
		}
	}
	if !ok {
		return false
	}
	return fn(user, resource)
}

// Denies is the inverse of Allows.
func Denies[U, R any](user U, ability string, resource R) bool {
	return !Allows[U, R](user, ability, resource)
}

// Authorize returns nil if allowed, or a 403 Forbidden *ctx.HTTPError if denied.
func Authorize[U, R any](user U, ability string, resource R) error {
	if !Allows[U, R](user, ability, resource) {
		return ctx.Forbidden(fmt.Sprintf("action %q is unauthorized", ability))
	}
	return nil
}
