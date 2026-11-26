package session_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/session"
)

func TestSessionRegeneratePreventsFixation(t *testing.T) {
	store := session.NewMemoryStore()
	r := router.New()
	r.Use(session.MiddlewareWithStore("sid", time.Hour, store))

	r.Get("/init", func(c *ctx.Ctx) error {
		session.Current(c).Set("pre", "1")
		return c.Text(session.Current(c).ID)
	})
	r.Post("/login", func(c *ctx.Ctx) error {
		s := session.Current(c)
		s.Regenerate()
		s.Set("user", "ada")
		return c.Text(s.ID)
	})
	r.Get("/check", func(c *ctx.Ctx) error {
		u, _ := session.Current(c).Get("user")
		if u == nil {
			return c.Status(http.StatusUnauthorized).Text("no")
		}
		return c.Text(u.(string))
	})

	// 1. Establish initial session
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/init", nil))
	oldCookie := rec1.Result().Cookies()[0]

	// 2. Login rotates session ID & deletes old ID
	req2 := httptest.NewRequest(http.MethodPost, "/login", nil)
	req2.AddCookie(oldCookie)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	newCookie := rec2.Result().Cookies()[0]
	if newCookie.Value == oldCookie.Value {
		t.Fatal("expected session ID to rotate on Regenerate()")
	}

	// 3. Replaying old cookie must NOT authenticate as "ada"
	req3 := httptest.NewRequest(http.MethodGet, "/check", nil)
	req3.AddCookie(oldCookie)
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Fatalf("expected old session cookie to be invalidated, got %d", rec3.Code)
	}

	// 4. New cookie authenticates as "ada"
	req4 := httptest.NewRequest(http.MethodGet, "/check", nil)
	req4.AddCookie(newCookie)
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK || rec4.Body.String() != "ada" {
		t.Fatalf("expected 200 ada, got %d %q", rec4.Code, rec4.Body.String())
	}
}
