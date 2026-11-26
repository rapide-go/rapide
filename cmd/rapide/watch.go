package main

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

type fileSig struct {
	modTime int64
	size    int64
}

var (
	watchExts = map[string]bool{
		".go":   true,
		".got":  true,
		".html": true,
		".tmpl": true,
		".env":  true,
	}
	ignoreDirs = map[string]bool{
		".git":         true,
		"node_modules": true,
		"storage":      true,
		"bin":          true,
		"tmp":          true,
		"vendor":       true,
	}
)

// watchDir polls root every interval and emits the first changed path on changes
// after debouncing rapid consecutive saves (zero third-party dependencies).
func watchDir(ctx context.Context, root string, interval, debounce time.Duration) <-chan string {
	changes := make(chan string, 1)

	go func() {
		defer close(changes)
		prev := snapshot(root)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var (
			pendingPath string
			debounceCh  <-chan time.Time
		)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				curr := snapshot(root)
				if changed := diffSnapshots(prev, curr); changed != "" {
					prev = curr
					pendingPath = changed
					debounceCh = time.After(debounce)
				}
			case <-debounceCh:
				debounceCh = nil
				if pendingPath != "" {
					select {
					case changes <- pendingPath:
					default:
					}
					pendingPath = ""
				}
			}
		}
	}()

	return changes
}

func snapshot(root string) map[string]fileSig {
	out := make(map[string]fileSig)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if strings.HasSuffix(strings.ToLower(d.Name()), "_got.go") {
			return nil
		}
		if !watchExts[ext] && d.Name() != ".env" {
			return nil
		}
		if info, err := d.Info(); err == nil {
			out[path] = fileSig{modTime: info.ModTime().UnixNano(), size: info.Size()}
		}
		return nil
	})
	return out
}

func diffSnapshots(prev, curr map[string]fileSig) string {
	for path, sig := range curr {
		if old, ok := prev[path]; !ok || old != sig {
			return path
		}
	}
	for path := range prev {
		if _, ok := curr[path]; !ok {
			return path
		}
	}
	return ""
}
