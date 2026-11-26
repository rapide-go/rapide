package inertia

import (
	"errors"
	"net/http"

	igo "github.com/rapide-go/inertia"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/session"
	"github.com/rapide-go/rapide/validation"
)

const (
	ctxKey           = "rapide.inertia"
	sessionFlashKey  = "inertia.flash"
	sessionErrorsKey = "inertia.errors"
)

// Middleware adapts github.com/rapide-go/inertia for Rapide's router.
// Handles asset version conflicts (409), pulls/flashes Rapide session data & validation error bags,
// syncs XSRF-TOKEN cookie, sets Vary: X-Inertia, and redirects empty 200 Inertia visits back.
func Middleware(cfg *igo.Config) router.Middleware {
	return MiddlewareWith(cfg, nil)
}

// MiddlewareWith works like Middleware and invokes shareFn on every request so apps can
// attach shared props (e.g. auth.user, app.name) in one place.
func MiddlewareWith(cfg *igo.Config, shareFn func(c *ctx.Ctx, in *igo.Inertia)) router.Middleware {
	if cfg == nil {
		cfg = igo.NewConfig()
	}

	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			req := igo.WrapRequest(c.Request)

			if igo.ApplyBefore(c.Writer, req, cfg) {
				return nil
			}

			in := igo.New(cfg, req)
			syncXSRFCookie(c)
			pullSessionState(c, in)
			if shareFn != nil {
				shareFn(c, in)
			}
			c.Set(ctxKey, in)

			err := next(c)
			igo.EnsureVary(c.Writer.Header())

			// On Inertia visits, flash validation.Errors into Rapide session (respecting X-Inertia-Error-Bag)
			// and redirect back so Inertia's useForm() populates form.errors automatically.
			if err != nil && req.IsInertia() && !c.Written() {
				if valErrs, ok := errors.AsType[validation.Errors](err); ok {
					flashValidationErrors(c, req, valErrs)
					in.Back(c.Writer)
					return nil
				}
				return err
			}
			if err != nil {
				return err
			}

			if req.IsInertia() && !c.Written() {
				in.Back(c.Writer)
			}
			return nil
		}
	}
}

func syncXSRFCookie(c *ctx.Ctx) {
	sess := session.Current(c)
	if sess == nil {
		return
	}
	token := sess.CSRFToken()
	if token == "" {
		return
	}
	c.SetCookie(&http.Cookie{
		Name:     "XSRF-TOKEN",
		Value:    token,
		Path:     "/",
		HttpOnly: false, // readable by Inertia / Axios
		SameSite: http.SameSiteLaxMode,
	})
}

// BackWithErrors flashes field-level validation errors into the session and redirects
// back with 303 See Other so Inertia's useForm() populates form.errors automatically.
func BackWithErrors(c *ctx.Ctx, errs map[string]string) error {
	if sess := session.Current(c); sess != nil && len(errs) > 0 {
		req := igo.WrapRequest(c.Request)
		if bag := req.ErrorBag(); bag != "" {
			sess.Flash(sessionErrorsKey, map[string]any{bag: errs})
		} else {
			sess.Flash(sessionErrorsKey, errs)
		}
	}
	From(c).Back(c.Writer)
	return nil
}

// Redirect issues a 303 See Other redirect (required by Inertia after POST/PUT/PATCH/DELETE).
func Redirect(c *ctx.Ctx, location string) error {
	return c.Redirect(303, location)
}

func pullSessionState(c *ctx.Ctx, in *igo.Inertia) {
	sess := session.Current(c)
	if sess == nil {
		return
	}
	if v, ok := sess.Pull(sessionFlashKey); ok {
		if m, ok := v.(map[string]any); ok {
			for k, val := range m {
				in.Flash(k, val)
			}
		}
	}
	if errs, ok := sess.Pull(sessionErrorsKey); ok {
		in.Share("errors", igo.Always(errs))
	}
}

func flashValidationErrors(c *ctx.Ctx, req *igo.Request, valErrs validation.Errors) {
	sess := session.Current(c)
	if sess == nil {
		return
	}
	flat := make(map[string]string, len(valErrs))
	for field, msgs := range valErrs {
		if len(msgs) > 0 {
			flat[field] = msgs[0]
		}
	}
	if bag := req.ErrorBag(); bag != "" {
		sess.Flash(sessionErrorsKey, map[string]any{bag: flat})
		return
	}
	sess.Flash(sessionErrorsKey, flat)
}

// FlashSession persists one-time flash data into the Rapide session so it survives a redirect.
func FlashSession(c *ctx.Ctx, key string, value any) {
	sess := session.Current(c)
	if sess == nil {
		return
	}
	m := map[string]any{key: value}
	if prev, ok := sess.Get("_flash." + sessionFlashKey); ok {
		if existing, ok := prev.(map[string]any); ok {
			existing[key] = value
			m = existing
		}
	}
	sess.Flash(sessionFlashKey, m)
}

// From retrieves the active *igo.Inertia handle from the Rapide request Ctx.
func From(c *ctx.Ctx) *igo.Inertia {
	if v, ok := c.Get(ctxKey); ok {
		return v.(*igo.Inertia)
	}
	cfg, err := container.TryUse[*igo.Config]()
	if err != nil {
		cfg = igo.NewConfig()
	}
	in := igo.New(cfg, igo.WrapRequest(c.Request))
	c.Set(ctxKey, in)
	return in
}

// Render resolves and sends an Inertia page directly through an Rapide *ctx.Ctx.
func Render(c *ctx.Ctx, component string, props igo.Props) error {
	return From(c).Render(component, props).Send(c.Writer)
}

// RenderStruct renders a strongly-typed Go struct T as an Inertia page's props.
func RenderStruct[T any](c *ctx.Ctx, component string, props T) error {
	m, err := igo.StructToProps(props)
	if err != nil {
		return err
	}
	return From(c).Render(component, m).Send(c.Writer)
}
