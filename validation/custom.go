package validation

import (
	"reflect"
	"sync"
)

// CustomRuleFunc validates a struct tag rule and returns an error message (or "" when valid).
type CustomRuleFunc func(field string, value reflect.Value, param string) string

var (
	customRulesMu sync.RWMutex
	customRules   = make(map[string]CustomRuleFunc)
)

// RegisterRule registers a named custom struct-tag validation rule (e.g. `validate:"slug"`).
func RegisterRule(name string, fn CustomRuleFunc) {
	customRulesMu.Lock()
	customRules[name] = fn
	customRulesMu.Unlock()
}

func lookupRule(name string) (CustomRuleFunc, bool) {
	customRulesMu.RLock()
	fn, ok := customRules[name]
	customRulesMu.RUnlock()
	return fn, ok
}
