package ctx_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rapide-go/rapide/http/ctx"
)

func TestSafeRedirectBlocksOpenRedirects(t *testing.T) {
	cases := []struct {
		target string
		want   string
	}{
		{"/dashboard", "/dashboard"},
		{"/posts/hello?tab=1", "/posts/hello?tab=1"},
		{"https://evil.com/phish", "/"},
		{"//evil.com/phish", "/"},
		{"/\\evil.com", "/"},
		{"/ok\r\nX-Injected: 1", "/"},
		{"", "/"},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := ctx.New(rec, req)
		_ = c.SafeRedirect(http.StatusFound, tc.target, "/")
		if got := rec.Header().Get("Location"); got != tc.want {
			t.Fatalf("SafeRedirect(%q): expected %q, got %q", tc.target, tc.want, got)
		}
	}
}

func TestBindRejectsTrailingJSONSmuggling(t *testing.T) {
	type reqDTO struct {
		Name string `json:"name" validate:"required"`
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"ada"}{"name":"evil"}`))
	c := ctx.New(rec, req)
	if _, err := ctx.Bind[reqDTO](c); err == nil {
		t.Fatal("expected error for trailing JSON smuggling payload")
	}

	// Strict binding rejects unknown fields
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"ada","admin":true}`))
	c2 := ctx.New(rec2, req2)
	if _, err := ctx.BindStrict[reqDTO](c2); err == nil {
		t.Fatal("expected BindStrict to reject unknown field")
	}
}

func TestDeferOnceAndContextBag(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := ctx.New(rec, req)

	// Context bag
	c.AddContext("tenant", "acme")
	c.PushContext("steps", "auth")
	c.PushContext("steps", "billing")
	if v, ok := c.ContextValue("tenant"); !ok || v != "acme" {
		t.Fatalf("expected tenant=acme, got %v", v)
	}
	if attrs := c.ContextAttrs(); len(attrs) != 4 {
		t.Fatalf("expected 4 attr elements, got %v", attrs)
	}

	// Request-scoped Once
	var calls int
	v1, err := ctx.Once(c, "user", func() (string, error) { calls++; return "ada", nil })
	v2, _ := ctx.Once(c, "user", func() (string, error) { calls++; return "other", nil })
	if err != nil || v1 != "ada" || v2 != "ada" || calls != 1 {
		t.Fatalf("Once failed: v1=%q v2=%q calls=%d err=%v", v1, v2, calls, err)
	}

	// Post-response Defer + DrainDeferred
	var ran atomic.Bool
	c.Defer(func(context.Context) { ran.Store(true) })
	c.RunDeferred()
	drainCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := ctx.DrainDeferred(drainCtx); err != nil || !ran.Load() {
		t.Fatalf("expected deferred task to run & drain (ran=%v err=%v)", ran.Load(), err)
	}
}
