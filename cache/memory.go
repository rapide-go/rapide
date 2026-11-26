package cache

import (
	"sync"
	"time"
)

type item struct {
	value     any
	expiresAt time.Time
}

// Memory is a thread-safe in-memory cache Store with TTL expiration and periodic eviction.
type Memory struct {
	mu      sync.RWMutex
	data    map[string]item
	stopCh  chan struct{}
	stopped sync.Once
}

func NewMemory() *Memory {
	return &Memory{
		data:   make(map[string]item),
		stopCh: make(chan struct{}),
	}
}

// StartJanitor periodically purges expired keys to prevent unbounded memory growth.
func (m *Memory) StartJanitor(interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.Sweep()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Sweep removes all currently expired entries.
func (m *Memory) Sweep() {
	now := time.Now()
	m.mu.Lock()
	for k, it := range m.data {
		if !it.expiresAt.IsZero() && now.After(it.expiresAt) {
			delete(m.data, k)
		}
	}
	m.mu.Unlock()
}

// Close stops the background janitor goroutine and satisfies io.Closer.
func (m *Memory) Close() error {
	m.stopped.Do(func() {
		close(m.stopCh)
	})
	return nil
}

func (m *Memory) Get(key string) (any, bool) {
	m.mu.RLock()
	it, ok := m.data[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !it.expiresAt.IsZero() && time.Now().After(it.expiresAt) {
		m.Forget(key)
		return nil, false
	}
	return it.value, true
}

func (m *Memory) Put(key string, value any, ttl time.Duration) {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.data[key] = item{value: value, expiresAt: exp}
	m.mu.Unlock()
}

// Touch extends key's expiration in place without copying or decoding the value.
func (m *Memory) Touch(key string, ttl time.Duration) bool {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.data[key]
	if !ok {
		return false
	}
	if !it.expiresAt.IsZero() && now.After(it.expiresAt) {
		delete(m.data, key)
		return false
	}
	if ttl > 0 {
		it.expiresAt = now.Add(ttl)
	} else {
		it.expiresAt = time.Time{}
	}
	m.data[key] = it
	return true
}

func (m *Memory) Forget(key string) {
	m.mu.Lock()
	delete(m.data, key)
	m.mu.Unlock()
}

func (m *Memory) Flush() {
	m.mu.Lock()
	m.data = make(map[string]item)
	m.mu.Unlock()
}
