package router

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/rapide-go/rapide/database/query"
	"github.com/rapide-go/rapide/http/ctx"
)

const routeParamCtxKey = "rapide.route.last_param"

// RouteKeyer provides a clean, idiomatic Go API.
// Implement RouteKeyName() on a model to customize which column Route Model Binding queries.
type RouteKeyer interface {
	RouteKeyName() string
}

// Model adapts a typed handler `func(*ctx.Ctx, T) error` into a standard router.Handler.
// Go infers T automatically from fn, and the route's `{param}` is resolved automatically.
//
// Example:
//
//	r.Get("/posts/{slug}", router.Model(blog.Show)).As("blog.show")
//	r.Post("/posts/{id}/delete", router.Model(blog.Destroy), webAuth).As("blog.destroy")
func Model[T any](fn func(c *ctx.Ctx, model T) error, param ...string) Handler {
	return func(c *ctx.Ctx) error {
		p := ""
		if len(param) > 0 && param[0] != "" {
			p = param[0]
		} else if v, ok := c.Get(routeParamCtxKey); ok {
			p, _ = v.(string)
		}
		if p == "" {
			var zero T
			p = strings.ToLower(reflect.TypeOf(zero).Name())
		}
		rec, err := Bind[T](c, p)
		if err != nil {
			return err
		}
		return fn(c, rec)
	}
}

// Bind resolves a route parameter into model T from the default database connection.
// By default it queries column "id", or T.RouteKeyName() if T implements RouteKeyer.
// Returns a 404 HTTPError when no matching record exists.
func Bind[T any](c *ctx.Ctx, param string, column ...string) (T, error) {
	var zero T
	val := c.Param(param)
	if val == "" {
		return zero, ctx.NotFound("resource not found")
	}

	col := "id"
	if len(column) > 0 && column[0] != "" {
		col = column[0]
	} else if rk, ok := any(zero).(RouteKeyer); ok && rk.RouteKeyName() != "" {
		col = rk.RouteKeyName()
	}

	rec, err := query.Query[T]().Where(col, "=", val).First()
	if err != nil {
		if errors.Is(err, query.ErrNotFound) {
			return zero, ctx.NotFound("resource not found")
		}
		return zero, err
	}
	return rec, nil
}

// GetBound registers a GET route with automatic generic Route Model Binding for T.
// The last `{param}` placeholder in path is automatically resolved into T.
//
// Example:
//
//	router.GetBound[models.Title](r, "/titles/{title}", func(c *ctx.Ctx, title models.Title) error {
//		return inertia.Render(c, "Titles/Show", igo.Props{"title": title})
//	})
func GetBound[T any](r *Router, path string, h func(c *ctx.Ctx, model T) error, mw ...Middleware) *RouteInfo {
	return addBound(r, http.MethodGet, path, h, mw...)
}

// PostBound registers a POST route with automatic generic Route Model Binding for T.
func PostBound[T any](r *Router, path string, h func(c *ctx.Ctx, model T) error, mw ...Middleware) *RouteInfo {
	return addBound(r, http.MethodPost, path, h, mw...)
}

// PutBound registers a PUT route with automatic generic Route Model Binding for T.
func PutBound[T any](r *Router, path string, h func(c *ctx.Ctx, model T) error, mw ...Middleware) *RouteInfo {
	return addBound(r, http.MethodPut, path, h, mw...)
}

// DeleteBound registers a DELETE route with automatic generic Route Model Binding for T.
func DeleteBound[T any](r *Router, path string, h func(c *ctx.Ctx, model T) error, mw ...Middleware) *RouteInfo {
	return addBound(r, http.MethodDelete, path, h, mw...)
}

func addBound[T any](r *Router, method, path string, h func(c *ctx.Ctx, model T) error, mw ...Middleware) *RouteInfo {
	param := lastPathParam(path)
	if param == "" {
		var zero T
		param = strings.ToLower(reflect.TypeOf(zero).Name())
	}
	return r.Add(method, path, func(c *ctx.Ctx) error {
		rec, err := Bind[T](c, param)
		if err != nil {
			return err
		}
		return h(c, rec)
	}, mw...)
}

func lastPathParam(path string) string {
	last := ""
	for {
		open := strings.IndexByte(path, '{')
		if open < 0 {
			break
		}
		close := strings.IndexByte(path[open+1:], '}')
		if close < 0 {
			break
		}
		p := path[open+1 : open+1+close]
		p = strings.TrimSuffix(p, "...")
		if p != "" {
			last = p
		}
		path = path[open+1+close+1:]
	}
	return last
}
