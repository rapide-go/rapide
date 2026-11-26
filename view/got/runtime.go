package got

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/view"
)

// Component is the core contract implemented by every AOT-compiled .got template.
type Component interface {
	Render(w io.Writer) error
}

// ComponentFunc adapts a render function into a Component.
type ComponentFunc func(w io.Writer) error

func (f ComponentFunc) Render(w io.Writer) error {
	if f == nil {
		return nil
	}
	return f(w)
}

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

// Render executes comp into a pooled buffer and writes the HTML response to c.
func Render(c *ctx.Ctx, comp Component) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if err := comp.Render(buf); err != nil {
		return err
	}
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return c.HTML(out)
}

// RenderToString executes comp and returns the rendered HTML string.
func RenderToString(comp Component) (string, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if err := comp.Render(buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Vite emits Vite dev/production <script> and <link> tags via view.Vite().
func Vite(entries ...string) string {
	return string(view.ViteTags(entries...))
}

// Truncate shortens s to at most max runes, appending "…" if truncated.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// Fmt formats interpolated component prop strings (used by generated *_got.go code).
func Fmt(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
