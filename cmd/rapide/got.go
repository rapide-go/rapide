package main

import (
	"path/filepath"

	gotcompiler "github.com/rapide-go/rapide/view/got/compiler"
)

// compileGot compiles all *.got templates in resources/views (or root if custom).
func compileGot(root string) (int, error) {
	viewsDir := filepath.Join(root, "resources", "views")
	return gotcompiler.CompileDir(viewsDir)
}
