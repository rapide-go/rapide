package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/http/middleware"
	router "github.com/rapide-go/rapide/routing"
)

func TestSecurityHeaders(t *testing.T) {
	r := router.New()
	r.Use(middleware.SecurityHeaders(middleware.WithCSP("default-src 'self'")))
	r.Get("/ok", func(c *ctx.Ctx) error { return c.Text("ok") })

	// Plain HTTP -> no HSTS
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if rec1.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
	if rec1.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing X-Frame-Options")
	}
	if rec1.Header().Get("Content-Security-Policy") != "default-src 'self'" {
		t.Fatal("missing CSP")
	}
	if rec1.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("did not expect HSTS on plain HTTP")
	}

	// HTTPS via reverse proxy -> HSTS attached
	req2 := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req2.Header.Set("X-Forwarded-Proto", "https")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("expected HSTS on HTTPS request")
	}
}
