package html

import (
	"fmt"
	"html"
	"io"
	"iter"
)

// Node is any renderable HTML unit (element, attribute, text, fragment, or component).
type Node interface {
	Render(w io.Writer) error
}

// NodeFunc adapts a plain function into a Node.
type NodeFunc func(w io.Writer) error

func (f NodeFunc) Render(w io.Writer) error { return f(w) }

// attrNode marks a Node as an HTML attribute so El renders it inside the opening tag.
type attrNode struct {
	name  string
	value string
	boolA bool
}

func (a attrNode) Render(w io.Writer) error {
	if a.boolA {
		_, err := fmt.Fprintf(w, " %s", a.name)
		return err
	}
	_, err := fmt.Fprintf(w, ` %s="%s"`, a.name, html.EscapeString(a.value))
	return err
}

// Attr creates a key="value" HTML attribute (value is XSS-escaped).
func Attr(name, value string) Node {
	return attrNode{name: name, value: value}
}

// BoolAttr creates a boolean HTML attribute (e.g. disabled, checked, required).
func BoolAttr(name string, on bool) Node {
	if !on {
		return Empty()
	}
	return attrNode{name: name, boolA: true}
}

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// El creates an HTML element streaming attributes and children directly to io.Writer.
func El(tag string, children ...Node) Node {
	return NodeFunc(func(w io.Writer) error {
		if _, err := fmt.Fprintf(w, "<%s", tag); err != nil {
			return err
		}
		var body []Node
		for _, ch := range children {
			if ch == nil {
				continue
			}
			if _, isAttr := ch.(attrNode); isAttr {
				if err := ch.Render(w); err != nil {
					return err
				}
			} else {
				body = append(body, ch)
			}
		}
		if _, err := io.WriteString(w, ">"); err != nil {
			return err
		}
		if voidElements[tag] {
			return nil
		}
		for _, ch := range body {
			if err := ch.Render(w); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, "</%s>", tag)
		return err
	})
}

// Text renders XSS-escaped text content.
func Text(s string) Node {
	return NodeFunc(func(w io.Writer) error {
		_, err := io.WriteString(w, html.EscapeString(s))
		return err
	})
}

// Any renders a Node directly if v implements Node; otherwise formats and XSS-escapes v.
// Used by the .alx JSX transpiler for `{expr}` interpolations.
func Any(v any) Node {
	if v == nil {
		return Empty()
	}
	if n, ok := v.(Node); ok {
		return n
	}
	return Text(fmt.Sprint(v))
}

// Raw renders trusted unescaped HTML. Use only with sanitized/trusted strings.
func Raw(trustedHTML string) Node {
	return NodeFunc(func(w io.Writer) error {
		_, err := io.WriteString(w, trustedHTML)
		return err
	})
}

// Empty renders nothing.
func Empty() Node {
	return NodeFunc(func(io.Writer) error { return nil })
}

// Fragment renders multiple sibling Nodes without a wrapper element.
func Fragment(nodes ...Node) Node {
	return NodeFunc(func(w io.Writer) error {
		for _, n := range nodes {
			if n == nil {
				continue
			}
			if err := n.Render(w); err != nil {
				return err
			}
		}
		return nil
	})
}

// If renders node only when cond is true.
func If(cond bool, node Node) Node {
	if !cond {
		return Empty()
	}
	return node
}

// IfElse renders whenTrue if cond is true, otherwise whenFalse.
func IfElse(cond bool, whenTrue, whenFalse Node) Node {
	if cond {
		return whenTrue
	}
	return whenFalse
}

// Each maps a slice []T into sibling Nodes using Go generics.
func Each[T any](items []T, fn func(T) Node) Node {
	return NodeFunc(func(w io.Writer) error {
		for _, item := range items {
			if n := fn(item); n != nil {
				if err := n.Render(w); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// EachSeq streams a Go 1.23+ iter.Seq[T] directly into HTML with zero intermediate slices.
func EachSeq[T any](seq iter.Seq[T], fn func(T) Node) Node {
	return NodeFunc(func(w io.Writer) error {
		for item := range seq {
			if n := fn(item); n != nil {
				if err := n.Render(w); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
