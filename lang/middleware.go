package lang

import (
	"io/fs"
	"strings"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/support/ctxkey"
)

var localeKey = ctxkey.New[string]()

// Middleware detects the request locale from `?lang=`, `X-Locale`, or `Accept-Language`
// and stores it on the request context via ctxkey.Key[string].
// When defaultLocale is empty, it falls back to `app.locale` from config (default "en").
func Middleware(defaultLocale string) router.Middleware {
	if defaultLocale == "" {
		defaultLocale = config.Get("app.locale", "en")
	}
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			loc := c.Query("lang")
			if loc == "" {
				loc = c.Header("X-Locale")
			}
			if loc == "" {
				if al := c.Header("Accept-Language"); al != "" {
					first, _, _ := strings.Cut(al, ",")
					first, _, _ = strings.Cut(first, ";")
					loc = first
				}
			}
			if loc == "" {
				loc = defaultLocale
			}
			c.Request = c.Request.WithContext(localeKey.With(c.Request.Context(), normalizeLocale(loc)))
			return next(c)
		}
	}
}

// Locale returns the active locale stored on the request context (default "en").
func Locale(c *ctx.Ctx) string {
	if c == nil || c.Request == nil {
		return "en"
	}
	return localeKey.MustGet(c.Request.Context(), "en")
}

// Default returns the global *Translator from the container.
func Default() *Translator {
	return container.Use[*Translator]()
}

// T translates key for the current request's locale using the container-bound *Translator.
func T(c *ctx.Ctx, key string, params ...map[string]any) string {
	return Default().Get(Locale(c), key, params...)
}

// Provider registers a *Translator into the Rapide container and loads FS/map dictionaries.
type Provider struct {
	foundation.BaseProvider
	Fallback string
	FS       fs.FS
	Dir      string
	Maps     map[string]map[string]string
}

func (p Provider) Register(app *foundation.App) error {
	fb := p.Fallback
	if fb == "" {
		fb = config.Get("app.fallback_locale", "en")
	}
	tr := New(fb)
	if p.FS != nil {
		if err := tr.LoadFS(p.FS, p.Dir); err != nil {
			return err
		}
	}
	for loc, dict := range p.Maps {
		tr.LoadMap(loc, dict)
	}
	container.Bind[*Translator](app.Container, tr)
	return nil
}
