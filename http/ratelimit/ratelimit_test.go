package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/http/ratelimit"
	router "github.com/rapide-go/rapide/routing"
)

func TestNamedRateLimiterMiddleware(t *testing.T) {
	lim := ratelimit.New()
	lim.For("login", func(c *ctx.Ctx) ratelimit.Limit {
		return ratelimit.PerMinute(2).By("user@example.com")
	})

	r := router.New()
	r.Post("/login", func(c *ctx.Ctx) error {
		return c.JSON(map[string]string{"ok": "true"})
	}, lim.Middleware("login"))

	for i := 1; i <= 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		r.ServeHTTP(rec, req)

		if i <= 2 && rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: expected 200, got %d", i, rec.Code)
		}
		if i == 3 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt 3: expected 429, got %d", rec.Code)
		}
		if i == 3 && rec.Header().Get("Retry-After") == "" {
			t.Fatalf("attempt 3: missing Retry-After header")
		}
	}
}
