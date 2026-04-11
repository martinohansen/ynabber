package enablebanking

import (
	"os"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// mustDate is a test helper that decodes a YYYY-MM-DD string into Date
// or fails the test immediately if the string is malformed.
func mustDate(t *testing.T, s string) Date {
	t.Helper()
	var d Date
	if err := d.Decode(s); err != nil {
		t.Fatalf("mustDate(%q): %v", s, err)
	}
	return d
}

// validBaseConfig returns a Config with all required fields set using typed
// Date values. Individual test cases override specific fields.
func validBaseConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		AppID:    "test-app",
		Country:  "NO",
		ASPSP:    "DNB",
		PEMFile:  "test.pem",
		FromDate: mustDate(t, "2024-01-01"), // typed Date, not string
	}
}

func sameUTCDate(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

// ---------------------------------------------------------------------------
// Date.Decode
// ---------------------------------------------------------------------------

// TestDateDecode_Enablebanking verifies that the shared Date decoder
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
			var got Date
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

// TestConfigFromDateIsTypedDate asserts that Config.FromDate is Date
// (i.e. time.Time underneath), not a raw string.  After Validate() the field
// must hold a correctly parsed time.Time value that callers can use directly
// without a secondary Parse call.
func TestConfigFromDateIsTypedDate(t *testing.T) {
	tests := []struct {
		name     string
		fromDate Date
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
// envconfig integration — FromDate required + Decode validation
// ---------------------------------------------------------------------------

// TestEnvconfigFromDateDecode verifies that envconfig.Process enforces the
// required:"true" tag on FromDate and that Date.Decode rejects
// malformed date strings before they reach Validate.
func TestEnvconfigFromDateDecode(t *testing.T) {
	base := map[string]string{
		"ENABLEBANKING_APP_ID":   "test-app",
		"ENABLEBANKING_COUNTRY":  "NO",
		"ENABLEBANKING_ASPSP":    "DNB",
		"ENABLEBANKING_PEM_FILE": "test.pem",
	}

	tests := []struct {
		name     string
		fromDate string // value for ENABLEBANKING_FROM_DATE; "" = leave unset
		wantErr  bool
	}{
		{name: "valid date accepted", fromDate: "2024-01-01", wantErr: false},
		{name: "absent date rejected (required)", fromDate: "", wantErr: true},
		{name: "wrong separator rejected", fromDate: "2024/01/01", wantErr: true},
		{name: "dd-mm-yyyy order rejected", fromDate: "01-01-2024", wantErr: true},
		{name: "datetime suffix rejected", fromDate: "2024-01-01T00:00:00Z", wantErr: true},
		{name: "plain text rejected", fromDate: "today", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range base {
				t.Setenv(k, v)
			}
			t.Setenv("ENABLEBANKING_FROM_DATE", tt.fromDate)

			var cfg Config
			err := envconfig.Process("", &cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("envconfig.Process() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config.Validate — omitted ToDate remains dynamic
// ---------------------------------------------------------------------------

// TestConfigValidateToDateDefaultsToTypedDate asserts that when ToDate is the
// zero Date, Validate leaves it unset so GetToDate can resolve it dynamically
// on each run.
func TestConfigValidateToDateDefaultsToTypedDate(t *testing.T) {
	cfg := validBaseConfig(t)
	cfg.ToDate = Date{} // explicitly zero — not provided

	if err := cfg.Validate("."); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if !time.Time(cfg.ToDate).IsZero() {
		t.Fatalf("Validate() materialized omitted ToDate as %v; want zero Date", time.Time(cfg.ToDate))
	}

	got, err := cfg.GetToDate()
	if err != nil {
		t.Fatalf("GetToDate() unexpected error: %v", err)
	}

	now := time.Now().UTC()
	if !sameUTCDate(got, now) {
		t.Errorf("GetToDate() = %v, want today (%v)", got, now.Format(dateFormat))
	}
}

// TestConfigGetToDateZeroDateUsesCurrentUTCDate captures the continuous-mode
// regression: when ENABLEBANKING_TO_DATE is omitted, date resolution must stay
// dynamic and follow the current UTC date instead of returning the zero time.
func TestConfigGetToDateZeroDateUsesCurrentUTCDate(t *testing.T) {
	tests := []struct {
		name   string
		toDate Date
		want   time.Time
	}{
		{
			name:   "implicit zero ToDate resolves to current UTC date",
			toDate: Date{},
		},
		{
			name:   "explicit ToDate remains fixed",
			toDate: mustDate(t, "2024-12-31"),
			want:   time.Date(2024, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBaseConfig(t)
			cfg.ToDate = tt.toDate

			got, err := cfg.GetToDate()
			if err != nil {
				t.Fatalf("GetToDate() unexpected error: %v", err)
			}

			if time.Time(tt.toDate).IsZero() {
				now := time.Now().UTC()
				if !sameUTCDate(got, now) {
					t.Fatalf("GetToDate() = %v, want current UTC date (%s) when ToDate is omitted",
						got, now.Format(dateFormat))
				}
				return
			}

			if got != tt.want {
				t.Fatalf("GetToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEnvconfigOmittedToDateRemainsDynamic verifies the narrow seam needed for
// continuous mode: omitting ENABLEBANKING_TO_DATE must not be materialized into
// a fixed date at startup, while an explicit date must remain fixed.
func TestEnvconfigOmittedToDateRemainsDynamic(t *testing.T) {
	baseEnv := map[string]string{
		"ENABLEBANKING_APP_ID":    "test-app",
		"ENABLEBANKING_COUNTRY":   "NO",
		"ENABLEBANKING_ASPSP":     "DNB",
		"ENABLEBANKING_PEM_FILE":  "test.pem",
		"ENABLEBANKING_FROM_DATE": "2024-01-01",
	}

	explicitToDate := "2024-12-31"
	tests := []struct {
		name           string
		toDateEnv      *string
		wantStoredZero bool
		want           time.Time
	}{
		{
			name:           "omitted ToDate stays unset and resolves dynamically",
			toDateEnv:      nil,
			wantStoredZero: true,
		},
		{
			name:      "explicit ToDate stays fixed after Validate",
			toDateEnv: &explicitToDate,
			want:      time.Date(2024, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range baseEnv {
				t.Setenv(k, v)
			}

			const toDateKey = "ENABLEBANKING_TO_DATE"
			if tt.toDateEnv == nil {
				prev, had := os.LookupEnv(toDateKey)
				os.Unsetenv(toDateKey)
				t.Cleanup(func() {
					if had {
						_ = os.Setenv(toDateKey, prev)
					} else {
						_ = os.Unsetenv(toDateKey)
					}
				})
			} else {
				t.Setenv(toDateKey, *tt.toDateEnv)
			}

			var cfg Config
			if err := envconfig.Process("", &cfg); err != nil {
				t.Fatalf("envconfig.Process() unexpected error: %v", err)
			}

			if err := cfg.Validate("."); err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}

			stored := time.Time(cfg.ToDate)
			if tt.wantStoredZero {
				if !stored.IsZero() {
					t.Fatalf("Validate() materialized omitted ENABLEBANKING_TO_DATE as %v; want zero Date so later runs can advance",
						stored)
				}
			} else if stored != tt.want {
				t.Fatalf("Validate() changed explicit ToDate = %v, want %v", stored, tt.want)
			}

			got, err := cfg.GetToDate()
			if err != nil {
				t.Fatalf("GetToDate() unexpected error: %v", err)
			}

			if tt.wantStoredZero {
				now := time.Now().UTC()
				if !sameUTCDate(got, now) {
					t.Fatalf("GetToDate() = %v, want current UTC date (%s) when ENABLEBANKING_TO_DATE is omitted",
						got, now.Format(dateFormat))
				}
				return
			}

			if got != tt.want {
				t.Fatalf("GetToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config.Validate — explicit ToDate is preserved as Date
// ---------------------------------------------------------------------------

// TestConfigValidateExplicitToDateIsPreserved checks that when ToDate is
// explicitly provided it survives Validate unchanged.
func TestConfigValidateExplicitToDateIsPreserved(t *testing.T) {
	tests := []struct {
		name   string
		toDate Date
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
// for any non-zero Date value — there is no format to re-parse.
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
