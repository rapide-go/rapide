package log

import (
	"log/slog"
	"strings"
)

// Config configures Rapide's slog-backed logging channels (loadable via config.Load[log.Config]()).
type Config struct {
	// Channel selects output destination: "text" (dev console), "json" (prod stderr),
	// "file" (single log file), "daily" (date-rotated files), or "stack" (console + daily file).
	Channel string `env:"LOG_CHANNEL" default:"text"`
	// Level sets minimum severity: "debug", "info", "warn", or "error".
	Level string `env:"LOG_LEVEL" default:"info"`
	// Path sets the file or directory path for file/daily/stack channels.
	Path string `env:"LOG_PATH" default:"storage/logs/rapide.log"`
	// AddSource includes file:line caller info on each log entry.
	AddSource bool `env:"LOG_SOURCE" default:"false"`
}

// ParseLevel converts a string level name into a slog.Level.
func ParseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
