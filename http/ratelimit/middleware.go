package ratelimit

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

// LimiterFunc resolves a Limit dynamically for an incoming request.
type LimiterFunc func(c *ctx.Ctx) Limit

// Middleware enforces a named rate limiter registered via For(name, fn),
// emitting standard X-RateLimit-Limit, X-RateLimit-Remaining, and Retry-After headers.
func Middleware(name string) router.Middleware {
	return Default.Middleware(name)
}

// PerMinuteMiddleware creates an inline per-IP rate-limiting middleware without registering a name.
func PerMinuteMiddleware(maxAttempts int) router.Middleware {
	return Default.EnforceLimit("inline", PerMinute(maxAttempts))
}

// Middleware enforces a named rate limiter on this Limiter instance.
func (l *Limiter) Middleware(name string) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			l.mu.Lock()
			fn := l.named[name]
			l.mu.Unlock()
			if fn == nil {
				return next(c)
			}
			return l.enforce(c, name, fn(c), next)
		}
	}
}

// EnforceLimit returns middleware enforcing a fixed Limit partitioned by client IP (or lim.Key).
func (l *Limiter) EnforceLimit(scope string, lim Limit) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			return l.enforce(c, scope, lim, next)
		}
	}
}

func (l *Limiter) enforce(c *ctx.Ctx, name string, lim Limit, next router.Handler) error {
	if lim.Unlimited {
		return next(c)
	}
	key := lim.Key
	if key == "" {
		key = clientIP(c)
	}
	res := l.Hit(name+":"+key, lim)

	c.SetHeader("X-RateLimit-Limit", strconv.Itoa(res.Limit))
	c.SetHeader("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))

	if !res.Allowed {
		secs := max(int(math.Ceil(res.RetryAfter.Seconds())), 1)
		c.SetHeader("Retry-After", strconv.Itoa(secs))
		return ctx.Abort(http.StatusTooManyRequests, fmt.Sprintf("too many requests; retry after %ds", secs))
	}
	return next(c)
}

func clientIP(c *ctx.Ctx) string {
	if xff := c.Header("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return c.Request.RemoteAddr
}
