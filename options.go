package rapide

import (
	"fmt"
	"io/fs"
	"time"

	"github.com/rapide-go/rapide/config"
	schedule "github.com/rapide-go/rapide/console/scheduling"
	"github.com/rapide-go/rapide/database/schema"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/http/health"
	router "github.com/rapide-go/rapide/routing"
)

// WithPort configures the HTTP listening port (e.g. WithPort(8080) -> ":8080").
func WithPort(port int) Option[App] {
	return func(a *App) {
		if port > 0 {
			a.Addr = fmt.Sprintf(":%d", port)
		}
	}
}

// WithAddr sets the exact network listen address (e.g. "127.0.0.1:8080").
func WithAddr(addr string) Option[App] {
	return func(a *App) {
		if addr != "" {
			a.Addr = addr
		}
	}
}

func WithReadTimeout(d time.Duration) Option[App] { return func(a *App) { a.Server.ReadTimeout = d } }
func WithReadHeaderTimeout(d time.Duration) Option[App] {
	return func(a *App) { a.Server.ReadHeaderTimeout = d }
}
func WithWriteTimeout(d time.Duration) Option[App] { return func(a *App) { a.Server.WriteTimeout = d } }
func WithIdleTimeout(d time.Duration) Option[App]  { return func(a *App) { a.Server.IdleTimeout = d } }
func WithShutdownTimeout(d time.Duration) Option[App] {
	return func(a *App) { a.Server.ShutdownTimeout = d }
}
func WithMaxHeaderBytes(n int) Option[App] { return func(a *App) { a.Server.MaxHeaderBytes = n } }

func WithEnvFile(path string) Option[App] { return func(a *App) { a.envFile = path } }
func WithDebug(enabled bool) Option[App]  { return func(a *App) { a.debug = new(enabled) } }

// WithConfig attaches a callback invoked during config.Provider.Register.
func WithConfig(fn func(*config.Repository) error) Option[App] {
	return func(a *App) {
		if fn != nil {
			a.configHooks = append(a.configHooks, fn)
		}
	}
}

// WithConfigBinders registers typed config.Section[T] binders at boot.
func WithConfigBinders(binders ...config.Binder) Option[App] {
	return func(a *App) {
		a.configBinders = append(a.configBinders, binders...)
	}
}

// WithDatabase enables the built-in SQL database provider using config (`database.driver`, `database.dsn`).
func WithDatabase() Option[App] {
	return func(a *App) { a.enableDB = true }
}

// WithMigrations enables the built-in SQL database provider and runs the given migrations on Boot.
func WithMigrations(migs ...schema.Migration) Option[App] {
	return func(a *App) {
		a.enableDB = true
		a.migrations = append(a.migrations, migs...)
	}
}

// WithSchedule registers scheduled tasks on the built-in schedule.Provider.
func WithSchedule(fn func(*schedule.Scheduler)) Option[App] {
	return func(a *App) { a.scheduleFn = fn }
}

// WithAssets serves embedded immutable production assets (e.g. Vite's public.BuildFS under "build").
func WithAssets(fsys fs.FS, dir ...string) Option[App] {
	return func(a *App) {
		a.assetsFS = fsys
		if len(dir) > 0 && dir[0] != "" {
			a.assetsDir = dir[0]
		}
	}
}

// WithHealth mounts the `/up` health-check endpoint with optional readiness checks.
func WithHealth(checks ...health.Check) Option[App] {
	return func(a *App) {
		a.enableHealth = true
		a.healthChecks = append(a.healthChecks, checks...)
	}
}

// WithOpenAPI mounts auto-generated OpenAPI 3.1 docs at `/docs`.
func WithOpenAPI(title, version string) Option[App] {
	return func(a *App) {
		a.openAPITitle = title
		a.openAPIVersion = version
	}
}

// WithLiveInertia mounts Rapide's multiplexed SSE live-props endpoint (default `/_inertia/live`)
// so `inertia.Live(...)` props auto-reload when server events fire.
func WithLiveInertia(path ...string) Option[App] {
	return func(a *App) {
		a.enableLiveInertia = true
		if len(path) > 0 && path[0] != "" {
			a.liveInertiaPath = path[0]
		}
	}
}

// WithTypeGen auto-generates TypeScript types, typed route helpers, and the live-props client
// (`types.ts`, `routes.ts`, `live.ts`) on boot in local/dev environments (default `resources/js/rapide`).
func WithTypeGen(outDir ...string) Option[App] {
	return func(a *App) {
		a.enableTypeGen = true
		if len(outDir) > 0 && outDir[0] != "" {
			a.typeGenDir = outDir[0]
		}
	}
}

// WithProviders registers app providers or overrides matching default framework providers by type.
func WithProviders(providers ...foundation.Provider) Option[App] {
	return func(a *App) {
		a.providers = append(a.providers, providers...)
	}
}

// WithoutDefaultProviders disables automatic registration of Rapide's default core providers.
func WithoutDefaultProviders() Option[App] {
	return func(a *App) { a.skipDefaults = true }
}

func WithMiddleware(mw ...router.Middleware) Option[App] {
	return func(a *App) { a.middleware = append(a.middleware, mw...) }
}

func WithRoutes(fns ...func(*router.Router)) Option[App] {
	return func(a *App) { a.routeFuncs = append(a.routeFuncs, fns...) }
}

// WithExceptions registers exception reporting/rendering hooks (ctx.Report, ctx.RenderError)
// declaratively inside bootstrap/app.go.
func WithExceptions(fn func()) Option[App] {
	return func(a *App) {
		if fn != nil {
			a.exceptionHooks = append(a.exceptionHooks, fn)
		}
	}
}

func WithRouter(r *router.Router) Option[App] {
	return func(a *App) {
		if r != nil {
			a.Router = r
		}
	}
}
