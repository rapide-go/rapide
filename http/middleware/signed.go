package middleware

import (
	"net/http"
	"strings"

	crypt "github.com/rapide-go/rapide/encryption"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

// Signed verifies that the incoming request URL carries a valid, non-expired HMAC-SHA256
// signature produced by crypt.SignURL (provides a clean, idiomatic Go API.
func Signed(secret string) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			scheme := "http"
			if c.Request.TLS != nil || strings.EqualFold(c.Request.Header.Get("X-Forwarded-Proto"), "https") {
				scheme = "https"
			}
			host := c.Request.Host
			if host == "" {
				host = "localhost"
			}
			fullURL := scheme + "://" + host + c.Request.URL.RequestURI()
			if err := crypt.VerifyURL(fullURL, secret); err != nil {
				return &ctx.HTTPError{Code: http.StatusForbidden, Message: "invalid or expired signature"}
			}
			return next(c)
		}
	}
}
