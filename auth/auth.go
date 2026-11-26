package auth

import (
	"context"
	"strings"

	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/support/ctxkey"
)

var userKey = ctxkey.New[any]()

// TokenResolver validates a bearer token and returns the authenticated user value of type U.
type TokenResolver[U any] func(token string) (U, error)

// WithUser attaches an authenticated user to a standard context.Context (also used by testkit.ActingAs).
func WithUser(parent context.Context, user any) context.Context {
	return userKey.With(parent, user)
}

// SetUser attaches an authenticated user to the request context.
func SetUser(c *ctx.Ctx, user any) {
	c.Request = c.Request.WithContext(WithUser(c.Request.Context(), user))
}

// User retrieves the authenticated user of type U from the request context.
func User[U any](c *ctx.Ctx) (U, bool) {
	var zero U
	if c == nil || c.Request == nil {
		return zero, false
	}
	raw, ok := userKey.Get(c.Request.Context())
	if !ok {
		return zero, false
	}
	u, ok := raw.(U)
	return u, ok
}

// MustUser retrieves the authenticated user of type U or returns a 401 Unauthorized error.
func MustUser[U any](c *ctx.Ctx) (U, error) {
	u, ok := User[U](c)
	if !ok {
		return u, ctx.Unauthorized("unauthenticated")
	}
	return u, nil
}

// Bearer creates a type-safe authentication middleware that extracts `Authorization: Bearer <token>`,
// resolves it via resolver, and stores the typed user U on the request Ctx.
func Bearer[U any](resolver TokenResolver[U]) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			header := c.Header("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || strings.TrimSpace(token) == "" {
				return ctx.Unauthorized("missing bearer token")
			}

			user, err := resolver(strings.TrimSpace(token))
			if err != nil {
				return ctx.Unauthorized("invalid token")
			}

			SetUser(c, user)
			return next(c)
		}
	}
}
