package ctx

import (
	"encoding/json"
	"io"
	"net/http"
)

// responseRecorder wraps http.ResponseWriter to track status code, bytes written, and header state.
type responseRecorder struct {
	http.ResponseWriter
	status  int
	bytes   int
	written bool
}

func (rr *responseRecorder) WriteHeader(code int) {
	if rr.written {
		return
	}
	rr.status = code
	rr.written = true
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(p []byte) (int, error) {
	if !rr.written {
		rr.WriteHeader(http.StatusOK)
	}
	n, err := rr.ResponseWriter.Write(p)
	rr.bytes += n
	return n, err
}

// Unwrap supports http.ResponseController (flushing, deadlines, hijacking).
func (rr *responseRecorder) Unwrap() http.ResponseWriter {
	return rr.ResponseWriter
}

// Status sets the HTTP status code to be sent on the next response write.
func (c *Ctx) Status(code int) *Ctx {
	c.status = code
	return c
}

// StatusCode returns the HTTP status code written to the response.
func (c *Ctx) StatusCode() int {
	return c.recorder.status
}

// BytesWritten returns the total number of response body bytes written.
func (c *Ctx) BytesWritten() int {
	return c.recorder.bytes
}

// Written reports whether a response has already been written.
func (c *Ctx) Written() bool {
	return c.recorder.written
}

// JSON writes a JSON response with the configured status code.
func (c *Ctx) JSON(payload any) error {
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(c.status)
	return json.NewEncoder(c.Writer).Encode(payload)
}

// HTML writes an HTML response with the configured status code.
func (c *Ctx) HTML(body []byte) error {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(c.status)
	_, err := c.Writer.Write(body)
	return err
}

// Text writes a plain text response.
func (c *Ctx) Text(body string) error {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(c.status)
	_, err := io.WriteString(c.Writer, body)
	return err
}

// NoContent writes a 204 No Content response.
func (c *Ctx) NoContent() error {
	c.Writer.WriteHeader(http.StatusNoContent)
	return nil
}

// SetCookie writes a Set-Cookie header onto the response.
func (c *Ctx) SetCookie(cookie *http.Cookie) *Ctx {
	if cookie != nil {
		http.SetCookie(c.Writer, cookie)
	}
	return c
}

// Redirect writes an HTTP redirect response to target.
func (c *Ctx) Redirect(code int, target string) error {
	if code < 300 || code >= 400 {
		code = http.StatusFound
	}
	http.Redirect(c.Writer, c.Request, target, code)
	return nil
}

// SafeRedirect redirects only to relative same-origin paths (blocking open redirects
// such as "https://evil.com", "//evil.com", or "/\evil.com"), falling back to fallback.
func (c *Ctx) SafeRedirect(code int, target, fallback string) error {
	if fallback == "" {
		fallback = "/"
	}
	if !isSafeRelativeRedirect(target) {
		target = fallback
	}
	return c.Redirect(code, target)
}

func isSafeRelativeRedirect(target string) bool {
	if len(target) == 0 || target[0] != '/' {
		return false
	}
	if len(target) > 1 && (target[1] == '/' || target[1] == '\\') {
		return false
	}
	for i := 0; i < len(target); i++ {
		if target[i] == '\r' || target[i] == '\n' || target[i] == '\\' {
			return false
		}
	}
	return true
}
