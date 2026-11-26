package router

import (
	"net/http"
	"reflect"

	"github.com/rapide-go/rapide/http/ctx"
)

// GetTyped registers a GET route with typed output Out and records OutType on RouteInfo.
func (r *Router) GetTyped[Out any](path string, fn func(*ctx.Ctx) (Out, error), mw ...Middleware) *RouteInfo {
	info := r.Get(path, HandleOut(fn), mw...)
	info.OutType = reflect.TypeFor[Out]()
	return info
}

// PostTyped registers a POST route with typed In/Out and records both types on RouteInfo.
func (r *Router) PostTyped[In, Out any](path string, fn func(*ctx.Ctx, In) (Out, error), mw ...Middleware) *RouteInfo {
	info := r.Post(path, Handle(fn), mw...)
	info.InType = reflect.TypeFor[In]()
	info.OutType = reflect.TypeFor[Out]()
	return info
}

// PutTyped registers a PUT route with typed In/Out and records both types on RouteInfo.
func (r *Router) PutTyped[In, Out any](path string, fn func(*ctx.Ctx, In) (Out, error), mw ...Middleware) *RouteInfo {
	info := r.Put(path, Handle(fn), mw...)
	info.InType = reflect.TypeFor[In]()
	info.OutType = reflect.TypeFor[Out]()
	return info
}

// PatchTyped registers a PATCH route with typed In/Out and records both types on RouteInfo.
func (r *Router) PatchTyped[In, Out any](path string, fn func(*ctx.Ctx, In) (Out, error), mw ...Middleware) *RouteInfo {
	info := r.Patch(path, Handle(fn), mw...)
	info.InType = reflect.TypeFor[In]()
	info.OutType = reflect.TypeFor[Out]()
	return info
}

// DeleteTyped registers a DELETE route with typed output Out and records OutType on RouteInfo.
func (r *Router) DeleteTyped[Out any](path string, fn func(*ctx.Ctx) (Out, error), mw ...Middleware) *RouteInfo {
	info := r.Add(http.MethodDelete, path, HandleOut(fn), mw...)
	info.OutType = reflect.TypeFor[Out]()
	return info
}

// GetTyped delegates to r.GetTyped (kept for backwards compatibility).
func GetTyped[Out any](r *Router, path string, fn func(*ctx.Ctx) (Out, error), mw ...Middleware) *RouteInfo {
	return r.GetTyped(path, fn, mw...)
}

// PostTyped delegates to r.PostTyped (kept for backwards compatibility).
func PostTyped[In, Out any](r *Router, path string, fn func(*ctx.Ctx, In) (Out, error), mw ...Middleware) *RouteInfo {
	return r.PostTyped(path, fn, mw...)
}

// PutTyped delegates to r.PutTyped (kept for backwards compatibility).
func PutTyped[In, Out any](r *Router, path string, fn func(*ctx.Ctx, In) (Out, error), mw ...Middleware) *RouteInfo {
	return r.PutTyped(path, fn, mw...)
}

// PatchTyped delegates to r.PatchTyped (kept for backwards compatibility).
func PatchTyped[In, Out any](r *Router, path string, fn func(*ctx.Ctx, In) (Out, error), mw ...Middleware) *RouteInfo {
	return r.PatchTyped(path, fn, mw...)
}

// Handle adapts `func(*ctx.Ctx, In) (Out, error)` into a standard Handler:
//   - Decodes & validates `In` automatically (413 on oversized body, 422 JSON on validation failure)
//   - Invokes fn with zero boilerplate
//   - Serializes `Out` to JSON automatically
func Handle[In, Out any](fn func(*ctx.Ctx, In) (Out, error)) Handler {
	return func(c *ctx.Ctx) error {
		in, err := ctx.Bind[In](c)
		if err != nil {
			return err
		}
		out, err := fn(c, in)
		if err != nil {
			return err
		}
		if c.Written() {
			return nil
		}
		return c.JSON(out)
	}
}

// HandleOut adapts a handler with no request body (`func(*ctx.Ctx) (Out, error)`).
func HandleOut[Out any](fn func(*ctx.Ctx) (Out, error)) Handler {
	return func(c *ctx.Ctx) error {
		out, err := fn(c)
		if err != nil {
			return err
		}
		if c.Written() {
			return nil
		}
		return c.JSON(out)
	}
}
