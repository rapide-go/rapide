package ctx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rapide-go/rapide/validation"
)

// MaxBodyBytes caps JSON request body decoding to prevent memory exhaustion DoS (default 2 MiB).
var MaxBodyBytes int64 = 2 << 20

// Authorizable may be implemented by request DTOs to enforce authorization during ctx.Bind
// (mirrors Rapide FormRequest::authorize()).
type Authorizable interface {
	Authorize(c *Ctx) error
}

// Preparable may be implemented by request DTOs (pointer receiver) to normalize or trim fields
// before validation runs (mirrors Rapide FormRequest::prepareForValidation()).
type Preparable interface {
	PrepareForValidation()
}

// Bind decodes a single JSON object (bounded by MaxBodyBytes) into T, runs optional
// PrepareForValidation / Authorize hooks, and validates it automatically.
func Bind[T any](c *Ctx) (T, error) {
	return bindJSON[T](c, false)
}

// BindStrict decodes JSON into T while rejecting unknown fields and trailing data,
// runs optional PrepareForValidation / Authorize hooks, and validates T.
func BindStrict[T any](c *Ctx) (T, error) {
	return bindJSON[T](c, true)
}

func bindJSON[T any](c *Ctx, strict bool) (T, error) {
	var in T
	if c.Request.Body != nil {
		body := http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
		dec := json.NewDecoder(body)
		if strict {
			dec.DisallowUnknownFields()
		}
		if err := dec.Decode(&in); err != nil && !errors.Is(err, io.EOF) {
			if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
				return in, Abort(http.StatusRequestEntityTooLarge, fmt.Sprintf("request body exceeds limit of %d bytes", maxErr.Limit))
			}
			return in, Abort(http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		}
		// Reject trailing JSON tokens (JSON smuggling protection).
		var trailing any
		if err := dec.Decode(&trailing); err != nil && !errors.Is(err, io.EOF) {
			return in, Abort(http.StatusBadRequest, "unexpected trailing data after JSON payload")
		} else if err == nil {
			return in, Abort(http.StatusBadRequest, "request body must contain a single JSON value")
		}
	}
	if p, ok := any(&in).(Preparable); ok {
		p.PrepareForValidation()
	}
	if a, ok := any(&in).(Authorizable); ok {
		if err := a.Authorize(c); err != nil {
			return in, err
		}
	} else if a, ok := any(in).(Authorizable); ok {
		if err := a.Authorize(c); err != nil {
			return in, err
		}
	}
	if err := validation.Validate(&in); err != nil {
		return in, err
	}
	return in, nil
}
