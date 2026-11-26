package router

import (
	"net/http"
	"slices"
	"strings"

	"github.com/rapide-go/rapide/http/ctx"
)

// Handler is the standard Rapide HTTP handler signature.
type Handler func(*ctx.Ctx) error

// Middleware wraps a Handler to run logic before/after the request.
type Middleware func(next Handler) Handler

// Router wraps Go's standard net/http.ServeMux with groups, middleware pipelines,
// named routes, resource registration, and generic typed handler adapters.
type Router struct {
	mux        *http.ServeMux
	prefix     string
	middleware []Middleware
	routes     *[]*RouteInfo
}

// New creates a new root Router backed by Go's enhanced net/http.ServeMux.
func New() *Router {
	routes := make([]*RouteInfo, 0, 32)
	return &Router{
		mux:    http.NewServeMux(),
		routes: &routes,
	}
}

// Use attaches middleware to this router or route group.
func (r *Router) Use(mw ...Middleware) *Router {
	r.middleware = append(r.middleware, mw...)
	return r
}

// Group creates a sub-router sharing the same ServeMux with an added URL prefix and middleware.
func (r *Router) Group(prefix string, fn func(g *Router), mw ...Middleware) {
	sub := &Router{
		mux:        r.mux,
		prefix:     joinPath(r.prefix, prefix),
		middleware: append(append([]Middleware(nil), r.middleware...), mw...),
		routes:     r.routes,
	}
	fn(sub)
}

// Add registers a route for method + path with optional per-route middleware.
func (r *Router) Add(method, path string, h Handler, mw ...Middleware) *RouteInfo {
	fullPath := joinPath(r.prefix, path)
	pattern := method + " " + fullPath

	chain := append(append([]Middleware(nil), r.middleware...), mw...)
	wrapped := applyMiddleware(h, chain)

	lastParam := lastPathParam(fullPath)
	r.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		c := ctx.New(w, req)
		if lastParam != "" {
			c.Set(routeParamCtxKey, lastParam)
		}
		if err := wrapped(c); err != nil {
			c.WriteError(err)
		}
		c.RunDeferred()
	})

	info := &RouteInfo{
		Method: method,
		Path:   fullPath,
	}
	*r.routes = append(*r.routes, info)
	return info
}

func (r *Router) Get(path string, h Handler, mw ...Middleware) *RouteInfo {
	return r.Add(http.MethodGet, path, h, mw...)
}

func (r *Router) Post(path string, h Handler, mw ...Middleware) *RouteInfo {
	return r.Add(http.MethodPost, path, h, mw...)
}

func (r *Router) Put(path string, h Handler, mw ...Middleware) *RouteInfo {
	return r.Add(http.MethodPut, path, h, mw...)
}

func (r *Router) Patch(path string, h Handler, mw ...Middleware) *RouteInfo {
	return r.Add(http.MethodPatch, path, h, mw...)
}

func (r *Router) Delete(path string, h Handler, mw ...Middleware) *RouteInfo {
	return r.Add(http.MethodDelete, path, h, mw...)
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func applyMiddleware(h Handler, mws []Middleware) Handler {
	for _, mw := range slices.Backward(mws) {
		h = mw(h)
	}
	return h
}

func joinPath(prefix, path string) string {
	p := strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(path, "/")
	if p != "/" {
		p = strings.TrimRight(p, "/")
	}
	return p
}
