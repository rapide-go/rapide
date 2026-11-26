package validation_test

import (
	"testing"

	"github.com/rapide-go/rapide/validation"
)

func TestFluentValidationRules(t *testing.T) {
	pw := validation.Password(8).MixedCase().Numbers().Symbols()
	if msg := pw.Validate("password", "weak"); msg == "" {
		t.Fatal("expected short password to fail")
	}
	if msg := pw.Validate("password", "alllowercase1!"); msg == "" {
		t.Fatal("expected missing uppercase to fail")
	}
	if msg := pw.Validate("password", "StrongPass1!"); msg != "" {
		t.Fatalf("expected strong password to pass, got %q", msg)
	}

	statusRule := validation.In("draft", "published", "archived")
	if msg := statusRule.Validate("status", "published"); msg != "" {
		t.Fatalf("expected valid status, got %q", msg)
	}
	if msg := statusRule.Validate("status", "deleted"); msg == "" {
		t.Fatal("expected invalid status to fail")
	}
}
