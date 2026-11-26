package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rapide-go/rapide/auth"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

type verifiableUser struct {
	verified bool
}

func (u verifiableUser) HasVerifiedEmail() bool { return u.verified }

func TestEmailVerificationAndMiddleware(t *testing.T) {
	link, err := auth.VerificationURL("https://rapide.test", 42, "Ada@Example.com", "secret-key", time.Hour)
	if err != nil {
		t.Fatalf("VerificationURL error: %v", err)
	}
	if err := auth.VerifyEmailLink(link, "ada@example.com", "secret-key"); err != nil {
		t.Fatalf("expected valid link: %v", err)
	}
	if err := auth.VerifyEmailLink(link, "other@example.com", "secret-key"); err == nil {
		t.Fatal("expected email hash mismatch error")
	}

	r := router.New()
	r.Get("/secret", func(c *ctx.Ctx) error {
		return c.Text("ok")
	}, func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			auth.SetUser(c, verifiableUser{verified: c.Query("v") == "1"})
			return next(c)
		}
	}, auth.Verified())

	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/secret?v=0", nil))
	if rec1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unverified, got %d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/secret?v=1", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for verified, got %d", rec2.Code)
	}
}
