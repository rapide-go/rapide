package view

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"
	"sync"
)

type namespaceSource struct {
	fsys    fs.FS
	pattern string
}

var (
	namespacesMu sync.RWMutex
	namespaces   = make(map[string]namespaceSource)
)

// RegisterNamespace mounts a third-party package's embedded views under a fluent namespace
// (e.g. `view.RegisterNamespace("billing", billingFS, "views")` -> `view.Render(c, "billing::invoice.html", data)`).
func RegisterNamespace(name string, fsys fs.FS, pattern string) {
	if pattern == "" {
		pattern = "views"
	}
	namespacesMu.Lock()
	namespaces[name] = namespaceSource{fsys: fsys, pattern: pattern}
	namespacesMu.Unlock()
}

func (e *Engine) loadNamespace(ns string) (*template.Template, error) {
	if !e.cfg.Reload {
		if cached, ok := e.nsCache.Load(ns); ok {
			return cached.(*template.Template), nil
		}
	}

	namespacesMu.RLock()
	src, ok := namespaces[ns]
	namespacesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("view: unknown namespace %q", ns)
	}

	funcs := e.mergedFuncs()
	var (
		tpl *template.Template
		err error
	)
	if strings.ContainsAny(src.pattern, "*?[") {
		dir := path.Dir(src.pattern)
		if dir != "" && dir != "." {
			tpl, err = parseTree(src.fsys, dir, funcs)
		} else {
			tpl, err = template.New("").Funcs(funcs).ParseFS(src.fsys, src.pattern)
		}
	} else {
		tpl, err = parseTree(src.fsys, src.pattern, funcs)
	}
	if err != nil {
		return nil, err
	}
	if !e.cfg.Reload {
		e.nsCache.Store(ns, tpl)
	}
	return tpl, nil
}

func splitNamespace(name string) (ns, tplName string, hasNS bool) {
	if before, after, ok := strings.Cut(name, "::"); ok {
		return before, after, true
	}
	return "", name, false
}
