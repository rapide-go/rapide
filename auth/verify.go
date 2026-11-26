package auth

import (
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	crypt "github.com/rapide-go/rapide/encryption"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

// Verifiable is implemented by user models supporting email verification.
type Verifiable interface {
	HasVerifiedEmail() bool
}

// EmailHash returns the deterministic SHA-1 hash of a normalized email address.
func EmailHash(email string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// VerificationURL builds a temporary signed email verification URL for userID + email.
func VerificationURL(baseURL string, userID any, email, secret string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	raw := fmt.Sprintf("%s/email/verify/%v/%s", strings.TrimRight(baseURL, "/"), userID, EmailHash(email))
	return crypt.SignURL(raw, secret, ttl)
}

// VerifyEmailLink validates both the signed URL signature/expiration and the expected email hash.
func VerifyEmailLink(rawURL, expectedEmail, secret string) error {
	if err := crypt.VerifyURL(rawURL, secret); err != nil {
		return err
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 1 {
		return errors.New("auth: invalid verification path")
	}
	gotHash := parts[len(parts)-1]
	wantHash := EmailHash(expectedEmail)
	if subtle.ConstantTimeCompare([]byte(gotHash), []byte(wantHash)) != 1 {
		return errors.New("auth: verification email hash mismatch")
	}
	return nil
}

// Verified creates a middleware ensuring the authenticated user implements Verifiable
// and has verified their email.
func Verified() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			raw, ok := userKey.Get(c.Request.Context())
			if !ok || raw == nil {
				return ctx.Unauthorized("unauthenticated")
			}
			u, ok := raw.(Verifiable)
			if !ok || !u.HasVerifiedEmail() {
				return ctx.Forbidden("email address is not verified")
			}
			return next(c)
		}
	}
}
