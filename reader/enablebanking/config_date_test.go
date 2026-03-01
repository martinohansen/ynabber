package enablebanking

import (
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
)

// mustDate is a test helper that decodes a YYYY-MM-DD string into ynabber.Date
// or fails the test immediately if the string is malformed.
func mustDate(t *testing.T, s string) ynabber.Date {
	t.Helper()
	var d ynabber.Date
	if err := d.Decode(s); err != nil {
		t.Fatalf("mustDate(%q): %v", s, err)
	}
	return d
}

// validBaseConfig returns a Config with all required fields set using typed
// ynabber.Date values. Individual test cases override specific fields.
func validBaseConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		AppID:       "test-app",
		Country:     "NO",
		ASPSP:       "DNB",
		RedirectURL: "https://example.com",
		PEMFile:     "test.pem",
		FromDate:    mustDate(t, "2024-01-01"), // typed ynabber.Date, not string
	}
}

// ---------------------------------------------------------------------------
// ynabber.Date.Decode
// ---------------------------------------------------------------------------

// TestDateDecode_Enablebanking verifies that the shared ynabber.Date decoder
// accepts valid YYYY-MM-DD strings and rejects invalid ones.  This is the
// decoder that Config will invoke automatically when envconfig processes
// ENABLEBANKING_FROM_DATE / ENABLEBANKING_TO_DATE.
func TestDateDecode_Enablebanking(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "valid date",
			input: "2024-01-15",
			want:  time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "leap day",
			input: "2024-02-29",
			want:  time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "year boundary",
			input: "2025-12-31",
			want:  time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "wrong separator",
			input:   "2024/01/15",
			wantErr: true,
		},
		{
			name:    "day-month-year order",
			input:   "15-01-2024",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "date with time component",
			input:   "2024-01-15T12:00:00",
			wantErr: true,
		},
		{
			name:    "non-existent date",
			input:   "2023-02-29",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ynabber.Date
			err := got.Decode(tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Decode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && time.Time(got) != tt.want {
				t.Errorf("Decode(%q) = %v, want %v", tt.input, time.Time(got), tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config.FromDate field type
// ---------------------------------------------------------------------------

// TestConfigFromDateIsTypedDate asserts that Config.FromDate is ynabber.Date
// (i.e. time.Time underneath), not a raw string.  After Validate() the field
// must hold a correctly parsed time.Time value that callers can use directly
// without a secondary Parse call.
func TestConfigFromDateIsTypedDate(t *testing.T) {
	tests := []struct {
		name     string
		fromDate ynabber.Date
		wantTime time.Time
	}{
		{
			name:     "start of year",
			fromDate: mustDate(t, "2024-01-01"),
			wantTime: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "mid year",
			fromDate: mustDate(t, "2024-06-15"),
			wantTime: time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "end of year",
			fromDate: mustDate(t, "2024-12-31"),
			wantTime: time.Date(2024, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseConfig(t)
			cfg.FromDate = tt.fromDate

			if err := cfg.Validate("."); err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}

			// GetFromDate must now be a simple cast — no parsing, no error.
			got, err := cfg.GetFromDate()
			if err != nil {
				t.Fatalf("GetFromDate() unexpected error: %v", err)
			}
			if got != tt.wantTime {
				t.Errorf("GetFromDate() = %v, want %v", got, tt.wantTime)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config.Validate — FromDate zero value is an error
// ---------------------------------------------------------------------------

// TestConfigValidateZeroFromDateIsError confirms that Validate returns an
// error when FromDate is the zero ynabber.Date (i.e. the env var was not set).
func TestConfigValidateZeroFromDateIsError(t *testing.T) {
	tests := []struct {
		name     string
		fromDate ynabber.Date // zero value = not provided
		wantErr  bool
	}{
		{
			name:     "zero FromDate must error",
			fromDate: ynabber.Date{}, // envconfig zero — not provided
			wantErr:  true,
		},
		{
			name:     "non-zero FromDate must not error",
			fromDate: mustDate(t, "2024-01-01"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				AppID:       "test-app",
				Country:     "NO",
				ASPSP:       "DNB",
				RedirectURL: "https://example.com",
				PEMFile:     "test.pem",
				FromDate:    tt.fromDate,
			}
			err := cfg.Validate(".")
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config.Validate — ToDate defaults to today as ynabber.Date
// ---------------------------------------------------------------------------

// TestConfigValidateToDateDefaultsToTypedDate asserts that when ToDate is the
// zero ynabber.Date, Validate sets it to today's date — and that the result is
// a ynabber.Date (time.Time), not a string that needs re-parsing.
func TestConfigValidateToDateDefaultsToTypedDate(t *testing.T) {
	cfg := validBaseConfig(t)
	cfg.ToDate = ynabber.Date{} // explicitly zero — not provided

	if err := cfg.Validate("."); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	got, err := cfg.GetToDate()
	if err != nil {
		t.Fatalf("GetToDate() unexpected error: %v", err)
	}

	now := time.Now()
	if got.Year() != now.Year() || got.Month() != now.Month() || got.Day() != now.Day() {
		t.Errorf("GetToDate() = %v, want today (%v)", got, now.Format(ynabber.DateFormat))
	}
}

// ---------------------------------------------------------------------------
// Config.Validate — explicit ToDate is preserved as ynabber.Date
// ---------------------------------------------------------------------------

// TestConfigValidateExplicitToDateIsPreserved checks that when ToDate is
// explicitly provided it survives Validate unchanged.
func TestConfigValidateExplicitToDateIsPreserved(t *testing.T) {
	tests := []struct {
		name   string
		toDate ynabber.Date
		want   time.Time
	}{
		{
			name:   "explicit end-of-year",
			toDate: mustDate(t, "2024-12-31"),
			want:   time.Date(2024, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "explicit mid-year",
			toDate: mustDate(t, "2024-06-15"),
			want:   time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseConfig(t)
			cfg.ToDate = tt.toDate

			if err := cfg.Validate("."); err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}

			got, err := cfg.GetToDate()
			if err != nil {
				t.Fatalf("GetToDate() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("GetToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetFromDate / GetToDate are infallible after Validate
// ---------------------------------------------------------------------------

// TestGetFromDateNeverErrors verifies that GetFromDate never returns an error
// for any non-zero ynabber.Date value — there is no format to re-parse.
func TestGetFromDateNeverErrors(t *testing.T) {
	dates := []string{"2020-01-01", "2023-06-30", "2024-12-31"}

	for _, s := range dates {
		t.Run(s, func(t *testing.T) {
			cfg := validBaseConfig(t)
			cfg.FromDate = mustDate(t, s)

			_, err := cfg.GetFromDate()
			if err != nil {
				t.Errorf("GetFromDate() returned unexpected error for %q: %v", s, err)
			}
		})
	}
}
