package router_test

import (
	"testing"
	"time"

	crypt "github.com/rapide-go/rapide/encryption"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

func TestNamedRouteURLAndSignedURL(t *testing.T) {
	r := router.New()
	r.Get("/posts/{slug}/comments/{id}", func(c *ctx.Ctx) error { return nil }).SetName("posts.comment")

	got, err := r.URL("posts.comment", map[string]any{
		"slug": "hello-world",
		"id":   42,
		"page": 2,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "/posts/hello-world/comments/42?page=2" {
		t.Fatalf("unexpected url: %q", got)
	}

	if _, err := r.URL("posts.comment", map[string]any{"slug": "only-slug"}); err == nil {
		t.Fatal("expected error for missing {id} param")
	}

	signed, err := r.SignedURL("https://rapide.test", "posts.comment", "super-secret", time.Minute, map[string]any{
		"slug": "hello-world",
		"id":   42,
	})
	if err != nil {
		t.Fatalf("signed url error: %v", err)
	}
	if err := crypt.VerifyURL(signed, "super-secret"); err != nil {
		t.Fatalf("verify signed url failed: %v", err)
	}
}
