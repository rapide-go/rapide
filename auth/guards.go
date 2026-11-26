package auth

import (
	"net/http"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/session"
)

const sessionUserIDKey = "rapide.auth.user_id"

// Login regenerates the session ID (preventing session fixation) and stores the authenticated user's ID.
func Login(c *ctx.Ctx, userID any) error {
	sess := session.Current(c)
	if sess == nil {
		return ctx.Abort(500, "session middleware is required for auth.Login")
	}
	sess.Regenerate()
	sess.Set(sessionUserIDKey, userID)
	return nil
}

// Logout invalidates the active session and rotates its ID.
func Logout(c *ctx.Ctx) {
	if sess := session.Current(c); sess != nil {
		sess.Invalidate()
	}
}

// SessionUserID returns the raw user ID stored in the active session, if any.
func SessionUserID(c *ctx.Ctx) (any, bool) {
	sess := session.Current(c)
	if sess == nil {
		return nil, false
	}
	id, ok := sess.Get(sessionUserIDKey)
	if !ok || id == nil {
		return nil, false
	}
	return id, true
}

// ResolveSession returns the authenticated user U from the request context or lazily loads it
// from the active session via loader, caching the resolved user on ctx.Ctx for subsequent calls.
func ResolveSession[U any](c *ctx.Ctx, loader func(userID any) (U, error)) (U, bool) {
	if u, ok := User[U](c); ok {
		return u, true
	}
	var zero U
	id, ok := SessionUserID(c)
	if !ok {
		return zero, false
	}
	user, err := loader(id)
	if err != nil {
		return zero, false
	}
	SetUser(c, user)
	return user, true
}

// Check reports whether an authenticated user of type U is present on the request context.
func Check[U any](c *ctx.Ctx) bool {
	_, ok := User[U](c)
	return ok
}

// Guest reports whether no authenticated user of type U is present on the request context.
func Guest[U any](c *ctx.Ctx) bool {
	return !Check[U](c)
}

// Session creates a stateful web authentication middleware backed by http/session.
// It reads the logged-in user ID from the session, loads the typed user U via loader,
// and attaches U to ctx.Ctx (retrievable via auth.User[U](c) / auth.MustUser[U](c)).
func Session[U any](loader func(userID any) (U, error)) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			if session.Current(c) == nil {
				return ctx.Unauthorized("session required")
			}
			if _, ok := ResolveSession(c, loader); !ok {
				return ctx.Unauthorized("unauthenticated")
			}
			return next(c)
		}
	}
}

// SessionRedirect enforces stateful session auth like Session, redirecting unauthenticated
// browser requests to loginURL (or returning 401 when the client expects JSON).
func SessionRedirect[U any](loginURL string, loader func(userID any) (U, error)) router.Middleware {
	if loginURL == "" {
		loginURL = "/login"
	}
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			if _, ok := ResolveSession(c, loader); !ok {
				if c.WantsJSON() {
					return ctx.Unauthorized("unauthenticated")
				}
				return c.Redirect(http.StatusFound, loginURL)
			}
			return next(c)
		}
	}
}

// APIKey creates a middleware that authenticates requests via an API key header (e.g. "X-API-Key")
// and attaches the resolved typed principal U to ctx.Ctx.
func APIKey[U any](headerName string, resolver func(apiKey string) (U, error)) router.Middleware {
	if headerName == "" {
		headerName = "X-API-Key"
	}
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			key := c.Header(headerName)
			if key == "" {
				return ctx.Unauthorized("missing API key")
			}
			principal, err := resolver(key)
			if err != nil {
				return ctx.Unauthorized("invalid API key")
			}
			SetUser(c, principal)
			return next(c)
		}
	}
}
