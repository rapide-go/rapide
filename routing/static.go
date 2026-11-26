package router

import (
	"io/fs"
	"net/http"
	"os"
	"strings"
)

// Static serves files from any fs.FS (including //go:embed embed.FS) under urlPrefix
// with optional Cache-Control header (e.g. "public, max-age=31536000, immutable" for Vite builds).
func (r *Router) Static(urlPrefix string, fsys fs.FS, cacheControl ...string) {
	prefix := strings.TrimRight(joinPath(r.prefix, urlPrefix), "/")
	if prefix == "" {
		prefix = "/"
	}

	fileServer := http.StripPrefix(prefix, http.FileServerFS(fsys))
	cc := ""
	if len(cacheControl) > 0 {
		cc = cacheControl[0]
	}

	pattern := "GET " + prefix + "/{filepath...}"
	if prefix == "/" {
		pattern = "GET /{filepath...}"
	}

	r.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		if cc != "" {
			w.Header().Set("Cache-Control", cc)
		}
		fileServer.ServeHTTP(w, req)
	})
}

// StaticDir serves static files from a local directory on disk (e.g. "public").
func (r *Router) StaticDir(urlPrefix, dir string, cacheControl ...string) {
	r.Static(urlPrefix, os.DirFS(dir), cacheControl...)
}
