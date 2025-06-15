package log

import (
	"log/slog"
	"testing"
)

func TestAllLogLevels(t *testing.T) {
	// Create a logger with trace level to show all messages
	logger := NewLoggerWithTrace(LevelTrace, true)
	slog.SetDefault(logger)

	// Test all log levels
	slog.Error("This is an ERROR message")
	slog.Warn("This is a WARN message")
	slog.Info("This is an INFO message")
	slog.Debug("This is a DEBUG message")
	Trace(logger, "This is a TRACE message")

	t.Log("All log levels have been printed above")
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
		hasError bool
	}{
		{"trace", LevelTrace, false},
		{"invalid", slog.LevelInfo, true},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			level, err := ParseLevel(test.input)

			if test.hasError {
				if err == nil {
					t.Errorf("Expected error for input %s, but got none", test.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %s: %v", test.input, err)
				}
				if level != test.expected {
					t.Errorf("Expected level %v for input %s, got %v", test.expected, test.input, level)
				}
			}
		})
	}
}
