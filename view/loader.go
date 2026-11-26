package view

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// parseTree recursively walks fsys (starting from rootDir, e.g. "views" or ".")
// and parses every .html / .tmpl file under its relative slash path (e.g. "auth/login.html"),
// avoiding Go's default filepath.Base collision across subdirectories.
func parseTree(fsys fs.FS, rootDir string, funcs template.FuncMap) (*template.Template, error) {
	if rootDir == "" {
		rootDir = "."
	}
	root := template.New("").Funcs(funcs)
	schemas := make(map[string]*PropSchema)
	attachComponentHelper(root, schemas)

	err := fs.WalkDir(fsys, rootDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(path.Ext(p))
		if ext != ".html" && ext != ".tmpl" {
			return nil
		}
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}

		// Register under path relative to rootDir (e.g. "views/auth/login.html" -> "auth/login.html").
		rel := strings.TrimPrefix(p, strings.TrimSuffix(rootDir, "/")+"/")
		if rootDir == "." {
			rel = p
		}
		if ps := parsePropSchema(string(raw)); ps != nil {
			schemas[rel] = ps
		}

		compiled, err := compileBladeComponents(string(raw))
		if err != nil {
			return fmt.Errorf("view: compile %s: %w", p, err)
		}
		if _, err := root.New(rel).Parse(compiled); err != nil {
			return fmt.Errorf("view: parse %s: %w", rel, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return root, nil
}

// loadFromConfig loads templates from an embedded fs.FS or local disk directory.
// Supports both legacy glob patterns ("views/*.html") and recursive directory roots ("views").
func (e *Engine) loadFromConfig() (*template.Template, error) {
	funcs := e.mergedFuncs()

	if e.cfg.FS != nil {
		if strings.ContainsAny(e.cfg.Pattern, "*?[") {
			// Extract base directory (e.g. "views/*.html" -> "views") for recursive loading.
			dir := path.Dir(e.cfg.Pattern)
			if dir != "" && dir != "." {
				if _, err := fs.Stat(e.cfg.FS, dir); err == nil {
					return parseTree(e.cfg.FS, dir, funcs)
				}
			}
			return template.New("").Funcs(funcs).ParseFS(e.cfg.FS, e.cfg.Pattern)
		}
		return parseTree(e.cfg.FS, e.cfg.Pattern, funcs)
	}

	if e.cfg.Dir != "" {
		if strings.ContainsAny(e.cfg.Dir, "*?[") {
			return template.New("").Funcs(funcs).ParseGlob(e.cfg.Dir)
		}
		return parseTree(os.DirFS(filepath.Clean(e.cfg.Dir)), ".", funcs)
	}

	return template.New("").Funcs(funcs), nil
}
