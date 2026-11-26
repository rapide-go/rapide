package sanctum

import (
	"strings"

	"github.com/rapide-go/rapide/auth"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/support/ctxkey"
)

var tokenKey = ctxkey.New[*AccessToken]()

// Middleware authenticates `Authorization: Bearer <token>` requests via Manager,
// loads typed user U, and attaches both U and *AccessToken to the request context.
func Middleware[U any](m *Manager, loader func(userID string) (U, error)) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			hdr := c.Header("Authorization")
			raw, ok := strings.CutPrefix(hdr, "Bearer ")
			if !ok || strings.TrimSpace(raw) == "" {
				return ctx.Unauthorized("missing bearer token")
			}
			tok, err := m.Verify(c.Request.Context(), raw)
			if err != nil {
				return ctx.Unauthorized("invalid or expired token")
			}
			user, err := loader(tok.UserID)
			if err != nil {
				return ctx.Unauthorized("unauthenticated")
			}
			auth.SetUser(c, user)
			c.Request = c.Request.WithContext(tokenKey.With(c.Request.Context(), tok))
			return next(c)
		}
	}
}

// CurrentToken returns the authenticated *AccessToken from the request context.
func CurrentToken(c *ctx.Ctx) *AccessToken {
	if c == nil || c.Request == nil {
		return nil
	}
	tok, _ := tokenKey.Get(c.Request.Context())
	return tok
}

// TokenCan reports whether the current request's Sanctum token has ability.
func TokenCan(c *ctx.Ctx, ability string) bool {
	return CurrentToken(c).Can(ability)
}

// RequireAbility rejects requests whose Sanctum token lacks any of the given abilities.
func RequireAbility(abilities ...string) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			tok := CurrentToken(c)
			for _, ab := range abilities {
				if !tok.Can(ab) {
					return ctx.Forbidden("token missing ability: " + ab)
				}
			}
			return next(c)
		}
	}
}
