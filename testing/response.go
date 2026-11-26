package testkit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Response wraps *httptest.ResponseRecorder with fluent fluent assertions.
type Response struct {
	t        testing.TB
	Recorder *httptest.ResponseRecorder
}

func (r *Response) AssertStatus(want int) *Response {
	r.t.Helper()
	if got := r.Recorder.Code; got != want {
		r.t.Fatalf("expected status %d, got %d (body: %s)", want, got, r.Recorder.Body.String())
	}
	return r
}

func (r *Response) AssertOk() *Response           { return r.AssertStatus(http.StatusOK) }
func (r *Response) AssertCreated() *Response      { return r.AssertStatus(http.StatusCreated) }
func (r *Response) AssertNoContent() *Response    { return r.AssertStatus(http.StatusNoContent) }
func (r *Response) AssertUnauthorized() *Response { return r.AssertStatus(http.StatusUnauthorized) }
func (r *Response) AssertForbidden() *Response    { return r.AssertStatus(http.StatusForbidden) }
func (r *Response) AssertNotFound() *Response     { return r.AssertStatus(http.StatusNotFound) }
func (r *Response) AssertUnprocessable() *Response {
	return r.AssertStatus(http.StatusUnprocessableEntity)
}

func (r *Response) AssertRedirect(location string) *Response {
	r.t.Helper()
	code := r.Recorder.Code
	if code < 300 || code >= 400 {
		r.t.Fatalf("expected 3xx redirect, got %d", code)
	}
	if location != "" && r.Recorder.Header().Get("Location") != location {
		r.t.Fatalf("expected redirect to %q, got %q", location, r.Recorder.Header().Get("Location"))
	}
	return r
}

func (r *Response) AssertHeader(key, want string) *Response {
	r.t.Helper()
	if got := r.Recorder.Header().Get(key); got != want {
		r.t.Fatalf("expected header %s=%q, got %q", key, want, got)
	}
	return r
}

func (r *Response) AssertSee(substr string) *Response {
	r.t.Helper()
	if !strings.Contains(r.Recorder.Body.String(), substr) {
		r.t.Fatalf("expected body to contain %q, got: %s", substr, r.Recorder.Body.String())
	}
	return r
}

func (r *Response) AssertDontSee(substr string) *Response {
	r.t.Helper()
	if strings.Contains(r.Recorder.Body.String(), substr) {
		r.t.Fatalf("expected body NOT to contain %q, got: %s", substr, r.Recorder.Body.String())
	}
	return r
}

// AssertJSON verifies that every key/value in want matches the decoded top-level JSON object.
func (r *Response) AssertJSON(want map[string]any) *Response {
	r.t.Helper()
	var got map[string]any
	if err := json.Unmarshal(r.Recorder.Body.Bytes(), &got); err != nil {
		r.t.Fatalf("expected valid JSON object: %v (body: %s)", err, r.Recorder.Body.String())
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			r.t.Fatalf("expected JSON key %q to exist in %v", k, got)
		}
		wBytes, _ := json.Marshal(wv)
		gBytes, _ := json.Marshal(gv)
		if !bytes.Equal(wBytes, gBytes) {
			r.t.Fatalf("JSON key %q: expected %s, got %s", k, wBytes, gBytes)
		}
	}
	return r
}

// Decode unmarshals the response body into a typed value T (Go 1.27 generic method).
func (r *Response) Decode[T any]() T {
	r.t.Helper()
	var out T
	if err := json.Unmarshal(r.Recorder.Body.Bytes(), &out); err != nil {
		r.t.Fatalf("testkit.Decode: %v (body: %s)", err, r.Recorder.Body.String())
	}
	return out
}

// DecodeJSON delegates to r.Decode[T]() (kept for backwards compatibility).
func DecodeJSON[T any](r *Response) T {
	return r.Decode[T]()
}
