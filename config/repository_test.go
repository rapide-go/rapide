package config_test

import (
	"testing"

	"github.com/rapide-go/rapide/config"
)

type sampleCfg struct {
	Name  string `env:"TEST_APP_NAME" default:"Rapide"`
	Port  int    `env:"TEST_APP_PORT" default:"9090"`
	Debug bool   `env:"TEST_APP_DEBUG" default:"true"`
}

func TestRepositoryGetAndBindStruct(t *testing.T) {
	r := config.NewRepository()
	cfg, err := config.BindStruct[sampleCfg](r, "app")
	if err != nil {
		t.Fatalf("BindStruct failed: %v", err)
	}
	if cfg.Port != 9090 || cfg.Name != "Rapide" || !cfg.Debug {
		t.Fatalf("unexpected bound struct: %+v", cfg)
	}

	// Type-inferred fallback + optional fallback
	if got := config.GetOn(r, "app.port", 8080); got != 9090 {
		t.Fatalf("expected 9090, got %d", got)
	}
	if got := config.GetOn(r, "app.missing", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
	if got := config.GetOn[int](r, "app.missing_int"); got != 0 {
		t.Fatalf("expected zero value 0, got %d", got)
	}

	r.Set("app.port", "3000")
	if got := config.GetOn(r, "app.port", 8080); got != 3000 {
		t.Fatalf("expected coerced int 3000, got %d", got)
	}
}
