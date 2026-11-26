package router

import (
	"reflect"
	"strings"

	"github.com/rapide-go/rapide/http/ctx"
)

// RouteInfo describes a registered route for `route:list`, named route lookups, and OpenAPI generation.
type RouteInfo struct {
	Method  string
	Path    string
	Name    string
	InType  reflect.Type
	OutType reflect.Type
}

// SetName assigns a fluent name (e.g. "users.store") to this route.
func (ri *RouteInfo) SetName(name string) *RouteInfo {
	ri.Name = name
	return ri
}

// As is a short fluent alias for SetName (e.g. r.Get("/posts/{slug}",...).As("blog.show")).
func (ri *RouteInfo) As(name string) *RouteInfo {
	ri.Name = name
	return ri
}

// ResourceController defines the conventional RESTful methods for Router.Resource.
type ResourceController interface {
	Index(*ctx.Ctx) error
	Show(*ctx.Ctx) error
	Store(*ctx.Ctx) error
	Update(*ctx.Ctx) error
	Destroy(*ctx.Ctx) error
}

// Resource registers the 5 standard RESTful routes for a ResourceController
// with conventional route names (e.g. posts.index, posts.store, posts.show, posts.update, posts.destroy).
func (r *Router) Resource(base string, ctrl ResourceController, mw ...Middleware) {
	base = strings.TrimRight(base, "/")
	namePrefix := strings.TrimLeft(strings.ReplaceAll(base, "/", "."), ".")
	r.Get(base, ctrl.Index, mw...).SetName(namePrefix + ".index")
	r.Post(base, ctrl.Store, mw...).SetName(namePrefix + ".store")
	r.Get(base+"/{id}", ctrl.Show, mw...).SetName(namePrefix + ".show")
	r.Put(base+"/{id}", ctrl.Update, mw...).SetName(namePrefix + ".update")
	r.Delete(base+"/{id}", ctrl.Destroy, mw...).SetName(namePrefix + ".destroy")
}

// Named looks up a registered route by its assigned name.
func (r *Router) Named(name string) (*RouteInfo, bool) {
	for _, info := range *r.routes {
		if info.Name == name {
			return info, true
		}
	}
	return nil, false
}

// Routes returns a snapshot of all registered routes for CLI inspection or OpenAPI export.
func (r *Router) Routes() []RouteInfo {
	out := make([]RouteInfo, len(*r.routes))
	for i, info := range *r.routes {
		out[i] = *info
	}
	return out
}
