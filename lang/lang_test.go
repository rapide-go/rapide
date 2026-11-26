package lang_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/lang"
	router "github.com/rapide-go/rapide/routing"
)

func TestTranslatorAndMiddleware(t *testing.T) {
	tr := lang.New("en")
	tr.LoadMap("en", map[string]string{
		"welcome": "Welcome, :name!",
		"apples":  "no apples|one apple|:count apples",
	})
	tr.LoadMap("fr", map[string]string{
		"welcome": "Bienvenue, :name!",
	})

	if got := tr.Get("fr", "welcome", map[string]any{"name": "Ada"}); got != "Bienvenue, Ada!" {
		t.Fatalf("got %q", got)
	}
	if got := tr.Get("fr", "apples"); got != "no apples|one apple|:count apples" {
		t.Fatalf("fallback failed: %q", got)
	}
	if got := tr.Choice("en", "apples", 0); got != "no apples" {
		t.Fatalf("choice 0: %q", got)
	}
	if got := tr.Choice("en", "apples", 1); got != "one apple" {
		t.Fatalf("choice 1: %q", got)
	}
	if got := tr.Choice("en", "apples", 5); got != "5 apples" {
		t.Fatalf("choice 5: %q", got)
	}

	r := router.New()
	r.Use(lang.Middleware("en"))
	r.Get("/greet", func(c *ctx.Ctx) error {
		return c.Text(tr.Get(lang.Locale(c), "welcome", map[string]any{"name": "Ada"}))
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/greet", nil)
	req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9,en;q=0.8")
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "Bienvenue, Ada!" {
		t.Fatalf("expected French greeting, got %q", rec.Body.String())
	}
}
