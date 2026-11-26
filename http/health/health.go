package health

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/routing"
)

// Check probes a single dependency (database, cache, external API) for readiness.
type Check struct {
	Name string
	Fn   func(ctx context.Context) error
}

// DBCheck returns a readiness probe that pings a *sql.DB.
func DBCheck(name string, db *sql.DB) Check {
	if name == "" {
		name = "database"
	}
	return Check{
		Name: name,
		Fn: func(ctx context.Context) error {
			return db.PingContext(ctx)
		},
	}
}

// Handler returns a Rapide `/up`-style health/readiness handler.
// Responds with 200 OK when all checks pass, or 503 Service Unavailable if any check fails.
func Handler(checks ...Check) router.Handler {
	return func(c *ctx.Ctx) error {
		probeCtx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
		defer cancel()

		results := make(map[string]string, len(checks))
		healthy := true
		for _, chk := range checks {
			if err := chk.Fn(probeCtx); err != nil {
				healthy = false
				results[chk.Name] = "down: " + err.Error()
			} else {
				results[chk.Name] = "ok"
			}
		}

		status := http.StatusOK
		overall := "up"
		if !healthy {
			status = http.StatusServiceUnavailable
			overall = "degraded"
		}
		return c.Status(status).JSON(map[string]any{
			"status": overall,
			"checks": results,
		})
	}
}

// Mount registers the health endpoint at `/up` on router r.
func Mount(r *router.Router, checks ...Check) *router.RouteInfo {
	return r.Get("/up", Handler(checks...)).SetName("health.up")
}
