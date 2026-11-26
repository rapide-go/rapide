package inertia

import (
	igo "github.com/rapide-go/inertia"
	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/routing"
)

// TypedPage binds an Inertia component name to a strongly-typed Go props struct P for Rapide handlers.
type TypedPage[P any] struct {
	Component string
}

// DefinePage declares a compile-time typed Inertia page for Rapide.
func DefinePage[P any](component string) TypedPage[P] {
	return TypedPage[P]{Component: component}
}

// Render builds and sends the TypedPage with strongly-typed props P through Rapide *ctx.Ctx.
func (tp TypedPage[P]) Render(c *ctx.Ctx, props P) error {
	return RenderStruct(c, tp.Component, props)
}

// Response converts typed props P into a fluent *igo.Response so callers can chain
// .With(...), .WithViewData(...), .Status(...), or .Flash(...) before calling .Send(c.Writer).
func (tp TypedPage[P]) Response(c *ctx.Ctx, props P) (*igo.Response, error) {
	m, err := igo.StructToProps(props)
	if err != nil {
		return nil, err
	}
	return From(c).Render(tp.Component, m), nil
}

// FormAction adapts a strongly-typed Inertia form handler `func(*ctx.Ctx, *igo.Inertia, In) error`
// into an Rapide router.Handler:
//   - Decodes & validates `In` automatically via ctx.Bind[In]
//   - On validation failure during an Inertia visit, Middleware flashes errors + redirects back
//   - Invokes fn with the typed input and active *igo.Inertia handle
func FormAction[In any](fn func(c *ctx.Ctx, in *igo.Inertia, form In) error) router.Handler {
	return func(c *ctx.Ctx) error {
		form, err := ctx.Bind[In](c)
		if err != nil {
			return err
		}
		return fn(c, From(c), form)
	}
}
