package session

import (
	"maps"
	"sync"
	"time"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/support/ctxkey"
)

var sessionKey = ctxkey.New[*Session]()

// Session stores per-user state and one-time flash messages for web/HTMX/Inertia routes.
type Session struct {
	ID          string
	mu          sync.RWMutex
	data        map[string]any
	expiresAt   time.Time
	rotatedFrom string
	onRotate    func(newID string)
}

// Regenerate rotates the session ID and CSRF token to prevent session fixation attacks
// (called automatically by auth.Login).
func (s *Session) Regenerate() {
	s.mu.Lock()
	if s.rotatedFrom == "" {
		s.rotatedFrom = s.ID
	}
	s.ID = randomHex(32)
	s.data["_csrf"] = randomHex(24)
	newID := s.ID
	cb := s.onRotate
	s.mu.Unlock()
	if cb != nil {
		cb(newID)
	}
}

// Invalidate clears all session data and rotates the session ID (called automatically by auth.Logout).
func (s *Session) Invalidate() {
	s.mu.Lock()
	if s.rotatedFrom == "" {
		s.rotatedFrom = s.ID
	}
	s.ID = randomHex(32)
	s.data = make(map[string]any)
	newID := s.ID
	cb := s.onRotate
	s.mu.Unlock()
	if cb != nil {
		cb(newID)
	}
}

func (s *Session) takeRotation() (oldID, newID string, rotated bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rotatedFrom == "" {
		return "", s.ID, false
	}
	old := s.rotatedFrom
	s.rotatedFrom = ""
	return old, s.ID, true
}

func newSession(id string, ttl time.Duration) *Session {
	return &Session{
		ID:        id,
		data:      make(map[string]any),
		expiresAt: time.Now().Add(ttl),
	}
}

func (s *Session) touch(ttl time.Duration) {
	s.mu.Lock()
	s.expiresAt = time.Now().Add(ttl)
	s.mu.Unlock()
}

func (s *Session) snapshot() (map[string]any, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]any, len(s.data))
	maps.Copy(cp, s.data)
	return cp, s.expiresAt
}

func (s *Session) Set(key string, val any) {
	s.mu.Lock()
	s.data[key] = val
	s.mu.Unlock()
}

func (s *Session) Get(key string) (any, bool) {
	s.mu.RLock()
	v, ok := s.data[key]
	s.mu.RUnlock()
	return v, ok
}

// Flash stores a value that is automatically deleted after being read once via Pull.
func (s *Session) Flash(key string, val any) {
	s.Set("_flash."+key, val)
}

// Pull retrieves and deletes a flashed value.
func (s *Session) Pull(key string) (any, bool) {
	full := "_flash." + key
	s.mu.Lock()
	v, ok := s.data[full]
	delete(s.data, full)
	s.mu.Unlock()
	return v, ok
}

// CSRFToken returns (or generates) a per-session CSRF token.
func (s *Session) CSRFToken() string {
	if v, ok := s.Get("_csrf"); ok {
		return v.(string)
	}
	tok := randomHex(24)
	s.Set("_csrf", tok)
	return tok
}

// Current retrieves the active *Session from the request context.
func Current(c *ctx.Ctx) *Session {
	if c == nil || c.Request == nil {
		return nil
	}
	sess, _ := sessionKey.Get(c.Request.Context())
	return sess
}
