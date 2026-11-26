package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a structured, wrappable application error carrying an HTTP status,
// machine-readable code, safe public message, operation breadcrumb, metadata, and root cause.
type Error struct {
	Status  int            `json:"-"`
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message"`
	Op      string         `json:"-"`
	Meta    map[string]any `json:"details,omitempty"`
	Err     error          `json:"-"`
}

func (e *Error) Error() string {
	var base string
	if e.Op != "" {
		base = e.Op + ": " + e.Message
	} else {
		base = e.Message
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", base, e.Err)
	}
	return base
}

// Unwrap exposes the underlying error for standard errors.Is and errors.As chains.
func (e *Error) Unwrap() error {
	return e.Err
}

// WithOp attaches an operation name (e.g. "BlogController.Store") to the error.
func (e *Error) WithOp(op string) *Error {
	e.Op = op
	return e
}

// WithCode sets a machine-readable error code (e.g. "EMAIL_TAKEN").
func (e *Error) WithCode(code string) *Error {
	e.Code = code
	return e
}

// WithMeta attaches structured key/value metadata for API responses and structured logs.
func (e *Error) WithMeta(key string, val any) *Error {
	if e.Meta == nil {
		e.Meta = make(map[string]any)
	}
	e.Meta[key] = val
	return e
}

// New constructs a new structured Error.
func New(status int, code, message string, cause ...error) *Error {
	var err error
	if len(cause) > 0 {
		err = cause[0]
	}
	return &Error{
		Status:  status,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Wrap wraps err with a contextual message while preserving any underlying *Error's
// HTTP Status and Code if present. Returns nil if err is nil.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	if inner, ok := errors.AsType[*Error](err); ok {
		status = inner.Status
		code = inner.Code
	}
	return &Error{
		Status:  status,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// WrapOp wraps err with an operation breadcrumb (e.g. `errs.WrapOp("users.store", err)`).
// Returns nil if err is nil.
func WrapOp(op string, err error) error {
	if err == nil {
		return nil
	}
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	msg := "Internal Server Error"
	if inner, ok := errors.AsType[*Error](err); ok {
		status = inner.Status
		code = inner.Code
		msg = inner.Message
	}
	return &Error{
		Status:  status,
		Code:    code,
		Message: msg,
		Op:      op,
		Err:     err,
	}
}

// Convenience constructors for common HTTP/domain error classes.
func BadRequest(msg string, cause ...error) *Error {
	return New(http.StatusBadRequest, "BAD_REQUEST", msg, cause...)
}

func Unauthorized(msg string, cause ...error) *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", msg, cause...)
}

func Forbidden(msg string, cause ...error) *Error {
	return New(http.StatusForbidden, "FORBIDDEN", msg, cause...)
}

func NotFound(msg string, cause ...error) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", msg, cause...)
}

func Conflict(msg string, cause ...error) *Error {
	return New(http.StatusConflict, "CONFLICT", msg, cause...)
}

func Internal(msg string, cause ...error) *Error {
	return New(http.StatusInternalServerError, "INTERNAL_ERROR", msg, cause...)
}

// As is a generic one-line wrapper around standard errors.As.
func As[T error](err error) (T, bool) {
	var target T
	ok := errors.As(err, &target)
	return target, ok
}

// Is delegates to standard errors.Is.
func Is(err, target error) bool {
	return errors.Is(err, target)
}
