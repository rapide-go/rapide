package view_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rapide-go/rapide/view"
)

func TestViteTagsDevAndProduction(t *testing.T) {
	// 1. Production manifest mode via embed/MapFS
	manifestFS := fstest.MapFS{
		"manifest.json": &fstest.MapFile{
			Data: []byte(`{"resources/js/app.ts":{"file":"assets/app-123.js","css":["assets/app-456.css"]}}`),
		},
	}
	view.ConfigureVite(view.ViteConfig{
		HotFile:      filepath.Join(t.TempDir(), "nonexistent-hot"),
		ManifestFS:   manifestFS,
		ManifestPath: "manifest.json",
		BuildPrefix:  "/build",
		Watch:        false,
	})

	prod := string(view.ViteTags("resources/js/app.ts"))
	if !strings.Contains(prod, `<link rel="stylesheet" href="/build/assets/app-456.css" />`) ||
		!strings.Contains(prod, `<script type="module" src="/build/assets/app-123.js"></script>`) {
		t.Fatalf("unexpected prod vite tags:\n%s", prod)
	}

	// 2. Dev HMR mode via hot file
	hotPath := filepath.Join(t.TempDir(), "hot")
	if err := os.WriteFile(hotPath, []byte("http://localhost:5173\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	view.ConfigureVite(view.ViteConfig{HotFile: hotPath})

	dev := string(view.ViteTags("resources/js/app.ts"))
	if !strings.Contains(dev, `http://localhost:5173/@vite/client`) ||
		!strings.Contains(dev, `http://localhost:5173/resources/js/app.ts`) {
		t.Fatalf("unexpected dev vite tags:\n%s", dev)
	}
}
