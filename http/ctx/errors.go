package ctx

import (
	"errors"
	"net/http"
	"sync"

	applog "github.com/rapide-go/rapide/log"
	"github.com/rapide-go/rapide/support/errs"
	"github.com/rapide-go/rapide/validation"
)

var (
	// DebugMode controls whether unhandled 500 errors expose internal error messages in JSON responses.
	DebugMode bool = false
	// CustomErrorHandler allows overriding how framework errors are rendered.
	CustomErrorHandler func(c *Ctx, err error)

	errHooksMu sync.RWMutex
	reporters  []func(c *Ctx, err error)
	renderers  []func(c *Ctx, err error) bool
)

// Report registers an error reporter hook (e.g. Sentry, Slack, metrics) invoked on unhandled errors
// .
func Report(fn func(c *Ctx, err error)) {
	errHooksMu.Lock()
	reporters = append(reporters, fn)
	errHooksMu.Unlock()
}

// RenderError registers a type-safe custom error renderer for any error type E
// .
//
// Example:
//
//	ctx.RenderError(func(c *ctx.Ctx, err *MyDomainError) error {
//	    return c.Status(409).JSON(map[string]string{"code": err.Code})
//	})
func RenderError[E error](fn func(c *Ctx, err E) error) {
	errHooksMu.Lock()
	renderers = append(renderers, func(c *Ctx, err error) bool {
		if target, ok := errors.AsType[E](err); ok {
			_ = fn(c, target)
			return true
		}
		return false
	})
	errHooksMu.Unlock()
}

// HTTPError represents an explicit HTTP status error returned from a handler.
type HTTPError struct {
	Code    int
	Message string
	Details any
}

func (e *HTTPError) Error() string {
	return e.Message
}

// Abort creates an HTTPError with the given status code and message.
func Abort(code int, message string) *HTTPError {
	return &HTTPError{Code: code, Message: message}
}

// NotFound returns a 404 HTTPError.
func NotFound(message string) *HTTPError {
	return Abort(http.StatusNotFound, message)
}

// Unauthorized returns a 401 HTTPError.
func Unauthorized(message string) *HTTPError {
	return Abort(http.StatusUnauthorized, message)
}

// Forbidden returns a 403 HTTPError.
func Forbidden(message string) *HTTPError {
	return Abort(http.StatusForbidden, message)
}

// WriteError translates framework errors (custom RenderError[E], validation.Errors, *HTTPError,
// or generic errors) into consistent responses while invoking registered Report hooks.
func (c *Ctx) WriteError(err error) {
	if c.Written() || err == nil {
		return
	}

	if CustomErrorHandler != nil {
		CustomErrorHandler(c, err)
		return
	}

	errHooksMu.RLock()
	customRenderers := append(([]func(*Ctx, error) bool)(nil), renderers...)
	customReporters := append(([]func(*Ctx, error))(nil), reporters...)
	errHooksMu.RUnlock()

	for _, r := range customRenderers {
		if r(c, err) {
			return
		}
	}

	if valErrs, ok := errors.AsType[validation.Errors](err); ok {
		_ = c.Status(http.StatusUnprocessableEntity).JSON(map[string]any{
			"message": "The given data was invalid.",
			"errors":  valErrs,
		})
		return
	}

	if appErr, ok := errors.AsType[*errs.Error](err); ok {
		status := appErr.Status
		if status == 0 {
			status = http.StatusInternalServerError
		}
		if status >= 500 {
			for _, rep := range customReporters {
				rep(c, err)
			}
			applog.Error("application error", "op", appErr.Op, "code", appErr.Code, "error", err)
		}
		msg := appErr.Message
		if status >= 500 && !DebugMode {
			msg = "Internal Server Error"
		}
		body := map[string]any{"message": msg}
		if appErr.Code != "" {
			body["code"] = appErr.Code
		}
		if len(appErr.Meta) > 0 && (status < 500 || DebugMode) {
			body["details"] = appErr.Meta
		}
		_ = c.Status(status).JSON(body)
		return
	}

	if httpErr, ok := errors.AsType[*HTTPError](err); ok {
		body := map[string]any{"message": httpErr.Message}
		if httpErr.Details != nil {
			body["details"] = httpErr.Details
		}
		_ = c.Status(httpErr.Code).JSON(body)
		return
	}

	for _, rep := range customReporters {
		rep(c, err)
	}
	applog.Error("unhandled http error", "method", c.Request.Method, "path", c.Request.URL.Path, "error", err)

	msg := "Internal Server Error"
	if DebugMode {
		msg = err.Error()
	}
	_ = c.Status(http.StatusInternalServerError).JSON(map[string]any{
		"message": msg,
	})
}
