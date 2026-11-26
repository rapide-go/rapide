package foundation

import "sync"

// Provider hooks a framework module or third-party package into the application lifecycle.
// Register runs first for all providers (bindings only); Boot runs second (routes, listeners, commands).
type Provider interface {
	Register(app *App) error
	Boot(app *App) error
}

// BaseProvider can be embedded by providers that only need Register or Boot.
type BaseProvider struct{}

func (BaseProvider) Register(*App) error { return nil }
func (BaseProvider) Boot(*App) error     { return nil }

var (
	globalProvidersMu sync.Mutex
	globalProviders   []Provider
)

// AddProvider allows packages to self-register via init() (blank import style).
func AddProvider(p Provider) {
	globalProvidersMu.Lock()
	defer globalProvidersMu.Unlock()
	globalProviders = append(globalProviders, p)
}
