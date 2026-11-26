package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/foundation"
	"github.com/rapide-go/rapide/support/option"
)

// Functional options for log.Config using Rapide's generic option.Option[Config].
func WithChannel(ch string) option.Option[Config] { return func(c *Config) { c.Channel = ch } }
func WithLevel(lvl string) option.Option[Config]  { return func(c *Config) { c.Level = lvl } }
func WithPath(path string) option.Option[Config]  { return func(c *Config) { c.Path = path } }
func WithSource(enabled bool) option.Option[Config] {
	return func(c *Config) { c.AddSource = enabled }
}

// New constructs a configured *slog.Logger, sets it as slog's global default,
// and returns an optional closer for any opened log files.
func New(cfg Config, opts ...option.Option[Config]) (*slog.Logger, io.Closer, error) {
	option.Apply(&cfg, opts...)
	handlerOpts := &slog.HandlerOptions{
		Level:     ParseLevel(cfg.Level),
		AddSource: cfg.AddSource,
	}

	var (
		handler slog.Handler
		closer  io.Closer
	)

	switch strings.ToLower(strings.TrimSpace(cfg.Channel)) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	case "file", "single":
		f, err := openLogFile(cfg.Path)
		if err != nil {
			return nil, nil, err
		}
		closer = f
		handler = slog.NewJSONHandler(f, handlerOpts)
	case "daily":
		dw, err := NewDailyWriter(cfg.Path)
		if err != nil {
			return nil, nil, err
		}
		closer = dw
		handler = slog.NewJSONHandler(dw, handlerOpts)
	case "stack":
		dw, err := NewDailyWriter(cfg.Path)
		if err != nil {
			return nil, nil, err
		}
		closer = dw
		handler = NewMultiHandler(
			slog.NewTextHandler(os.Stderr, handlerOpts),
			slog.NewJSONHandler(dw, handlerOpts),
		)
	default: // "text"
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, closer, nil
}

func openLogFile(path string) (*os.File, error) {
	if path == "" {
		path = "storage/logs/rapide.log"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// Log is the Rapide facade accessor returning the active *slog.Logger.
func Log() *slog.Logger {
	if l, err := container.TryUse[*slog.Logger](); err == nil && l != nil {
		return l
	}
	return slog.Default()
}

// Convenience fluent facade helpers.
func Debug(msg string, args ...any) { Log().Debug(msg, args...) }
func Info(msg string, args ...any)  { Log().Info(msg, args...) }
func Warn(msg string, args ...any)  { Log().Warn(msg, args...) }
func Error(msg string, args ...any) { Log().Error(msg, args...) }
func With(args ...any) *slog.Logger { return Log().With(args...) }

// Provider configures slog from Config (or environment variables), binds *slog.Logger
// into the container, and closes open log files on graceful shutdown.
type Provider struct {
	foundation.BaseProvider
	Config *Config
}

func (p Provider) Register(app *foundation.App) error {
	var cfg Config
	if p.Config != nil {
		cfg = *p.Config
	} else {
		loaded, err := config.Load[Config]()
		if err != nil {
			return err
		}
		cfg = loaded
	}

	logger, closer, err := New(cfg)
	if err != nil {
		return err
	}
	container.Bind[*slog.Logger](app.Container, logger)
	if closer != nil {
		app.OnShutdown(func(ctx context.Context) error {
			return closer.Close()
		})
	}
	return nil
}
