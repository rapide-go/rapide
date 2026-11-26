package container_test

import (
	"testing"

	"github.com/rapide-go/rapide/container"
)

type dbStub struct{ name string }
type svcStub struct{ db *dbStub }

func TestNestedSingletonAndExtend(t *testing.T) {
	c := container.New()

	container.Singleton[*dbStub](c, func(*container.Container) (*dbStub, error) {
		return &dbStub{name: "sqlite"}, nil
	})
	container.Singleton[*svcStub](c, func(ct *container.Container) (*svcStub, error) {
		// Nested singleton resolution must not deadlock.
		db := container.Resolve[*dbStub](ct)
		return &svcStub{db: db}, nil
	})

	if err := container.Extend[*dbStub](c, func(d *dbStub) *dbStub {
		d.name += "+traced"
		return d
	}); err != nil {
		t.Fatalf("Extend failed: %v", err)
	}

	svc := container.Resolve[*svcStub](c)
	if svc.db.name != "sqlite+traced" {
		t.Fatalf("expected sqlite+traced, got %q", svc.db.name)
	}

	restore := container.Fake[*dbStub](c, &dbStub{name: "mock"})
	if got := container.Resolve[*dbStub](c).name; got != "mock" {
		t.Fatalf("expected mock, got %q", got)
	}
	restore()
	if got := container.Resolve[*dbStub](c).name; got != "sqlite+traced" {
		t.Fatalf("expected restored sqlite+traced, got %q", got)
	}
}
