package testkit

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/rapide-go/rapide/auth"
)

// Suite wraps any http.Handler (such as *router.Router) with fluent fluent HTTP test helpers.
type Suite struct {
	t       testing.TB
	handler http.Handler
	headers map[string]string
	cookies []*http.Cookie
	actor   any
}

// New creates a fluent HTTP test Suite around h.
func New(t testing.TB, h http.Handler) *Suite {
	t.Helper()
	return &Suite{
		t:       t,
		handler: h,
		headers: make(map[string]string),
	}
}

func (s *Suite) clone() *Suite {
	return &Suite{
		t:       s.t,
		handler: s.handler,
		headers: maps.Clone(s.headers),
		cookies: slices.Clone(s.cookies),
		actor:   s.actor,
	}
}

// WithHeader returns a derived Suite sending key: value on every request.
func (s *Suite) WithHeader(key, value string) *Suite {
	cp := s.clone()
	cp.headers[key] = value
	return cp
}

// WithBearerToken attaches an Authorization: Bearer <token> header.
func (s *Suite) WithBearerToken(token string) *Suite {
	return s.WithHeader("Authorization", "Bearer "+token)
}

// WithCookie attaches a cookie to outgoing requests.
func (s *Suite) WithCookie(c *http.Cookie) *Suite {
	cp := s.clone()
	cp.cookies = append(cp.cookies, c)
	return cp
}

// ActingAs authenticates every request from the returned Suite as user
// (retrievable inside handlers via auth.User[U](c) / auth.MustUser[U](c)).
func (s *Suite) ActingAs(user any) *Suite {
	cp := s.clone()
	cp.actor = user
	return cp
}

func (s *Suite) Get(path string) *Response    { return s.Do(http.MethodGet, path, nil, "") }
func (s *Suite) Delete(path string) *Response { return s.Do(http.MethodDelete, path, nil, "") }
func (s *Suite) PostJSON(path string, body any) *Response {
	return s.doJSON(http.MethodPost, path, body)
}
func (s *Suite) PutJSON(path string, body any) *Response { return s.doJSON(http.MethodPut, path, body) }
func (s *Suite) PatchJSON(path string, body any) *Response {
	return s.doJSON(http.MethodPatch, path, body)
}

func (s *Suite) doJSON(method, path string, body any) *Response {
	s.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			s.t.Fatalf("testkit: encode JSON body: %v", err)
		}
	}
	return s.Do(method, path, &buf, "application/json")
}

// Do executes an HTTP request against the Suite's handler and returns a fluent *Response.
func (s *Suite) Do(method, path string, body io.Reader, contentType string) *Response {
	s.t.Helper()
	req := httptest.NewRequest(method, path, body)
	if s.actor != nil {
		req = req.WithContext(auth.WithUser(req.Context(), s.actor))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	for _, ck := range s.cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return &Response{t: s.t, Recorder: rec}
}
