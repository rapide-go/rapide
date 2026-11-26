package router

import (
	"net/http"

	"github.com/rapide-go/rapide/http/ctx"
)

// WrapHTTP adapts any standard Go http.Handler (e.g. pprof, Prometheus metrics, http.FileServer)
// into an Rapide router.Handler.
func WrapHTTP(h http.Handler) Handler {
	return func(c *ctx.Ctx) error {
		h.ServeHTTP(c.Writer, c.Request)
		return nil
	}
}

// WrapHTTPFunc adapts any standard Go http.HandlerFunc into an Rapide router.Handler.
func WrapHTTPFunc(fn http.HandlerFunc) Handler {
	return WrapHTTP(fn)
}

// WrapMiddleware adapts any standard Go `func(http.Handler) http.Handler` middleware
// (e.g. Chi, Gorilla, OpenTelemetry, inertia-go) into an Rapide router.Middleware.
func WrapMiddleware(mw func(http.Handler) http.Handler) Middleware {
	return func(next Handler) Handler {
		return func(c *ctx.Ctx) error {
			var handlerErr error
			h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.Writer = w
				c.Request = r
				handlerErr = next(c)
			}))
			h.ServeHTTP(c.Writer, c.Request)
			return handlerErr
		}
	}
}
