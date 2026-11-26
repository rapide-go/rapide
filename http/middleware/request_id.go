package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"

	"github.com/rapide-go/rapide/http/ctx"
	applog "github.com/rapide-go/rapide/log"
	"github.com/rapide-go/rapide/routing"
)

const (
	requestIDKey = "rapide.request_id"
	reqLoggerKey = "rapide.logger"
)

// RequestID reads or generates an X-Request-ID header, echoes it on the response,
// and attaches a request-scoped *slog.Logger with `request_id` to ctx.Ctx.
func RequestID() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			id := c.Header("X-Request-ID")
			if id == "" {
				b := make([]byte, 8)
				_, _ = io.ReadFull(rand.Reader, b)
				id = hex.EncodeToString(b)
			}
			c.SetHeader("X-Request-ID", id)
			c.Set(requestIDKey, id)
			c.Set(reqLoggerKey, applog.Log().With("request_id", id))
			return next(c)
		}
	}
}

// LoggerFrom retrieves the request-scoped *slog.Logger (with request_id and any c.AddContext fields)
// from c, falling back to the global Rapide logger.
func LoggerFrom(c *ctx.Ctx) *slog.Logger {
	base := applog.Log()
	if v, ok := c.Get(reqLoggerKey); ok {
		if l, ok := v.(*slog.Logger); ok && l != nil {
			base = l
		}
	}
	if attrs := c.ContextAttrs(); len(attrs) > 0 {
		return base.With(attrs...)
	}
	return base
}
