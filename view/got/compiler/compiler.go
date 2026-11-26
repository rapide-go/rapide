package compiler

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// CompileDir scans rootDir recursively for *.got templates, resolves all <x-*>
// components under components/, and emits formatted *_got.go files into rootDir.
func CompileDir(rootDir string) (int, error) {
	info, err := os.Stat(rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("got: %s is not a directory", rootDir)
	}

	pkgName := sanitizePkgName(filepath.Base(rootDir))
	var templates []*Template
	registry := make(map[string]ComponentMeta)

	// Pass 1: discover all *.got files, parse frontmatter, register components.
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".got") {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		fm, body, err := parseFrontmatter(string(raw))
		if err != nil {
			return fmt.Errorf("%s: %w", relSlash, err)
		}
		nodes, err := parseBody(body)
		if err != nil {
			return fmt.Errorf("%s: %w", relSlash, err)
		}

		tpl := &Template{
			RelPath:     relSlash,
			Frontmatter: fm,
			Nodes:       nodes,
		}

		noExt := strings.TrimSuffix(relSlash, ".got")
		if after, ok := strings.CutPrefix(noExt, "components/"); ok {
			compRel := after
			tpl.IsComponent = true
			tpl.CompKey = strings.ToLower(strings.ReplaceAll(compRel, "/", "."))
			tpl.FuncName = toExportedIdent(compRel)
			tpl.PropsType = tpl.FuncName + "Props"

			byLower := make(map[string]PropDef, len(fm.Props))
			for _, pr := range fm.Props {
				key := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(pr.Name, "-", ""), "_", ""))
				byLower[key] = pr
			}
			registry[tpl.CompKey] = ComponentMeta{
				CompKey:      tpl.CompKey,
				FuncName:     tpl.FuncName,
				PropsType:    tpl.PropsType,
				PropsByLower: byLower,
			}
		} else {
			tpl.FuncName = toExportedIdent(noExt)
			tpl.PropsType = tpl.FuncName + "Props"
		}

		templates = append(templates, tpl)
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Pass 2: emit formatted *_got.go files into rootDir.
	for _, tpl := range templates {
		code, err := generateGo(tpl, pkgName, registry)
		if err != nil {
			return 0, err
		}
		outName := generatedFileName(tpl.RelPath)
		outPath := filepath.Join(rootDir, outName)
		if err := os.WriteFile(outPath, code, 0o644); err != nil {
			return 0, err
		}
	}

	return len(templates), nil
}

func generatedFileName(relSlash string) string {
	noExt := strings.TrimSuffix(relSlash, ".got")
	clean := strings.NewReplacer("/", "_", "-", "_", ".", "_").Replace(noExt)
	return strings.ToLower(clean) + "_got.go"
}

func toExportedIdent(pathNoExt string) string {
	parts := strings.FieldsFunc(pathNoExt, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.'
	})
	var sb strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		sb.WriteString(string(runes))
	}
	if sb.Len() == 0 {
		return "View"
	}
	return sb.String()
}

func sanitizePkgName(name string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	if sb.Len() == 0 {
		return "views"
	}
	return sb.String()
}
