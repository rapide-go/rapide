package str_test

import (
	"testing"

	"github.com/rapide-go/rapide/support/str"
)

func TestStrHelpers(t *testing.T) {
	if got := str.Slug("Hello Rapide 2026!"); got != "hello-rapide-2026" {
		t.Fatalf("Slug: got %q", got)
	}
	if got := str.Snake("UserProfileID"); got != "user_profile_id" {
		t.Fatalf("Snake: got %q", got)
	}
	if got := str.Kebab("UserProfile"); got != "user-profile" {
		t.Fatalf("Kebab: got %q", got)
	}
	if got := str.Studly("user_profile-name"); got != "UserProfileName" {
		t.Fatalf("Studly: got %q", got)
	}
	if got := str.Camel("user_profile_name"); got != "userProfileName" {
		t.Fatalf("Camel: got %q", got)
	}
	if got := str.Limit("Hello world from Rapide", 11); got != "Hello world..." {
		t.Fatalf("Limit: got %q", got)
	}
	if got := str.Mask("ada@rapide.dev", '*', 2, 4); got != "ad****pide.dev" {
		t.Fatalf("Mask: got %q", got)
	}
	if got := str.Of("  Hello World!  ").Trim().Slug().Append("-v1").String(); got != "hello-world-v1" {
		t.Fatalf("Stringable: got %q", got)
	}
	if u := str.UUID(); len(u) != 36 {
		t.Fatalf("UUID: unexpected length %d (%q)", len(u), u)
	}
	if r := str.Random(24); len(r) != 24 {
		t.Fatalf("Random: unexpected length %d", len(r))
	}
}
