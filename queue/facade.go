package queue

import (
	"context"
	"time"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/support/manager"
)

// Manager returns the multi-queue *manager.Manager[*Pool] bound in the container.
func Manager() *manager.Manager[*Pool] {
	return container.Use[*manager.Manager[*Pool]]()
}

// Queue is the facade accessor returning the default *Pool from the container.
func Queue() *Pool {
	if m, err := container.TryUse[*manager.Manager[*Pool]](); err == nil && m != nil {
		return m.Default()
	}
	return container.Use[*Pool]()
}

// UseQueue resolves a specific named queue pool (e.g. queue.UseQueue("emails"), queue.UseQueue("heavy")).
func UseQueue(name string) *Pool {
	return Manager().Must(name)
}

// Dispatch enqueues any Job onto the default Queue pool.
func Dispatch(job Job, retries ...int) error {
	return Queue().Push(job, retries...)
}

// DispatchAfter delays enqueuing job on the default Queue pool by delay.
func DispatchAfter(delay time.Duration, job Job, retries ...int) {
	Queue().PushAfter(delay, job, retries...)
}

// DispatchFunc enqueues a closure as a background job.
func DispatchFunc(fn func(ctx context.Context) error, retries ...int) error {
	return Queue().Push(JobFunc(fn), retries...)
}

// Provider binds a multi-queue manager.Manager[*Pool] and manages worker lifecycles with the App.
type Provider struct {
	foundation.BaseProvider
	Default  string
	Workers  int
	Buffer   int
	Queues   map[string]PoolConfig
	OnFailed FailedHandler
}

func (p Provider) Register(app *foundation.App) error {
	workers := p.Workers
	if workers <= 0 {
		workers = config.Get("queue.workers", 4)
	}
	buf := p.Buffer
	if buf <= 0 {
		buf = config.Get("queue.buffer", 256)
	}

	m := manager.New[*Pool]()
	defPool := NewPool(workers, buf)
	if p.OnFailed != nil {
		defPool.OnFailed(p.OnFailed)
	}
	m.Set("default", defPool)

	for name, cfg := range p.Queues {
		pool := NewPool(cfg.Workers, cfg.Buffer)
		if p.OnFailed != nil {
			pool.OnFailed(p.OnFailed)
		}
		m.Set(name, pool)
	}
	if p.Default != "" {
		m.SetDefault(p.Default)
	}

	container.Bind[*manager.Manager[*Pool]](app.Container, m)
	container.Bind[*Pool](app.Container, m.Default())
	return nil
}

func (p Provider) Boot(app *foundation.App) error {
	m := container.Resolve[*manager.Manager[*Pool]](app.Container)
	for _, name := range m.Names() {
		pool := m.Must(name)
		pool.Start(context.Background())
		app.OnShutdown(pool.Stop)
	}
	return nil
}
