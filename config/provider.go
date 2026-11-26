package config

import (
	"os"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
)

// Binder registers a typed config section onto a *Repository.
type Binder func(r *Repository) error

// Section returns a Binder that loads and binds struct T under namespace (e.g. config.Section[appcfg.App]("app")).
func Section[T any](namespace string) Binder {
	return func(r *Repository) error {
		_, err := BindStruct[T](r, namespace)
		return err
	}
}

// RegisterAll runs every Binder in order, stopping at the first error.
func RegisterAll(r *Repository, binders ...Binder) error {
	for _, b := range binders {
		if b == nil {
			continue
		}
		if err := b(r); err != nil {
			return err
		}
	}
	return nil
}

// Provider loads `.env` (and optional `.env.<APP_ENV>`), binds the global *config.Repository
// into the container, and binds all configured typed config structs once at boot.
type Provider struct {
	foundation.BaseProvider
	EnvFile   string
	Bind      []Binder
	Configure func(r *Repository) error
}

func (p Provider) Register(app *foundation.App) error {
	envFile := p.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	if err := LoadEnv(envFile); err != nil {
		return err
	}
	if appEnv := os.Getenv("APP_ENV"); appEnv != "" {
		_ = LoadEnv(envFile + "." + appEnv)
	}

	container.Bind[*Repository](app.Container, Default)
	if len(p.Bind) > 0 {
		if err := RegisterAll(Default, p.Bind...); err != nil {
			return err
		}
	}
	if p.Configure != nil {
		return p.Configure(Default)
	}
	return nil
}
