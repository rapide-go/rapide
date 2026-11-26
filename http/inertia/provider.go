package inertia

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"strings"

	igo "github.com/rapide-go/inertia-go"
	"github.com/rapide-go/rapide/auth"
	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/view"
)

// ShareAuth returns an Inertia share callback that resolves the logged-in user U from session
// via loader and shares `auth: { user: ... }` on every Inertia response.
func ShareAuth[U any](loader func(id any) (U, error)) func(c *ctx.Ctx, in *igo.Inertia) {
	return func(c *ctx.Ctx, in *igo.Inertia) {
		if u, ok := auth.ResolveSession(c, loader); ok {
			in.Share("auth", map[string]any{"user": u})
			return
		}
		in.Share("auth", map[string]any{"user": nil})
	}
}

// MiddlewareFromConfig resolves the *igo.Config bound by inertia.Provider lazily at request time
// and attaches optional shared-prop callbacks.
func MiddlewareFromConfig(shareFn ...func(c *ctx.Ctx, in *igo.Inertia)) router.Middleware {
	var fn func(c *ctx.Ctx, in *igo.Inertia)
	if len(shareFn) > 0 {
		fn = shareFn[0]
	}
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			cfg, err := container.TryUse[*igo.Config]()
			if err != nil || cfg == nil {
				cfg = igo.NewConfig()
			}
			return MiddlewareWith(cfg, fn)(next)(c)
		}
	}
}

// Provider configures Vite asset resolution and binds a default Inertia *igo.Config
// with an HTML5 root shell into the Rapide container.
type Provider struct {
	foundation.BaseProvider
	Config       *igo.Config
	Title        string
	Entry        string
	HotFile      string
	ManifestFS   fs.FS
	ManifestPath string
	BuildPrefix  string
	Watch        bool
}

func (p Provider) Register(app *foundation.App) error {
	if p.Config != nil {
		app.Container.Bind(p.Config)
		return nil
	}
	title := p.Title
	if title == "" {
		title = config.Get("app.name", "Rapide")
	}
	entry := p.Entry
	if entry == "" {
		entry = "resources/js/app.js"
	}
	hotFile := p.HotFile
	if hotFile == "" {
		hotFile = "public/hot"
	}
	manifestPath := p.ManifestPath
	if manifestPath == "" {
		manifestPath = "build/manifest.json"
	}
	buildPrefix := p.BuildPrefix
	if buildPrefix == "" {
		buildPrefix = "/build"
	}

	view.ConfigureVite(view.ViteConfig{
		HotFile:      hotFile,
		ManifestFS:   p.ManifestFS,
		ManifestPath: manifestPath,
		BuildPrefix:  buildPrefix,
		Watch:        p.Watch,
	})

	inertiaCfg := igo.NewConfig().
		VersionFunc(igo.VersionFromFile("public/"+manifestPath, true)).
		RootView(func(v *igo.View) string {
			raw, _ := json.Marshal(v.Page)
			pageAttr := html.EscapeString(string(raw))
			viteTags := string(view.ViteTags(entry))
			return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s</title>
  %s
  %s
</head>
<body>
  <div id="app" data-page="%s"></div>
</body>
</html>`, html.EscapeString(title), strings.Join(v.Head, "\n"), viteTags, pageAttr)
		})

	app.Container.Bind(inertiaCfg)
	return nil
}
