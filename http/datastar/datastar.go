package datastar

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/http/html"
	"github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/support/option"
	"github.com/rapide-go/rapide/validation"
)

// ReadSignals decodes Datastar reactive signals into a typed struct T
// (from the `datastar` query parameter on GET/DELETE, or JSON body on POST/PUT/PATCH)
// and validates T using Rapide's struct-tag validator.
func ReadSignals[T any](c *ctx.Ctx) (T, error) {
	var signals T
	switch c.Request.Method {
	case http.MethodGet, http.MethodDelete:
		raw := c.Query("datastar")
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &signals); err != nil {
				return signals, ctx.Abort(http.StatusBadRequest, "invalid datastar signals: "+err.Error())
			}
		}
		if err := validation.Validate(&signals); err != nil {
			return signals, err
		}
		return signals, nil
	default:
		return ctx.Bind[T](c)
	}
}

// PatchOptions configures how Datastar morphs/patches DOM elements.
type PatchOptions struct {
	Selector string // CSS selector target (optional; defaults to element ID morphing)
	Mode     string // outer, inner, replace, prepend, append, before, after, remove
}

func WithSelector(sel string) option.Option[PatchOptions] {
	return func(o *PatchOptions) { o.Selector = sel }
}

func WithMode(mode string) option.Option[PatchOptions] {
	return func(o *PatchOptions) { o.Mode = mode }
}

// SSE wraps Rapide's hardened *ctx.SSEWriter with Datastar v1 DOM-morphing and signal-patching helpers.
type SSE struct {
	*ctx.SSEWriter
}

// NewSSE initializes a Datastar SSE stream backed by c.SSE() (deadline-free, proxy-unbuffered).
func NewSSE(c *ctx.Ctx) *SSE {
	return &SSE{SSEWriter: c.SSE()}
}

// PatchElements renders an Rapide html.Node (pure Go or compiled .alx) and streams a
// `datastar-patch-elements` SSE event to morph the client DOM in place.
func (s *SSE) PatchElements(node html.Node, opts ...option.Option[PatchOptions]) error {
	markup, err := html.RenderString(node)
	if err != nil {
		return err
	}
	return s.PatchHTML(markup, opts...)
}

// PatchHTML streams a raw HTML string as a `datastar-patch-elements` SSE event.
func (s *SSE) PatchHTML(markup string, opts ...option.Option[PatchOptions]) error {
	var cfg PatchOptions
	option.Apply(&cfg, opts...)

	var lines []string
	if cfg.Mode != "" {
		lines = append(lines, "mode "+cfg.Mode)
	}
	if cfg.Selector != "" {
		lines = append(lines, "selector "+cfg.Selector)
	}
	for line := range strings.SplitSeq(markup, "\n") {
		lines = append(lines, "elements "+line)
	}
	return s.Send("datastar-patch-elements", strings.Join(lines, "\n"))
}

// RemoveElements removes all DOM elements matching selector on the client.
func (s *SSE) RemoveElements(selector string) error {
	return s.Send("datastar-patch-elements", fmt.Sprintf("mode remove\nselector %s", selector))
}

// PatchSignals updates reactive client-side Datastar signals from any JSON-serializable struct or map.
func (s *SSE) PatchSignals(signals any, onlyIfMissing ...bool) error {
	raw, err := json.Marshal(signals)
	if err != nil {
		return err
	}
	var lines []string
	if len(onlyIfMissing) > 0 && onlyIfMissing[0] {
		lines = append(lines, "onlyIfMissing true")
	}
	lines = append(lines, "signals "+string(raw))
	return s.Send("datastar-patch-signals", strings.Join(lines, "\n"))
}

// Action adapts a strongly-typed Datastar handler `func(*ctx.Ctx, *SSE, S) error` into an Rapide router.Handler.
func Action[S any](fn func(c *ctx.Ctx, sse *SSE, signals S) error) router.Handler {
	return func(c *ctx.Ctx) error {
		signals, err := ReadSignals[S](c)
		if err != nil {
			return err
		}
		return fn(c, NewSSE(c), signals)
	}
}
