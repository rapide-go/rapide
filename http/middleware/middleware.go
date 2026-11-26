package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

// Logger logs every incoming HTTP request with method, path, status code, bytes, and latency.
func Logger() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			start := time.Now()
			err := next(c)
			if err != nil && !c.Written() {
				c.WriteError(err)
				err = nil
			}
			attrs := []any{
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"status", c.StatusCode(),
				"bytes", c.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
			}
			logger := LoggerFrom(c)
			if c.StatusCode() >= 500 {
				logger.Error("http request", attrs...)
			} else {
				logger.Info("http request", attrs...)
			}
			return err
		}
	}
}

// Recover catches panics inside handlers and converts them into a 500 JSON response.
func Recover() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) (err error) {
			defer func() {
				if r := recover(); r != nil {
					LoggerFrom(c).Error("panic recovered", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
					err = ctx.Abort(http.StatusInternalServerError, "internal server error")
				}
			}()
			return next(c)
		}
	}
}
