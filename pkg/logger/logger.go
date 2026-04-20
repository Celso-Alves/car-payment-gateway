// Package logger wraps the standard library's slog to provide a
// structured, levelled logger that can be injected across layers.
// Using slog (Go 1.21+) avoids third-party dependencies while still
// producing machine-readable JSON output suitable for log aggregators.
package logger

import (
	"log/slog"
	"os"
)

// New creates a JSON-structured logger writing to stdout.
// The level defaults to Info; set LOG_LEVEL=DEBUG in the environment
// to enable debug messages.
func New() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "DEBUG" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
