package factory_test

import (
	"strings"
	"testing"

	"github.com/rapide-go/rapide/database/factory"
)

type testUser struct {
	Name  string
	Email string
	Admin bool
}

func TestFactoryMakeAndStates(t *testing.T) {
	userFactory := factory.Define(func() testUser {
		return testUser{
			Name:  factory.Name(),
			Email: factory.Email(),
		}
	})

	admins := userFactory.Count(3).State(func(u *testUser) {
		u.Admin = true
	}).MakeMany()

	if len(admins) != 3 {
		t.Fatalf("expected 3 users, got %d", len(admins))
	}
	seen := make(map[string]bool)
	for _, u := range admins {
		if !u.Admin || !strings.Contains(u.Email, "@") || seen[u.Email] {
			t.Fatalf("invalid generated user: %+v", u)
		}
		seen[u.Email] = true
	}

	if uuid := factory.UUID(); len(uuid) != 36 {
		t.Fatalf("expected 36-char UUID, got %q", uuid)
	}
}
