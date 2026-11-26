package socialite_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rapide-go/rapide/auth/socialite"
)

func TestSocialiteExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok_123"})
		case "/user":
			if r.Header.Get("Authorization") != "Bearer tok_123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         99,
				"name":       "Ada Lovelace",
				"email":      "ada@rapide.dev",
				"avatar_url": "https://avatars.test/ada.png",
			})
		}
	}))
	defer srv.Close()

	gh := socialite.GitHub("cid", "csec", "https://app.test/cb")
	gh.TokenURL = srv.URL + "/token"
	gh.UserInfoURL = srv.URL + "/user"

	state := socialite.GenerateState()
	authURL := gh.AuthCodeURL(state)
	if !strings.Contains(authURL, "state="+state) {
		t.Fatalf("missing state in auth url: %s", authURL)
	}

	u, err := gh.Exchange(context.Background(), "auth_code_xyz")
	if err != nil {
		t.Fatalf("exchange failed: %v", err)
	}
	if u.Name != "Ada Lovelace" || u.Email != "ada@rapide.dev" || u.AccessToken != "tok_123" {
		t.Fatalf("unexpected user: %+v", u)
	}
}
