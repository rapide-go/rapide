package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/session"
)

type testUser struct {
	ID   int
	Name string
}

func TestSessionRedirectAndResolve(t *testing.T) {
	loader := func(id any) (testUser, error) {
		return testUser{ID: id.(int), Name: "Ada"}, nil
	}

	// Unauthenticated browser request -> 302 redirect to /login
	mw := session.Middleware("rapide_session", time.Hour)(SessionRedirect("/login", loader)(func(c *ctx.Ctx) error {
		return c.Text("ok")
	}))
	w := httptest.NewRecorder()
	c := ctx.New(w, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if err := mw(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Fatalf("expected 302 -> /login, got %d (%q)", w.Code, w.Header().Get("Location"))
	}

	// Unauthenticated JSON request -> 401
	reqJSON := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	reqJSON.Header.Set("Accept", "application/json")
	cJSON := ctx.New(httptest.NewRecorder(), reqJSON)
	err := mw(cJSON)
	if err == nil {
		t.Fatal("expected 401 error for JSON request")
	}

	// Check / Guest helpers
	if Check[testUser](cJSON) || !Guest[testUser](cJSON) {
		t.Fatal("expected Guest=true and Check=false before SetUser")
	}
	SetUser(cJSON, testUser{ID: 1, Name: "Ada"})
	if !Check[testUser](cJSON) || Guest[testUser](cJSON) {
		t.Fatal("expected Check=true and Guest=false after SetUser")
	}
}
