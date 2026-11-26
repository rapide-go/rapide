package cache

import (
	"context"
	"time"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/support/manager"
	"github.com/rapide-go/rapide/support/option"
)

// Provider binds a multi-store manager.Manager[Store] and manages janitor/closer lifecycles.
type Provider struct {
	foundation.BaseProvider
	Default         string
	Driver          Store
	Stores          map[string]manager.Factory[Store]
	Options         []option.Option[manager.Manager[Store]]
	JanitorInterval time.Duration
}

func (p Provider) Register(app *foundation.App) error {
	m := manager.New[Store](
		manager.WithFactory[Store]("memory", func() (Store, error) {
			mem := NewMemory()
			mem.StartJanitor(p.JanitorInterval)
			return mem, nil
		}),
	)

	if p.Driver != nil {
		m.Set("default", p.Driver)
		m.SetDefault("default")
	}
	for name, fn := range p.Stores {
		m.Register(name, fn)
	}
	option.Apply(m, p.Options...)

	if p.Default != "" {
		m.SetDefault(p.Default)
	} else if p.Driver == nil {
		m.SetDefault(config.Get("cache.driver", "memory"))
	}

	container.Bind[*manager.Manager[Store]](app.Container, m)
	app.OnShutdown(func(ctx context.Context) error {
		return m.CloseAll()
	})
	return nil
}
