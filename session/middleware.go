package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

// MiddlewareFromConfig creates session middleware using `session.cookie`, `session.lifetime`,
// and the container-bound default session.Store, resolved lazily after app boot.
func MiddlewareFromConfig() router.Middleware {
	var (
		storeOnce sync.Once
		mw        router.Middleware
	)
	return func(next router.Handler) router.Handler {
		var (
			wrapOnce sync.Once
			wrapped  router.Handler
		)
		return func(c *ctx.Ctx) error {
			storeOnce.Do(func() {
				cookie := config.Get("session.cookie", "rapide_session")
				ttl := config.Get("session.lifetime", 2*time.Hour)
				var st Store
				if resolved, err := tryDefaultStore(); err == nil {
					st = resolved
				}
				mw = MiddlewareWithStore(cookie, ttl, st)
			})
			wrapOnce.Do(func() {
				wrapped = mw(next)
			})
			return wrapped(c)
		}
	}
}

func tryDefaultStore() (Store, error) {
	if m, err := tryManager(); err == nil && m != nil {
		return m.Default(), nil
	}
	return nil, nil
}

// Middleware attaches a cookie-backed in-memory Session with TTL expiration.
func Middleware(cookieName string, ttl time.Duration) router.Middleware {
	return MiddlewareWithStore(cookieName, ttl, NewMemoryStore())
}

// MiddlewareWithStore attaches a Store-backed Session to each request,
// persisting mutations automatically when the handler completes.
func MiddlewareWithStore(cookieName string, ttl time.Duration, store Store) router.Middleware {
	if cookieName == "" {
		cookieName = "rapide_session"
	}
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	if store == nil {
		store = NewMemoryStore()
	}

	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			reqCtx := c.Request.Context()
			var sess *Session
			isNew := false

			if ck, err := c.Request.Cookie(cookieName); err == nil && ck.Value != "" {
				if data, exp, ok, lerr := store.Load(reqCtx, ck.Value); lerr == nil && ok {
					sess = &Session{ID: ck.Value, data: data, expiresAt: exp}
					sess.touch(ttl)
				}
			}

			if sess == nil {
				sess = newSession(randomHex(32), ttl)
				isNew = true
			}

			setSessionCookie := func(id string) {
				isHTTPS := c.Request.TLS != nil || c.Header("X-Forwarded-Proto") == "https"
				http.SetCookie(c.Writer, &http.Cookie{
					Name:     cookieName,
					Value:    id,
					Path:     "/",
					HttpOnly: true,
					Secure:   isHTTPS,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   int(ttl.Seconds()),
				})
			}

			sess.onRotate = setSessionCookie
			if isNew {
				setSessionCookie(sess.ID)
			}

			c.Request = c.Request.WithContext(sessionKey.With(reqCtx, sess))
			err := next(c)

			if oldID, _, rotated := sess.takeRotation(); rotated {
				_ = store.Delete(reqCtx, oldID)
			}

			data, exp := sess.snapshot()
			_ = store.Save(reqCtx, sess.ID, data, exp)
			return err
		}
	}
}

// VerifyCSRF validates the X-CSRF-Token header or `_token` form field on mutating requests
// using constant-time comparison. Optional except patterns (e.g. "webhooks/*", "/stripe/*")
// bypass CSRF checks (Rapide validateCsrfTokens(except:...) equivalent).
func VerifyCSRF(except ...string) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}
			if matchesAnyPath(c.Request.URL.Path, except) {
				return next(c)
			}
			sess := Current(c)
			if sess == nil {
				return ctx.Forbidden("session required for CSRF verification")
			}
			token := c.Header("X-CSRF-Token")
			if token == "" {
				token = c.Request.FormValue("_token")
			}
			expected := sess.CSRFToken()
			if len(token) == 0 || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
				return ctx.Forbidden("CSRF token mismatch")
			}
			return next(c)
		}
	}
}

func matchesAnyPath(reqPath string, patterns []string) bool {
	reqPath = strings.TrimLeft(reqPath, "/")
	for _, pat := range patterns {
		pat = strings.TrimLeft(pat, "/")
		if pat == "" {
			continue
		}
		if before, ok := strings.CutSuffix(pat, "/*"); ok {
			prefix := before
			if reqPath == prefix || strings.HasPrefix(reqPath, prefix+"/") {
				return true
			}
			continue
		}
		if ok, _ := path.Match(pat, reqPath); ok {
			return true
		}
	}
	return false
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}
