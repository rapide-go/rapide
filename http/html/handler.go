package html

import (
	"bytes"
	"html/template"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/routing"
)

// Render renders a Node into a buffer and writes it as a UTF-8 HTML response through c.
func Render(c *ctx.Ctx, node Node) error {
	var buf bytes.Buffer
	if node != nil {
		if err := node.Render(&buf); err != nil {
			return err
		}
	}
	return c.HTML(buf.Bytes())
}

// RenderString renders a Node to an HTML string (useful for emails or embedding in templates).
func RenderString(node Node) (string, error) {
	var buf bytes.Buffer
	if node != nil {
		if err := node.Render(&buf); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

// RenderHTML renders a Node into a template.HTML value for direct use in html/template FuncMaps.
func RenderHTML(node Node) (template.HTML, error) {
	s, err := RenderString(node)
	return template.HTML(s), err
}

// Page adapts a JSX-style handler `func(*ctx.Ctx) (html.Node, error)` into an Rapide router.Handler.
func Page(fn func(*ctx.Ctx) (Node, error)) router.Handler {
	return func(c *ctx.Ctx) error {
		node, err := fn(c)
		if err != nil {
			return err
		}
		if c.Written() {
			return nil
		}
		return Render(c, node)
	}
}

// FormPage adapts a typed form handler `func(*ctx.Ctx, In) (html.Node, error)` into a router.Handler,
// decoding & validating In automatically and returning an HTML fragment (ideal for HTMX forms).
func FormPage[In any](fn func(*ctx.Ctx, In) (Node, error)) router.Handler {
	return func(c *ctx.Ctx) error {
		in, err := ctx.Bind[In](c)
		if err != nil {
			return err
		}
		node, err := fn(c, in)
		if err != nil {
			return err
		}
		if c.Written() {
			return nil
		}
		return Render(c, node)
	}
}
