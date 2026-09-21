package log

import (
	"bytes"
	"log/slog"
	"strings"
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

func TestSecretStringLogValue(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			var handler slog.Handler
			opts := &slog.HandlerOptions{Level: LevelTrace}
			if format == "json" {
				handler = slog.NewJSONHandler(&output, opts)
			} else {
				handler = slog.NewTextHandler(&output, opts)
			}
			logger := slog.New(handler)
			Trace(logger, "secret", "value", SecretString("private-secret"))

			got := output.String()
			if strings.Contains(got, "private-secret") {
				t.Fatalf("log contains private value: %s", got)
			}
			if !strings.Contains(got, "REDACTED") {
				t.Fatalf("log omits redaction marker: %s", got)
			}
		})
	}
}

func TestMaskedBankIdentifierLogValue(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			var handler slog.Handler
			if format == "json" {
				handler = slog.NewJSONHandler(&output, nil)
			} else {
				handler = slog.NewTextHandler(&output, nil)
			}
			logger := slog.New(handler)
			logger.Info("account", "id", MaskedBankIdentifier("DK5000400440116243"))

			got := output.String()
			if strings.Contains(got, "DK5000400440116243") {
				t.Fatalf("log contains complete identifier: %s", got)
			}
			if !strings.Contains(got, "DK50...6243") {
				t.Fatalf("log omits masked identifier: %s", got)
			}
		})
	}
}
