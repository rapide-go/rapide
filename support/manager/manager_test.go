package manager_test

import (
	"testing"

	"github.com/rapide-go/rapide/support/manager"
)

func TestGenericManagerAndOptions(t *testing.T) {
	calls := 0
	m := manager.New[string](
		manager.WithFactory[string]("s3", func() (string, error) {
			calls++
			return "s3-client", nil
		}),
		manager.WithInstance[string]("local", "local-disk"),
		manager.WithDefault[string]("local"),
	)

	if got := m.Default(); got != "local-disk" {
		t.Fatalf("expected local-disk, got %q", got)
	}
	if calls != 0 {
		t.Fatalf("s3 factory should be lazy, called %d times", calls)
	}

	if got := m.Must("s3"); got != "s3-client" {
		t.Fatalf("expected s3-client, got %q", got)
	}
	_ = m.Must("s3")
	if calls != 1 {
		t.Fatalf("expected s3 factory called once, got %d", calls)
	}

	restore := m.Fake("s3", "mock-s3")
	if got := m.Must("s3"); got != "mock-s3" {
		t.Fatalf("expected mock-s3, got %q", got)
	}
	restore()
	if got := m.Must("s3"); got != "s3-client" {
		t.Fatalf("expected restored s3-client, got %q", got)
	}
}
