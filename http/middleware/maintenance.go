package middleware

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"sync"

	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

const maintenanceBypassCookie = "rapide_maintenance_bypass"

// MaintenanceMode controls fluent `down` / `up` maintenance mode with secret bypass.
type MaintenanceMode struct {
	mu         sync.RWMutex
	down       bool
	secret     string
	retryAfter int
	message    string
}

// NewMaintenanceMode creates a MaintenanceMode controller (initially up).
func NewMaintenanceMode() *MaintenanceMode {
	return &MaintenanceMode{message: "Service Unavailable"}
}

// Down puts the application into maintenance mode (503) with optional bypass secret and Retry-After seconds.
func (m *MaintenanceMode) Down(secret string, retryAfterSeconds int) {
	m.mu.Lock()
	m.down = true
	m.secret = secret
	m.retryAfter = retryAfterSeconds
	m.mu.Unlock()
}

// Up brings the application back online.
func (m *MaintenanceMode) Up() {
	m.mu.Lock()
	m.down = false
	m.secret = ""
	m.retryAfter = 0
	m.mu.Unlock()
}

// IsDown reports whether maintenance mode is currently active.
func (m *MaintenanceMode) IsDown() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.down
}

// Middleware intercepts requests with 503 Service Unavailable while Down(),
// allowing access via `X-Maintenance-Bypass: <secret>`, `?<secret>` / `?secret=<secret>`, or bypass cookie.
func (m *MaintenanceMode) Middleware() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *ctx.Ctx) error {
			m.mu.RLock()
			down := m.down
			secret := m.secret
			retry := m.retryAfter
			msg := m.message
			m.mu.RUnlock()

			if !down {
				return next(c)
			}

			if secret != "" {
				// Allow `?secret=<secret>` to set a bypass cookie and redirect cleanly
				if q := c.Query("secret"); q != "" && subtle.ConstantTimeCompare([]byte(q), []byte(secret)) == 1 {
					http.SetCookie(c.Writer, &http.Cookie{
						Name:     maintenanceBypassCookie,
						Value:    secret,
						Path:     "/",
						HttpOnly: true,
						SameSite: http.SameSiteLaxMode,
					})
					return next(c)
				}
				if hdr := c.Header("X-Maintenance-Bypass"); hdr != "" && subtle.ConstantTimeCompare([]byte(hdr), []byte(secret)) == 1 {
					return next(c)
				}
				if ck, err := c.Request.Cookie(maintenanceBypassCookie); err == nil && subtle.ConstantTimeCompare([]byte(ck.Value), []byte(secret)) == 1 {
					return next(c)
				}
			}

			if retry > 0 {
				c.SetHeader("Retry-After", strconv.Itoa(retry))
			}
			return c.Status(http.StatusServiceUnavailable).JSON(map[string]any{
				"message": msg,
			})
		}
	}
}
