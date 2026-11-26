package ctx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sampleFormRequest struct {
	Email string `json:"email" validate:"required,email"`
	Allow bool   `json:"allow"`
}

func (r *sampleFormRequest) PrepareForValidation() {
	r.Email = strings.TrimSpace(strings.ToLower(r.Email))
}

func (r *sampleFormRequest) Authorize(_ *Ctx) error {
	if !r.Allow {
		return Forbidden("not allowed")
	}
	return nil
}

func TestBindFormRequestHooks(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"email":"  Alice@Example.COM ","allow":true}`))
	c := New(httptest.NewRecorder(), req)

	in, err := Bind[sampleFormRequest](c)
	if err != nil {
		t.Fatalf("expected Bind to succeed, got %v", err)
	}
	if in.Email != "alice@example.com" {
		t.Fatalf("expected PrepareForValidation to normalize email, got %q", in.Email)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"email":"alice@example.com","allow":false}`))
	c2 := New(httptest.NewRecorder(), req2)
	_, err = Bind[sampleFormRequest](c2)
	if err == nil {
		t.Fatal("expected Authorize failure")
	}
	httpErr, ok := err.(*HTTPError)
	if !ok || httpErr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %v", err)
	}
}

func TestQueryAndRequestHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/items?page=3&bad=abc&active=yes&off=0", nil)
	req.Header.Set("Accept", "application/json")
	c := New(httptest.NewRecorder(), req)

	if got := c.QueryInt("page", 1); got != 3 {
		t.Fatalf("QueryInt(page) = %d, want 3", got)
	}
	if got := c.QueryInt("bad", 7); got != 7 {
		t.Fatalf("QueryInt(bad) = %d, want fallback 7", got)
	}
	if !c.QueryBool("active") {
		t.Fatal("expected QueryBool(active) to be true")
	}
	if c.QueryBool("off", true) {
		t.Fatal("expected QueryBool(off) to be false")
	}
	if !c.WantsJSON() {
		t.Fatal("expected WantsJSON() to be true")
	}
	if !c.IsMethod("get") {
		t.Fatal("expected IsMethod(get) to be true")
	}

	req.Header.Set("X-Inertia", "true")
	if c.WantsJSON() {
		t.Fatal("expected WantsJSON() to be false for Inertia requests")
	}
}
