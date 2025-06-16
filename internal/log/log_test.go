package log

import (
	"log/slog"
	"testing"
)

type Foo struct {
	Bar Baz `json:"bar"`
}

type Baz struct {
	Baz string `json:"baz"`
}

func TestAllLogLevels(t *testing.T) {
	var foo = Foo{Bar: Baz{Baz: "🎉"}} // Dummy nested json doc

	for _, format := range []string{"text", "json"} {
		logger, err := NewLoggerWithTrace(LevelTrace, true, format)
		if err != nil {
			t.Fatalf("creating logger: %v", err)
		}
		slog.SetDefault(logger)

		// Test all log levels
		slog.Error("This is an ERROR message", "foo", foo)
		slog.Warn("This is a WARN message", "foo", foo)
		slog.Info("This is an INFO message", "foo", foo)
		slog.Debug("This is a DEBUG message", "foo", foo)
		Trace(logger, "This is a TRACE message", "foo", foo)

	}
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
