package session

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/database/query"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/support/manager"
)

// Store abstracts session persistence (MemoryStore, SQLStore, Redis, etc.).
type Store interface {
	Load(ctx context.Context, id string) (map[string]any, time.Time, bool, error)
	Save(ctx context.Context, id string, data map[string]any, expiresAt time.Time) error
	Delete(ctx context.Context, id string) error
	Sweep(ctx context.Context, now time.Time) error
}

// MemoryStore is the default thread-safe in-memory session Store.
type MemoryStore struct {
	mu        sync.RWMutex
	items     map[string]memEntry
	lastSweep time.Time
}

type memEntry struct {
	data      map[string]any
	expiresAt time.Time
}

// NewMemoryStore creates an in-memory session Store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items:     make(map[string]memEntry),
		lastSweep: time.Now(),
	}
}

func (m *MemoryStore) Load(_ context.Context, id string) (map[string]any, time.Time, bool, error) {
	now := time.Now()
	m.mu.RLock()
	entry, ok := m.items[id]
	m.mu.RUnlock()
	if !ok {
		return nil, time.Time{}, false, nil
	}
	if now.After(entry.expiresAt) {
		_ = m.Delete(context.Background(), id)
		return nil, time.Time{}, false, nil
	}
	cp := make(map[string]any, len(entry.data))
	maps.Copy(cp, entry.data)
	return cp, entry.expiresAt, true, nil
}

func (m *MemoryStore) Save(_ context.Context, id string, data map[string]any, expiresAt time.Time) error {
	now := time.Now()
	m.mu.Lock()
	m.items[id] = memEntry{data: data, expiresAt: expiresAt}
	if now.Sub(m.lastSweep) > 5*time.Minute && len(m.items) > 256 {
		for k, e := range m.items {
			if now.After(e.expiresAt) {
				delete(m.items, k)
			}
		}
		m.lastSweep = now
	}
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	delete(m.items, id)
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) Sweep(_ context.Context, now time.Time) error {
	m.mu.Lock()
	for k, e := range m.items {
		if now.After(e.expiresAt) {
			delete(m.items, k)
		}
	}
	m.lastSweep = now
	m.mu.Unlock()
	return nil
}

func tryManager() (*manager.Manager[Store], error) {
	return container.TryUse[*manager.Manager[Store]]()
}

// Manager returns the multi-driver *manager.Manager[Store] bound in the container.
func Manager() *manager.Manager[Store] {
	return container.Use[*manager.Manager[Store]]()
}

// DefaultStore returns the default session Store bound in the container.
func DefaultStore() Store {
	if m, err := container.TryUse[*manager.Manager[Store]](); err == nil && m != nil {
		return m.Default()
	}
	return container.Use[Store]()
}

// Provider registers session Stores into the Rapide container.
type Provider struct {
	foundation.BaseProvider
	Default string
	Stores  map[string]Store
}

func (p Provider) Register(app *foundation.App) error {
	m := manager.New[Store]()
	m.Set("memory", NewMemoryStore())
	for name, st := range p.Stores {
		m.Set(name, st)
	}
	def := p.Default
	if def == "" {
		def = config.Get("session.driver", "memory")
	}
	if (def == "sql" || def == "database") && !slices.Contains(m.Names(), def) {
		table := config.Get("session.table", "sessions")
		m.Register(def, func() (Store, error) {
			return NewSQLStore(query.DB(), query.DefaultDialect, table)
		})
	}
	if def != "" && slices.Contains(m.Names(), def) {
		m.SetDefault(def)
	}
	container.Bind[*manager.Manager[Store]](app.Container, m)
	return nil
}
