package view

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	alxhtml "github.com/rapide-go/rapide/http/html"
)

var (
	globalFuncsMu sync.RWMutex
	globalFuncs   = template.FuncMap{}
)

// Register registers any .alx / html.Node component function `func(...) html.Node`
// as a global Blade template helper (e.g. `view.Register("alert", ui.Alert)` -> `{{alert .Flash "default"}}`).
func Register(name string, fn any) {
	globalFuncsMu.Lock()
	defer globalFuncsMu.Unlock()
	globalFuncs[name] = wrapComponentFunc(fn)
}

func wrapComponentFunc(fn any) any {
	rv := reflect.ValueOf(fn)
	rt := rv.Type()
	nodeType := reflect.TypeFor[alxhtml.Node]()
	if rt.Kind() != reflect.Func || rt.NumOut() != 1 || !rt.Out(0).Implements(nodeType) {
		return fn
	}
	inTypes := make([]reflect.Type, rt.NumIn())
	for i := range rt.NumIn() {
		inTypes[i] = rt.In(i)
	}
	outTypes := []reflect.Type{reflect.TypeFor[template.HTML](), reflect.TypeFor[error]()}
	wrapperType := reflect.FuncOf(inTypes, outTypes, rt.IsVariadic())

	wrapped := reflect.MakeFunc(wrapperType, func(args []reflect.Value) []reflect.Value {
		var res []reflect.Value
		if rt.IsVariadic() {
			res = rv.CallSlice(args)
		} else {
			res = rv.Call(args)
		}
		var n alxhtml.Node
		if !res[0].IsNil() {
			n = res[0].Interface().(alxhtml.Node)
		}
		rendered, err := alxhtml.RenderHTML(n)
		errVal := reflect.Zero(reflect.TypeFor[error]())
		if err != nil {
			errVal = reflect.ValueOf(err)
		}
		return []reflect.Value{reflect.ValueOf(rendered), errVal}
	})
	return wrapped.Interface()
}

func defaultFuncs() template.FuncMap {
	return template.FuncMap{
		"cn": JoinClasses,
		"cond": func(ok bool, a, b any) any {
			if ok {
				return a
			}
			return b
		},
		"upper":    strings.ToUpper,
		"lower":    strings.ToLower,
		"title":    toTitle,
		"contains": strings.Contains,
		"join":     strings.Join,
		"add":      func(a, b int) int { return a + b },
		"sub":      func(a, b int) int { return a - b },
		"truncate": func(s string, maxLen int) string {
			r := []rune(s)
			if maxLen <= 0 || len(r) <= maxLen {
				return s
			}
			return string(r[:maxLen]) + "…"
		},
		"dict": func(pairs ...any) (map[string]any, error) {
			if len(pairs)%2 != 0 {
				return nil, fmt.Errorf("view: dict requires an even number of arguments")
			}
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i < len(pairs); i += 2 {
				k, ok := pairs[i].(string)
				if !ok {
					return nil, fmt.Errorf("view: dict keys must be strings")
				}
				m[k] = pairs[i+1]
			}
			return m, nil
		},
		"csrf_field": func(token string) template.HTML {
			return template.HTML(fmt.Sprintf(`<input type="hidden" name="_token" value="%s" />`, html.EscapeString(token)))
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"json": func(v any) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"date": func(t time.Time, layout string) string {
			if layout == "" {
				layout = "2006-01-02"
			}
			return t.Format(layout)
		},
		"default": func(fallback, val any) any {
			if val == nil || val == "" {
				return fallback
			}
			return val
		},
		"node": func(n alxhtml.Node) (template.HTML, error) {
			return alxhtml.RenderHTML(n)
		},
		"vite": ViteTags,
	}
}

func toTitle(s string) string {
	prevSpace := true
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			prevSpace = true
			return r
		}
		if prevSpace {
			prevSpace = false
			return unicode.ToTitle(r)
		}
		return r
	}, s)
}
