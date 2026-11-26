package view

import (
	"maps"
	"strings"
	"sync"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/session"
)

// ComposerFunc enriches view data right before rendering.
type ComposerFunc func(c *ctx.Ctx, data map[string]any)

type composerEntry struct {
	pattern string
	fn      ComposerFunc
}

var (
	sharedMu  sync.RWMutex
	shared    = make(map[string]any)
	composers []composerEntry
)

// Share registers a global key/value available to every rendered view.
func Share(key string, value any) {
	sharedMu.Lock()
	shared[key] = value
	sharedMu.Unlock()
}

// Composer registers a callback invoked before rendering views matching pattern
// ("*" for all views, "blog/*" for a prefix, or an exact name like "login.html").
func Composer(pattern string, fn ComposerFunc) {
	if pattern == "" {
		pattern = "*"
	}
	sharedMu.Lock()
	composers = append(composers, composerEntry{pattern: pattern, fn: fn})
	sharedMu.Unlock()
}

func prepareData(c *ctx.Ctx, viewName string, data any) any {
	// If the caller passed a strongly-typed struct (via view.Define[T]), pass it directly.
	if data != nil {
		if _, isMap := data.(map[string]any); !isMap {
			return data
		}
	}

	merged := make(map[string]any)

	sharedMu.RLock()
	maps.Copy(merged, shared)
	matched := make([]ComposerFunc, 0, len(composers))
	for _, ce := range composers {
		if matchViewPattern(ce.pattern, viewName) {
			matched = append(matched, ce.fn)
		}
	}
	sharedMu.RUnlock()

	// Automatically expose CSRFToken and RequestPath when Ctx/Session are available.
	if c != nil {
		merged["RequestPath"] = c.Request.URL.Path
		if sess := session.Current(c); sess != nil {
			merged["CSRFToken"] = sess.CSRFToken()
		}
	}

	for _, fn := range matched {
		fn(c, merged)
	}

	if m, ok := data.(map[string]any); ok {
		maps.Copy(merged, m)
	}
	return merged
}

func matchViewPattern(pattern, name string) bool {
	if pattern == "*" || pattern == name {
		return true
	}
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(name, prefix)
	}
	return false
}
