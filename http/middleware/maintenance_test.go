package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/http/middleware"
	router "github.com/rapide-go/rapide/routing"
)

func TestMaintenanceMode(t *testing.T) {
	maint := middleware.NewMaintenanceMode()
	r := router.New()
	r.Use(maint.Middleware())
	r.Get("/ping", func(c *ctx.Ctx) error { return c.Text("pong") })

	// 1. Up -> 200
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 when up, got %d", rec1.Code)
	}

	// 2. Down -> 503 + Retry-After
	maint.Down("let-me-in", 60)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if rec2.Code != http.StatusServiceUnavailable || rec2.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected 503 with Retry-After=60, got %d (%q)", rec2.Code, rec2.Header().Get("Retry-After"))
	}

	// 3. Secret bypass header -> 200
	req3 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req3.Header.Set("X-Maintenance-Bypass", "let-me-in")
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 with bypass header, got %d", rec3.Code)
	}

	// 4. Up again -> 200
	maint.Up()
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 after Up(), got %d", rec4.Code)
	}
}
