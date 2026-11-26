package rapide

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"github.com/rapide-go/rapide/auth/socialite"
	"github.com/rapide-go/rapide/cache"
	"github.com/rapide-go/rapide/config"
	schedule "github.com/rapide-go/rapide/console/scheduling"
	"github.com/rapide-go/rapide/database/query"
	"github.com/rapide-go/rapide/database/schema"
	storage "github.com/rapide-go/rapide/filesystem"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/http/health"
	"github.com/rapide-go/rapide/http/inertia"
	"github.com/rapide-go/rapide/http/openapi"
	"github.com/rapide-go/rapide/http/typegen"
	"github.com/rapide-go/rapide/lang"
	applog "github.com/rapide-go/rapide/log"
	"github.com/rapide-go/rapide/mail"
	"github.com/rapide-go/rapide/queue"
	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/session"
	"github.com/rapide-go/rapide/support/option"
)

// Option is Rapide's reusable generic functional option for any configurable type T.
type Option[T any] = option.Option[T]

// Apply invokes all non-nil generic options against target and returns target.
func Apply[T any](target *T, opts ...Option[T]) *T {
	return option.Apply(target, opts...)
}

// App is the high-level Rapide application entrypoint bundling the kernel,
// HTTP router, and production server configuration.
type App struct {
	Kernel *foundation.App
	Router *router.Router
	Addr   string
	Server foundation.ServerOptions

	envFile           string
	debug             *bool
	skipDefaults      bool
	enableDB          bool
	migrations        []schema.Migration
	scheduleFn        func(*schedule.Scheduler)
	assetsFS          fs.FS
	assetsDir         string
	enableHealth      bool
	healthChecks      []health.Check
	openAPITitle      string
	openAPIVersion    string
	enableLiveInertia bool
	liveInertiaPath   string
	enableTypeGen     bool
	typeGenDir        string
	providers         []foundation.Provider
	middleware        []router.Middleware
	routeFuncs        []func(*router.Router)
	configBinders     []config.Binder
	configHooks       []func(*config.Repository) error
	exceptionHooks    []func()
}

// New creates and boots a Rapide App with sensible default core providers.
// Any provider passed via WithProviders(...) overrides the matching default provider by type.
func New(opts ...Option[App]) (*App, error) {
	app := &App{
		Addr:    ":8080",
		Server:  foundation.DefaultServerOptions(),
		envFile: ".env",
	}
	Apply(app, opts...)

	if app.Router == nil {
		app.Router = router.New()
	}

	defBinders, defMigs, defSchedules, defAssetsFS, defAssetsDir := snapshotRegistry()
	allBinders := append(defBinders, app.configBinders...)
	allMigs := append(defMigs, app.migrations...)
	if len(allMigs) > 0 {
		app.enableDB = true
	}
	if app.assetsFS == nil && defAssetsFS != nil {
		app.assetsFS = defAssetsFS
		app.assetsDir = defAssetsDir
	}

	var combinedSchedule func(*schedule.Scheduler)
	if len(defSchedules) > 0 || app.scheduleFn != nil {
		combinedSchedule = func(s *schedule.Scheduler) {
			for _, fn := range defSchedules {
				fn(s)
			}
			if app.scheduleFn != nil {
				app.scheduleFn(s)
			}
		}
	}

	base := []foundation.Provider{
		config.Provider{
			EnvFile: app.envFile,
			Bind:    allBinders,
			Configure: func(r *config.Repository) error {
				for _, fn := range app.configHooks {
					if err := fn(r); err != nil {
						return err
					}
				}
				return nil
			},
		},
	}

	if !app.skipDefaults {
		base = append(base, applog.Provider{})
		if app.enableDB {
			base = append(base, query.Provider{Migrations: allMigs})
		}
		base = append(base,
			session.Provider{},
			lang.Provider{},
			socialite.ServiceProvider{},
			cache.Provider{},
			storage.Provider{},
			mail.Provider{},
			queue.Provider{},
			schedule.Provider{Configure: combinedSchedule},
		)
	}

	merged := foundation.MergeProviders(base, app.providers)
	app.Kernel = foundation.New(merged...)
	app.Kernel.Container.Bind(app.Router)
	app.Kernel.OnShutdown(ctx.DrainDeferred)

	for _, fn := range app.exceptionHooks {
		fn()
	}

	if err := app.Kernel.Boot(); err != nil {
		return nil, err
	}

	if len(app.middleware) > 0 {
		app.Router.Use(app.middleware...)
	}
	app.mountStaticAssets()
	for _, register := range app.routeFuncs {
		if register != nil {
			register(app.Router)
		}
	}
	if app.enableHealth {
		checks := app.healthChecks
		if len(checks) == 0 && app.enableDB {
			checks = []health.Check{health.DBCheck("database", query.DB())}
		}
		health.Mount(app.Router, checks...)
	}
	if app.openAPITitle != "" {
		openapi.Mount(app.Router, app.openAPITitle, app.openAPIVersion)
	}
	if app.enableLiveInertia {
		inertia.MountLive(app.Router, app.liveInertiaPath)
	}
	if app.enableTypeGen && config.Get("app.env", "local") != "production" {
		if err := typegen.Generate(app.Router, app.typeGenDir); err != nil {
			applog.Warn("typegen generation skipped", "error", err)
		}
	}

	if app.debug != nil {
		ctx.DebugMode = *app.debug
	} else {
		ctx.DebugMode = config.Get("app.debug", false)
	}

	return app, nil
}

func (a *App) mountStaticAssets() {
	if a.assetsFS == nil {
		return
	}
	dir := a.assetsDir
	if dir == "" {
		dir = "build"
	}
	if sub, err := fs.Sub(a.assetsFS, dir); err == nil {
		prefix := "/" + strings.Trim(dir, "/") + "/"
		fileServer := http.StripPrefix(prefix, http.FileServer(http.FS(sub)))
		a.Router.Get(prefix+"{path...}", func(c *ctx.Ctx) error {
			c.Writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			fileServer.ServeHTTP(c.Writer, c.Request)
			return nil
		})
	}
}

// ServeHTTP implements http.Handler so *rapide.App works directly with httptest or custom servers.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Router.ServeHTTP(w, r)
}

// Serve starts the hardened production HTTP server and gracefully shuts down on SIGINT/SIGTERM.
func (a *App) Serve() error {
	return a.Kernel.ServeWithOptions(a.Addr, a.Router, a.Server)
}

// Shutdown gracefully drains the HTTP server and runs all registered kernel shutdown hooks.
func (a *App) Shutdown(c context.Context) error {
	return a.Kernel.Shutdown(c)
}
