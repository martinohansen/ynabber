package log

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// LevelTrace is a custom log level for trace logging like request and responses
// from external APIs.
const LevelTrace = slog.Level(-8)

func ParseLevel(s string) (slog.Level, error) {
	// Handle our custom trace level first
	if strings.ToLower(s) == "trace" {
		return LevelTrace, nil
	}

	// Use slog's built-in parsing for standard levels
	var level slog.Level
	var err = level.UnmarshalText([]byte(s))
	return level, err
}

// NewLoggerWithTrace creates a logger with trace support
func NewLoggerWithTrace(minLevel slog.Level, addSource bool) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     minLevel,
		AddSource: addSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize level names
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				switch {
				case level == LevelTrace:
					a.Value = slog.StringValue("TRACE")
				}
			}
			return a
		},
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	return slog.New(handler)
}

// Trace logs a message at trace level using the provided logger.
func Trace(logger *slog.Logger, msg string, args ...any) {
	logger.Log(context.Background(), LevelTrace, msg, args...)
}
