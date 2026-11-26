package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Local stores files on the local filesystem using Go's OS-confined os.Root
// to prevent both lexical (../) and symlink path-traversal escapes.
type Local struct {
	root string
}

func NewLocal(root string) *Local {
	return &Local{root: filepath.Clean(root)}
}

func cleanRel(rel string) (string, error) {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimLeft(rel, `/\`)))
	if cleaned == "" || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("storage: path escapes root directory")
	}
	return cleaned, nil
}

func (l *Local) openRoot() (*os.Root, error) {
	if err := os.MkdirAll(l.root, 0o755); err != nil {
		return nil, err
	}
	return os.OpenRoot(l.root)
}

func (l *Local) Put(path string, data []byte) error {
	rel, err := cleanRel(path)
	if err != nil {
		return err
	}
	r, err := l.openRoot()
	if err != nil {
		return err
	}
	defer r.Close()
	if dir := filepath.ToSlash(filepath.Dir(rel)); dir != "." && dir != "" {
		if err := r.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return r.WriteFile(rel, data, 0o644)
}

func (l *Local) Get(path string) ([]byte, error) {
	rel, err := cleanRel(path)
	if err != nil {
		return nil, err
	}
	r, err := l.openRoot()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.ReadFile(rel)
}

func (l *Local) Exists(path string) bool {
	rel, err := cleanRel(path)
	if err != nil {
		return false
	}
	r, err := l.openRoot()
	if err != nil {
		return false
	}
	defer r.Close()
	_, err = r.Stat(rel)
	return err == nil
}

func (l *Local) Delete(path string) error {
	rel, err := cleanRel(path)
	if err != nil {
		return err
	}
	r, err := l.openRoot()
	if err != nil {
		return err
	}
	defer r.Close()
	if err := r.Remove(rel); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Memory is an in-memory Disk driver ideal for testing and faking Storage().
type Memory struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func NewMemory() *Memory {
	return &Memory{files: make(map[string][]byte)}
}

func (m *Memory) Put(path string, data []byte) error {
	cp := append([]byte(nil), data...)
	m.mu.Lock()
	m.files[path] = cp
	m.mu.Unlock()
	return nil
}

func (m *Memory) Get(path string) ([]byte, error) {
	m.mu.RLock()
	data, ok := m.files[path]
	m.mu.RUnlock()
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (m *Memory) Exists(path string) bool {
	m.mu.RLock()
	_, ok := m.files[path]
	m.mu.RUnlock()
	return ok
}

func (m *Memory) Delete(path string) error {
	m.mu.Lock()
	delete(m.files, path)
	m.mu.Unlock()
	return nil
}
