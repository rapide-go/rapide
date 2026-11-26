package ctx

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rapide-go/rapide/container"
)

// Ctx wraps an HTTP request/response lifecycle with ergonomic fluent helpers.
type Ctx struct {
	Writer    http.ResponseWriter
	Request   *http.Request
	Container *container.Container

	recorder *responseRecorder
	status   int
	values   map[string]any
}

// New creates a new request Ctx scoped to the global container.
func New(w http.ResponseWriter, r *http.Request) *Ctx {
	rec := &responseRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
	return &Ctx{
		Writer:    rec,
		Request:   r,
		Container: container.Default.Scope(),
		recorder:  rec,
		status:    http.StatusOK,
		values:    make(map[string]any),
	}
}

// Context returns the underlying request context.Context.
func (c *Ctx) Context() context.Context {
	return c.Request.Context()
}

// Param reads a Go 1.22+ ServeMux path parameter (e.g. /posts/{id}).
func (c *Ctx) Param(name string) string {
	return c.Request.PathValue(name)
}

// ParamInt reads a path parameter as an integer.
func (c *Ctx) ParamInt(name string) (int, error) {
	raw := c.Param(name)
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, Abort(http.StatusBadRequest, fmt.Sprintf("invalid path parameter %q", name))
	}
	return n, nil
}

// Query reads a URL query parameter with an optional fallback.
func (c *Ctx) Query(name string, fallback ...string) string {
	v := c.Request.URL.Query().Get(name)
	if v == "" && len(fallback) > 0 {
		return fallback[0]
	}
	return v
}

// QueryInt reads a URL query parameter as an integer with an optional fallback (defaults to 0).
func (c *Ctx) QueryInt(name string, fallback ...int) int {
	raw := c.Request.URL.Query().Get(name)
	if raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			return n
		}
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return 0
}

// QueryBool reads a URL query parameter as a boolean ("1", "true", "yes", "on") with an optional fallback.
func (c *Ctx) QueryBool(name string, fallback ...bool) bool {
	raw := strings.TrimSpace(strings.ToLower(c.Request.URL.Query().Get(name)))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return false
}

// WantsJSON reports whether the client prefers a JSON response (Rapide $request->expectsJson()).
func (c *Ctx) WantsJSON() bool {
	if c.Header("X-Inertia") != "" {
		return false
	}
	accept := strings.ToLower(c.Header("Accept"))
	if strings.Contains(accept, "application/json") || strings.Contains(accept, "+json") {
		return true
	}
	return strings.EqualFold(c.Header("X-Requested-With"), "XMLHttpRequest")
}

// IsMethod reports whether the HTTP request method matches method (case-insensitive).
func (c *Ctx) IsMethod(method string) bool {
	return strings.EqualFold(c.Request.Method, method)
}

// Header reads a request header.
func (c *Ctx) Header(name string) string {
	return c.Request.Header.Get(name)
}

// SetHeader sets a response header.
func (c *Ctx) SetHeader(name, value string) *Ctx {
	c.Writer.Header().Set(name, value)
	return c
}

// Set stores a per-request value (e.g. authenticated user from middleware).
func (c *Ctx) Set(key string, val any) {
	c.values[key] = val
}

// Get retrieves a per-request value.
func (c *Ctx) Get(key string) (any, bool) {
	v, ok := c.values[key]
	return v, ok
}
