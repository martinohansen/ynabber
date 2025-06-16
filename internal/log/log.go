package log

import (
	"context"
	"fmt"
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
func NewLoggerWithTrace(minLevel slog.Level, addSource bool, format string) (*slog.Logger, error) {
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

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stderr, opts)
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}

	return slog.New(handler), nil
}

// Trace logs a message at trace level using the provided logger.
func Trace(logger *slog.Logger, msg string, args ...any) {
	logger.Log(context.Background(), LevelTrace, msg, args...)
}
