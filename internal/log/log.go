package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	// LevelTrace for trace logging like request and responses from external
	// APIs.
	LevelTrace = slog.Level(-8)

	// LevelFatal for errors that should print and exit with a non-zero code.
	LevelFatal = slog.Level(16)
)

func ParseLevel(s string) (slog.Level, error) {
	// Handle custom levels
	if strings.ToLower(s) == "trace" {
		return LevelTrace, nil
	}
	if strings.ToLower(s) == "fatal" {
		return LevelFatal, nil
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
				case level == LevelFatal:
					a.Value = slog.StringValue("FATAL")
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

// Fatal logs a message at fatal level and exits
func Fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Log(context.Background(), LevelFatal, msg, args...)
	os.Exit(1)
}
