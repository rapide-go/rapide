package view

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"strings"
	"sync"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/http/ctx"
)

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// Config configures the Blade-style view Engine (embedded FS for prod, live disk reload for dev).
type Config struct {
	FS      fs.FS
	Pattern string
	Dir     string
	Reload  bool
	Funcs   template.FuncMap
}

// Engine renders HTML templates with nested folders, View Composers, cached namespaces,
// buffer pooling, .alx component helpers, and optional dev live-reload.
type Engine struct {
	cfg       Config
	templates *template.Template
	nsCache   sync.Map // map[string]*template.Template
}

// New creates a view Engine supporting both prod embed.FS and dev live-reload from disk.
func New(cfg Config) (*Engine, error) {
	if cfg.Pattern == "" && cfg.Dir == "" {
		cfg.Pattern = "views"
	}
	e := &Engine{cfg: cfg}
	if !cfg.Reload {
		tpl, err := e.loadFromConfig()
		if err != nil {
			return nil, err
		}
		e.templates = tpl
	}
	return e, nil
}

// NewFromFS parses all templates under pattern/directory from an embedded fs.FS.
func NewFromFS(fsys fs.FS, pattern string) (*Engine, error) {
	return New(Config{FS: fsys, Pattern: pattern})
}

// NewFromGlob parses templates from disk matching a directory or glob pattern.
func NewFromGlob(pattern string) (*Engine, error) {
	return New(Config{Dir: pattern})
}

func (e *Engine) mergedFuncs() template.FuncMap {
	funcs := defaultFuncs()
	globalFuncsMu.RLock()
	maps.Copy(funcs, globalFuncs)
	globalFuncsMu.RUnlock()
	maps.Copy(funcs, e.cfg.Funcs)
	return funcs
}

// Render executes template name (or "namespace::path/to/view.html") with shared data & composers.
func (e *Engine) Render(c *ctx.Ctx, name string, data any) error {
	var (
		tpl     *template.Template
		err     error
		tplName = name
	)
	if ns, inner, hasNS := splitNamespace(name); hasNS {
		tpl, err = e.loadNamespace(ns)
		if err != nil {
			return err
		}
		tplName = inner
	} else {
		tpl = e.templates
		if e.cfg.Reload || tpl == nil {
			tpl, err = e.loadFromConfig()
			if err != nil {
				return err
			}
		}
	}

	resolvedData := prepareData(c, name, data)

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if err := tpl.ExecuteTemplate(buf, tplName, resolvedData); err != nil {
		return fmt.Errorf("view: render %q: %w", name, err)
	}
	return c.HTML(append([]byte(nil), buf.Bytes()...))
}

// View returns the active *Engine bound in the global container.
func View() *Engine {
	return container.Use[*Engine]()
}

// Render renders a template using the container-bound view *Engine.
func Render(c *ctx.Ctx, name string, data any) error {
	return View().Render(c, name, data)
}

// IsHTMX reports whether the request was initiated by HTMX (`HX-Request: true`).
func IsHTMX(c *ctx.Ctx) bool {
	return strings.EqualFold(c.Header("HX-Request"), "true")
}

// TypedView binds a template name to a compile-time checked Go data struct T.
type TypedView[T any] struct {
	Name string
}

// Define declares a compile-time typed Blade view (zero codegen).
func Define[T any](name string) TypedView[T] {
	return TypedView[T]{Name: name}
}

// Render executes the TypedView with strongly-typed data T.
func (tv TypedView[T]) Render(c *ctx.Ctx, data T) error {
	return Render(c, tv.Name, data)
}
