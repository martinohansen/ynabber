package wealthreader

import (
	"os"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
)

func mustDate(t *testing.T, value string) Date {
	t.Helper()
	var d Date
	if err := d.Decode(value); err != nil {
		t.Fatalf("decode date %s: %v", value, err)
	}
	return d
}

func TestEnvconfigRequiredFields(t *testing.T) {
	t.Setenv("_PARALLEL_GUARD", "")
	allVars := map[string]string{
		"WEALTHREADER_API_KEY":      "11dfdeab",
		"WEALTHREADER_CODE":         "bbva",
		"WEALTHREADER_REDIRECT_URL": "https://example.com/ok",
		"WEALTHREADER_FROM_DATE":    "2024-01-01",
	}

	tests := []struct {
		name    string
		omit    string
		wantErr bool
	}{
		{name: "all required fields set", omit: "", wantErr: false},
		{name: "missing API_KEY", omit: "WEALTHREADER_API_KEY", wantErr: true},
		{name: "missing CODE", omit: "WEALTHREADER_CODE", wantErr: true},
		{name: "missing REDIRECT_URL", omit: "WEALTHREADER_REDIRECT_URL", wantErr: true},
		{name: "missing FROM_DATE", omit: "WEALTHREADER_FROM_DATE", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range allVars {
				if k == tt.omit {
					prev, had := os.LookupEnv(k)
					os.Unsetenv(k)
					t.Cleanup(func() {
						if had {
							os.Setenv(k, prev)
						} else {
							os.Unsetenv(k)
						}
					})
				} else {
					t.Setenv(k, v)
				}
			}
			var cfg Config
			err := envconfig.Process("", &cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("envconfig.Process() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidateDefaultsSessionFile(t *testing.T) {
	config := Config{
		ApiKey:      "11dfdeab",
		Code:        "BBVA",
		RedirectURL: "https://example.com/ok",
		FromDate:    mustDate(t, "2024-01-01"),
	}
	if err := config.Validate("."); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
	if config.SessionFile != "wealthreader_bbva_session.json" {
		t.Errorf("SessionFile = %q, want wealthreader_bbva_session.json", config.SessionFile)
	}
}

func TestConfigGetToDate(t *testing.T) {
	config := Config{ToDate: Date{}}
	date, err := config.GetToDate()
	if err != nil {
		t.Fatalf("GetToDate() failed: %v", err)
	}
	now := time.Now().UTC()
	if date.Year() != now.Year() || date.Month() != now.Month() || date.Day() != now.Day() {
		t.Errorf("expected current UTC date, got %v", date)
	}

	config.ToDate = mustDate(t, "2024-12-31")
	date, err = config.GetToDate()
	if err != nil {
		t.Fatalf("GetToDate() failed: %v", err)
	}
	if date.Year() != 2024 || date.Month() != time.December || date.Day() != 31 {
		t.Errorf("expected 2024-12-31, got %v", date)
	}
}

func TestSanitizeSessionPart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"bbva", "bbva"},
		{"BBVA", "bbva"},
		{"psd2_abanca", "psd2_abanca"},
		{"Caixa Bank", "caixa_bank"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		if got := sanitizeSessionPart(tt.input); got != tt.want {
			t.Errorf("sanitizeSessionPart(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
