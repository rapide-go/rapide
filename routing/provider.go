package router

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/http/ctx"
)

// Provider creates and binds *Router into the container, mounts global middleware,
// serves optional embedded static build assets, and invokes all route registration callbacks
// .
type Provider struct {
	foundation.BaseProvider
	Middleware   []Middleware
	Routes       []func(r *Router)
	AssetsFS     fs.FS
	AssetsDir    string
	AssetsPrefix string
	Configure    func(r *Router)
}

func (p Provider) Register(app *foundation.App) error {
	r := New()
	if len(p.Middleware) > 0 {
		r.Use(p.Middleware...)
	}
	app.Container.Bind(r)
	return nil
}

func (p Provider) Boot(app *foundation.App) error {
	r := container.Resolve[*Router](app.Container)

	if p.AssetsFS != nil {
		dir := p.AssetsDir
		if dir == "" {
			dir = "build"
		}
		prefix := p.AssetsPrefix
		if prefix == "" {
			prefix = "/" + strings.Trim(dir, "/")
		}
		if sub, err := fs.Sub(p.AssetsFS, dir); err == nil {
			strip := strings.TrimRight(prefix, "/") + "/"
			fileServer := http.StripPrefix(strip, http.FileServer(http.FS(sub)))
			r.Get(strip+"{path...}", func(c *ctx.Ctx) error {
				c.Writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				fileServer.ServeHTTP(c.Writer, c.Request)
				return nil
			})
		}
	}

	for _, fn := range p.Routes {
		if fn != nil {
			fn(r)
		}
	}
	if p.Configure != nil {
		p.Configure(r)
	}
	return nil
}
