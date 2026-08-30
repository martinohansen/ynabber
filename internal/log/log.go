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

// SecretString prevents a string from being written by a slog handler.
type SecretString string

func (s SecretString) LogValue() slog.Value {
	return slog.StringValue("REDACTED")
}

// MaskedBankIdentifier retains enough of an IBAN, BBAN, or CPAN to correlate
// log entries without recording the complete bank identifier.
type MaskedBankIdentifier string

func (id MaskedBankIdentifier) LogValue() slog.Value {
	return slog.StringValue(id.String())
}

func (id MaskedBankIdentifier) String() string {
	runes := []rune(id)
	if len(runes) <= 8 {
		return "****"
	}
	return string(runes[:4]) + "..." + string(runes[len(runes)-4:])
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
