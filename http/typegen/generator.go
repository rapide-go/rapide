package typegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	router "github.com/rapide-go/rapide/routing"
	"github.com/rapide-go/rapide/support/option"
)

// Config controls TypeGen output directory and codegen emitter.
type Config struct {
	OutDir  string
	Emitter Emitter
}

// WithOutputDir sets the target directory for generated files (default `resources/js/rapide`).
func WithOutputDir(dir string) option.Option[Config] {
	return func(c *Config) {
		if dir != "" {
			c.OutDir = dir
		}
	}
}

// WithEmitter plugs in a custom Emitter (defaults to NewTSEmitter()).
func WithEmitter(e Emitter) option.Option[Config] {
	return func(c *Config) {
		if e != nil {
			c.Emitter = e
		}
	}
}

// Generator orchestrates the 3-stage TypeGen pipeline: Inspect -> Emit -> Atomic Write.
type Generator struct {
	cfg Config
}

// New creates a Generator configured with Rapide's generic option.Option[Config].
func New(opts ...option.Option[Config]) *Generator {
	cfg := Config{
		OutDir:  filepath.Join("resources", "js", "rapide"),
		Emitter: NewTSEmitter(),
	}
	option.Apply(&cfg, opts...)
	return &Generator{cfg: cfg}
}

// Run inspects routes, emits all target files, and writes them atomically (refusing symlinks).
func (g *Generator) Run(r *router.Router) error {
	if strings.ContainsRune(g.cfg.OutDir, '\x00') {
		return fmt.Errorf("typegen: invalid output directory")
	}
	outDir := filepath.Clean(g.cfg.OutDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("typegen: mkdir %s: %w", outDir, err)
	}

	schema := Inspect(r.Routes())
	files := g.cfg.Emitter.Emit(schema)
	for name, content := range files {
		if strings.ContainsAny(name, `/\`) || name == "" || name == "." || name == ".." {
			return fmt.Errorf("typegen: unsafe output filename %q", name)
		}
		target := filepath.Join(outDir, name)
		if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("typegen: refusing to overwrite symlink %s", target)
		}
		if existing, err := os.ReadFile(target); err == nil && string(existing) == content {
			continue
		}
		if err := writeFileAtomic(target, []byte(content)); err != nil {
			return fmt.Errorf("typegen: write %s: %w", target, err)
		}
	}
	return nil
}

// Generate is the default convenience entrypoint writing TypeScript bindings into outDir.
func Generate(r *router.Router, outDir string) error {
	return New(WithOutputDir(outDir)).Run(r)
}

// GenerateTypes renders TypeScript interface definitions for routes.
func GenerateTypes(routes []router.RouteInfo) string {
	return NewTSEmitter().EmitTypes(Inspect(routes))
}

// GenerateRoutes renders TypeScript route helpers for routes.
func GenerateRoutes(routes []router.RouteInfo) string {
	return NewTSEmitter().EmitRoutes(Inspect(routes))
}

func writeFileAtomic(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".typegen-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
