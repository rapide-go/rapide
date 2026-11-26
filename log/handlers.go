package log

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DailyWriter writes logs to date-stamped files (e.g. storage/logs/rapide-2026-11-26.log)
// and rotates automatically at midnight UTC.
type DailyWriter struct {
	mu      sync.Mutex
	dir     string
	base    string
	ext     string
	curDate string
	file    *os.File
}

// NewDailyWriter creates a daily-rotating log writer derived from basePath.
func NewDailyWriter(basePath string) (*DailyWriter, error) {
	dir := filepath.Dir(basePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ext := filepath.Ext(basePath)
	if ext == "" {
		ext = ".log"
	}
	base := strings.TrimSuffix(filepath.Base(basePath), ext)
	dw := &DailyWriter{dir: dir, base: base, ext: ext}
	if err := dw.rotateLocked(time.Now().UTC().Format("2006-01-02")); err != nil {
		return nil, err
	}
	return dw, nil
}

func (d *DailyWriter) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	today := time.Now().UTC().Format("2006-01-02")
	if today != d.curDate || d.file == nil {
		if err := d.rotateLocked(today); err != nil {
			return 0, err
		}
	}
	return d.file.Write(p)
}

func (d *DailyWriter) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file == nil {
		return nil
	}
	err := d.file.Close()
	d.file = nil
	return err
}

func (d *DailyWriter) rotateLocked(date string) error {
	if d.file != nil {
		_ = d.file.Close()
	}
	name := fmt.Sprintf("%s-%s%s", d.base, date, d.ext)
	f, err := os.OpenFile(filepath.Join(d.dir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	d.curDate = date
	d.file = f
	return nil
}

// MultiHandler fans out log records to multiple slog.Handlers.
type MultiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: next}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: next}
}
