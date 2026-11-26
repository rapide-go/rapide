package testkit_test

import (
	"context"
	"testing"
	"time"

	"github.com/rapide-go/rapide/auth"
	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/mail"
	router "github.com/rapide-go/rapide/routing"
	testkit "github.com/rapide-go/rapide/testing"
)

type testActor struct {
	Name string
}

func TestSuiteActingAsAndFakeMail(t *testing.T) {
	fm := testkit.FakeMail(t)
	fm.AssertNothingSent(t)

	r := router.New()
	r.Get("/me", func(c *ctx.Ctx) error {
		u, err := auth.MustUser[testActor](c)
		if err != nil {
			return err
		}
		_ = mail.Send(c.Request.Context(), mail.Message{Subject: "Hi " + u.Name})
		return c.JSON(map[string]any{"name": u.Name})
	})

	// Unauthenticated -> 401
	testkit.New(t, r).Get("/me").AssertUnauthorized()

	// ActingAs -> 200 + mail.Send captured by FakeMail
	resp := testkit.New(t, r).
		ActingAs(testActor{Name: "Ada"}).
		Get("/me").
		AssertOk().
		AssertJSON(map[string]any{"name": "Ada"})

	got := resp.Decode[map[string]string]()
	if got["name"] != "Ada" {
		t.Fatalf("unexpected decoded: %+v", got)
	}
	fm.AssertSentCount(t, 1)
	if fm.Sent[0].Subject != "Hi Ada" {
		t.Fatalf("expected subject 'Hi Ada', got %q", fm.Sent[0].Subject)
	}

	// Clock travel
	clk := testkit.NewClock()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk.Freeze(base)
	clk.Travel(2 * time.Hour)
	if !clk.Now().Equal(base.Add(2 * time.Hour)) {
		t.Fatalf("expected +2h, got %v", clk.Now())
	}
	_ = context.Background()
}
