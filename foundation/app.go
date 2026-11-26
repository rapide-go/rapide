package foundation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"sync"

	"github.com/rapide-go/rapide/concurrency"
	"github.com/rapide-go/rapide/container"
)

// ShutdownHook is invoked during graceful application shutdown.
type ShutdownHook func(ctx context.Context) error

// App is the central Rapide application kernel.
type App struct {
	Container *container.Container

	mu            sync.Mutex
	providers     []Provider
	bootingHooks  []LifecycleHook
	bootedHooks   []LifecycleHook
	shutdownHooks []ShutdownHook
	extensions    *extensionStore
	booted        bool
	httpServer    *http.Server
}

// New constructs an App wired to the global default container and registers any provided providers.
func New(providers ...Provider) *App {
	app := &App{
		Container:  container.Default,
		extensions: newExtensionStore(),
	}

	globalProvidersMu.Lock()
	app.providers = append(app.providers, globalProviders...)
	globalProvidersMu.Unlock()

	app.providers = append(app.providers, providers...)
	container.Bind[*App](app.Container, app)
	return app
}

// MergeProviders combines base default providers with user overrides.
// Any override matching the concrete type of a base provider replaces it in place,
// preserving deterministic boot order.
func MergeProviders(base, overrides []Provider) []Provider {
	out := make([]Provider, 0, len(base)+len(overrides))
	idxByType := make(map[reflect.Type]int, len(base))
	for _, p := range base {
		if p == nil {
			continue
		}
		t := reflect.TypeOf(p)
		if idx, exists := idxByType[t]; exists {
			out[idx] = p
			continue
		}
		idxByType[t] = len(out)
		out = append(out, p)
	}
	for _, p := range overrides {
		if p == nil {
			continue
		}
		t := reflect.TypeOf(p)
		if idx, exists := idxByType[t]; exists {
			out[idx] = p
			continue
		}
		idxByType[t] = len(out)
		out = append(out, p)
	}
	return out
}

// OnBooting registers a hook invoked after all providers Register(), right before Boot().
func (a *App) OnBooting(fn LifecycleHook) {
	a.mu.Lock()
	a.bootingHooks = append(a.bootingHooks, fn)
	a.mu.Unlock()
}

// OnBooted registers a hook invoked after all providers finish Boot().
func (a *App) OnBooted(fn LifecycleHook) {
	a.mu.Lock()
	a.bootedHooks = append(a.bootedHooks, fn)
	a.mu.Unlock()
}

// Register appends one or more service providers to the application.
func (a *App) Register(providers ...Provider) *App {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.providers = append(a.providers, providers...)
	return a
}

// OnShutdown registers a cleanup callback executed in reverse order on shutdown.
func (a *App) OnShutdown(fn ShutdownHook) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdownHooks = append(a.shutdownHooks, fn)
}

// Boot executes Phase 1 (Register) across all providers, then Phase 2 (Boot).
func (a *App) Boot() error {
	a.mu.Lock()
	if a.booted {
		a.mu.Unlock()
		return nil
	}
	providers := append([]Provider(nil), a.providers...)
	a.mu.Unlock()

	for _, p := range providers {
		if err := p.Register(a); err != nil {
			return fmt.Errorf("foundation: provider register failed (%T): %w", p, err)
		}
	}

	a.mu.Lock()
	booting := append([]LifecycleHook(nil), a.bootingHooks...)
	a.mu.Unlock()
	for _, fn := range booting {
		if err := fn(a); err != nil {
			return fmt.Errorf("foundation: booting hook failed: %w", err)
		}
	}

	for _, p := range providers {
		if err := p.Boot(a); err != nil {
			return fmt.Errorf("foundation: provider boot failed (%T): %w", p, err)
		}
	}

	a.mu.Lock()
	booted := append([]LifecycleHook(nil), a.bootedHooks...)
	a.booted = true
	a.mu.Unlock()
	for _, fn := range booted {
		if err := fn(a); err != nil {
			return fmt.Errorf("foundation: booted hook failed: %w", err)
		}
	}
	return nil
}

// Shutdown gracefully stops the HTTP server and runs all registered shutdown hooks in LIFO order.
func (a *App) Shutdown(ctx context.Context) error {
	var errs []error

	if a.httpServer != nil {
		if err := a.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	a.mu.Lock()
	hooks := append([]ShutdownHook(nil), a.shutdownHooks...)
	a.mu.Unlock()

	for _, hook := range slices.Backward(hooks) {
		if err := hook(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if err := conc.Drain(ctx); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
