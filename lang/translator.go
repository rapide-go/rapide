package lang

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"strings"
	"sync"
)

// Translator manages locale dictionaries, `:param` interpolation, and pluralization
// (Rapide Lang / __() / trans_choice equivalent).
type Translator struct {
	mu       sync.RWMutex
	fallback string
	messages map[string]map[string]string
}

// New creates a Translator with the given fallback locale (default "en").
func New(fallback string) *Translator {
	if fallback == "" {
		fallback = "en"
	}
	return &Translator{
		fallback: fallback,
		messages: make(map[string]map[string]string),
	}
}

// LoadMap merges a key->translation dictionary for a locale (e.g. "en", "fr").
func (t *Translator) LoadMap(locale string, entries map[string]string) {
	locale = normalizeLocale(locale)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.messages[locale] == nil {
		t.messages[locale] = make(map[string]string, len(entries))
	}
	maps.Copy(t.messages[locale], entries)
}

// LoadFS loads `<locale>.json` files (e.g. `en.json`, `fr.json`) from an fs.FS directory.
func (t *Translator) LoadFS(fsys fs.FS, dir string) error {
	if dir == "" {
		dir = "."
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		locale := strings.TrimSuffix(e.Name(), ".json")
		raw, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(dir, e.Name())))
		if err != nil {
			return err
		}
		var dict map[string]string
		if err := json.Unmarshal(raw, &dict); err != nil {
			return fmt.Errorf("lang: parse %s: %w", e.Name(), err)
		}
		t.LoadMap(locale, dict)
	}
	return nil
}

// Get returns the translated string for key in locale (falling back to fallback locale, then key).
func (t *Translator) Get(locale, key string, params ...map[string]any) string {
	locale = normalizeLocale(locale)
	t.mu.RLock()
	val, ok := t.messages[locale][key]
	if !ok && strings.Contains(locale, "-") {
		base, _, _ := strings.Cut(locale, "-")
		val, ok = t.messages[base][key]
	}
	if !ok {
		val, ok = t.messages[t.fallback][key]
	}
	t.mu.RUnlock()
	if !ok {
		val = key
	}
	if len(params) > 0 && params[0] != nil {
		val = interpolate(val, params[0])
	}
	return val
}

// Choice selects singular/plural variants separated by `|`.
func (t *Translator) Choice(locale, key string, count int, params ...map[string]any) string {
	raw := t.Get(locale, key)
	parts := strings.Split(raw, "|")
	var chosen string
	switch len(parts) {
	case 1:
		chosen = parts[0]
	case 2:
		if count == 1 {
			chosen = parts[0]
		} else {
			chosen = parts[1]
		}
	default:
		switch count {
		case 0:
			chosen = parts[0]
		case 1:
			chosen = parts[1]
		default:
			chosen = parts[2]
		}
	}
	merged := map[string]any{"count": count}
	if len(params) > 0 {
		maps.Copy(merged, params[0])
	}
	return interpolate(strings.TrimSpace(chosen), merged)
}

func interpolate(tpl string, params map[string]any) string {
	for k, v := range params {
		tpl = strings.ReplaceAll(tpl, ":"+k, fmt.Sprint(v))
	}
	return tpl
}

func normalizeLocale(loc string) string {
	return strings.ReplaceAll(strings.TrimSpace(strings.ToLower(loc)), "_", "-")
}
