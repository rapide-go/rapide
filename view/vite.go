package view

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"os"
	"strings"
	"sync"
)

// ViteConfig configures the `{{vite "resources/js/app.ts"}}` template helper
// (provides a clean, idiomatic Go API...])` directive).
type ViteConfig struct {
	HotFile      string // defaults to "public/hot"
	ManifestPath string // defaults to "public/build/manifest.json"
	ManifestFS   fs.FS  // optional embed.FS for single-binary production deploys
	BuildPrefix  string // defaults to "/build"
	Watch        bool   // if false, caches parsed manifest in memory
}

type viteManifestEntry struct {
	File string   `json:"file"`
	CSS  []string `json:"css"`
}

var (
	viteMu       sync.RWMutex
	viteCfg      = ViteConfig{HotFile: "public/hot", ManifestPath: "public/build/manifest.json", BuildPrefix: "/build", Watch: true}
	viteManifest map[string]viteManifestEntry
)

// ConfigureVite sets global Vite paths/options used by `{{vite...}}` in Blade views.
func ConfigureVite(cfg ViteConfig) {
	if cfg.HotFile == "" {
		cfg.HotFile = "public/hot"
	}
	if cfg.ManifestPath == "" {
		cfg.ManifestPath = "public/build/manifest.json"
	}
	if cfg.BuildPrefix == "" {
		cfg.BuildPrefix = "/build"
	}
	viteMu.Lock()
	viteCfg = cfg
	viteManifest = nil
	viteMu.Unlock()
}

// ViteTags renders Vite dev HMR tags (when HotFile exists) or production manifest tags.
func ViteTags(entries ...string) template.HTML {
	viteMu.RLock()
	cfg := viteCfg
	cached := viteManifest
	viteMu.RUnlock()

	if cfg.HotFile != "" {
		if raw, err := os.ReadFile(cfg.HotFile); err == nil {
			if devURL := strings.TrimRight(strings.TrimSpace(string(raw)), "/"); devURL != "" {
				var sb strings.Builder
				fmt.Fprintf(&sb, `<script type="module" src="%s/@vite/client"></script>`+"\n", html.EscapeString(devURL))
				for _, entry := range entries {
					e := strings.TrimLeft(entry, "/")
					fmt.Fprintf(&sb, `<script type="module" src="%s/%s"></script>`+"\n", html.EscapeString(devURL), html.EscapeString(e))
				}
				return template.HTML(sb.String())
			}
		}
	}

	manifest := cached
	if manifest == nil || cfg.Watch {
		loaded, err := readViteManifest(cfg)
		if err != nil {
			return ""
		}
		manifest = loaded
		if !cfg.Watch {
			viteMu.Lock()
			viteManifest = loaded
			viteMu.Unlock()
		}
	}

	prefix := strings.TrimRight(cfg.BuildPrefix, "/")
	var sb strings.Builder
	for _, entry := range entries {
		item, ok := manifest[entry]
		if !ok {
			continue
		}
		for _, css := range item.CSS {
			fmt.Fprintf(&sb, `<link rel="stylesheet" href="%s/%s" />`+"\n", prefix, html.EscapeString(css))
		}
		if item.File != "" {
			if strings.HasSuffix(item.File, ".css") {
				fmt.Fprintf(&sb, `<link rel="stylesheet" href="%s/%s" />`+"\n", prefix, html.EscapeString(item.File))
			} else {
				fmt.Fprintf(&sb, `<script type="module" src="%s/%s"></script>`+"\n", prefix, html.EscapeString(item.File))
			}
		}
	}
	return template.HTML(sb.String())
}

func readViteManifest(cfg ViteConfig) (map[string]viteManifestEntry, error) {
	var (
		raw []byte
		err error
	)
	if cfg.ManifestFS != nil {
		raw, err = fs.ReadFile(cfg.ManifestFS, cfg.ManifestPath)
	} else {
		raw, err = os.ReadFile(cfg.ManifestPath)
	}
	if err != nil {
		return nil, err
	}
	var m map[string]viteManifestEntry
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}
