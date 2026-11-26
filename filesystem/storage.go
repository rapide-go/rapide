package storage

import (
	"context"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/support/manager"
	"github.com/rapide-go/rapide/support/option"
)

// Disk is the filesystem storage contract (Local, Memory, S3, MinIO, R2, etc.).
type Disk interface {
	Put(path string, data []byte) error
	Get(path string) ([]byte, error)
	Exists(path string) bool
	Delete(path string) error
}

// Manager returns the multi-disk *manager.Manager[Disk] bound in the container.
func Manager() *manager.Manager[Disk] {
	return container.Use[*manager.Manager[Disk]]()
}

// Storage returns the default Disk (e.g. FILESYSTEM_DISK=local|s3|minio).
func Storage() Disk {
	if m, err := container.TryUse[*manager.Manager[Disk]](); err == nil && m != nil {
		return m.Default()
	}
	return container.Use[Disk]()
}

// UseDisk resolves a specific named disk (e.g. storage.UseDisk("s3"), storage.UseDisk("minio"), storage.UseDisk("local")).
func UseDisk(name string) Disk {
	return Manager().Must(name)
}

// Extend registers a custom named disk factory.
func Extend(name string, fn manager.Factory[Disk]) {
	Manager().Extend(name, fn)
}

// Put writes data to path on the default Disk.
func Put(path string, data []byte) error { return Storage().Put(path, data) }

// Get reads path from the default Disk.
func Get(path string) ([]byte, error) { return Storage().Get(path) }

// Exists reports whether path exists on the default Disk.
func Exists(path string) bool { return Storage().Exists(path) }

// Delete removes path from the default Disk.
func Delete(path string) error { return Storage().Delete(path) }

// Provider binds a multi-disk manager.Manager[Disk] and default Disk into the application container.
type Provider struct {
	foundation.BaseProvider
	Default string
	Root    string
	Disks   map[string]manager.Factory[Disk]
	Options []option.Option[manager.Manager[Disk]]
}

func (p Provider) Register(app *foundation.App) error {
	root := p.Root
	if root == "" {
		root = "storage/app"
	}

	m := manager.New[Disk](
		manager.WithFactory[Disk]("local", func() (Disk, error) {
			return NewLocal(root), nil
		}),
		manager.WithFactory[Disk]("memory", func() (Disk, error) {
			return NewMemory(), nil
		}),
	)

	for name, fn := range p.Disks {
		m.Register(name, fn)
	}
	option.Apply(m, p.Options...)

	def := p.Default
	if def == "" {
		def = config.Env("FILESYSTEM_DISK", "local")
	}
	m.SetDefault(def)

	container.Bind[*manager.Manager[Disk]](app.Container, m)
	container.Bind[Disk](app.Container, m.Default())

	app.OnShutdown(func(ctx context.Context) error {
		return m.CloseAll()
	})
	return nil
}
