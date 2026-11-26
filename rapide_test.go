package rapide_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rapide-go/rapide"
	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

func TestNewWithGenericOptions(t *testing.T) {
	app, err := rapide.New(
		rapide.WithEnvFile(""),
		rapide.WithPort(9090),
		rapide.WithReadTimeout(15*time.Second),
		rapide.WithWriteTimeout(20*time.Second),
		rapide.WithIdleTimeout(60*time.Second),
		rapide.WithShutdownTimeout(5*time.Second),
		rapide.WithDebug(true),
		rapide.WithRoutes(func(r *router.Router) {
			r.Get("/ping", func(c *ctx.Ctx) error {
				return c.JSON(map[string]string{"pong": "ok"})
			})
		}),
	)
	if err != nil {
		t.Fatalf("rapide.New failed: %v", err)
	}

	if app.Addr != ":9090" {
		t.Fatalf("expected addr :9090, got %q", app.Addr)
	}
	if app.Server.ReadTimeout != 15*time.Second || app.Server.WriteTimeout != 20*time.Second || app.Server.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected server timeouts: %+v", app.Server)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestWithPortAndWithAddr(t *testing.T) {
	a1, err := rapide.New(rapide.WithEnvFile(""), rapide.WithPort(8081))
	if err != nil || a1.Addr != ":8081" {
		t.Fatalf("expected :8081, got %q (err=%v)", a1.Addr, err)
	}

	a2, err := rapide.New(rapide.WithEnvFile(""), rapide.WithAddr("127.0.0.1:8082"))
	if err != nil || a2.Addr != "127.0.0.1:8082" {
		t.Fatalf("expected 127.0.0.1:8082, got %q (err=%v)", a2.Addr, err)
	}
}
