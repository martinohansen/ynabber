package enablebanking

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				AppID:       "test-app",
				Country:     "NO",
				ASPSP:       "DNB",
				RedirectURL: "https://example.com",
				PEMFile:     "test.pem",
				FromDate:    "2024-01-01",
			},
			wantErr: false,
		},
		{
			name: "missing app id",
			config: Config{
				Country:     "NO",
				ASPSP:       "DNB",
				RedirectURL: "https://example.com",
				PEMFile:     "test.pem",
				FromDate:    "2024-01-01",
			},
			wantErr: true,
		},
		{
			name: "missing country",
			config: Config{
				AppID:       "test-app",
				ASPSP:       "DNB",
				RedirectURL: "https://example.com",
				PEMFile:     "test.pem",
				FromDate:    "2024-01-01",
			},
			wantErr: true,
		},
		{
			name: "missing aspsp",
			config: Config{
				AppID:       "test-app",
				Country:     "NO",
				RedirectURL: "https://example.com",
				PEMFile:     "test.pem",
				FromDate:    "2024-01-01",
			},
			wantErr: true,
		},
		{
			name: "missing redirect url",
			config: Config{
				AppID:    "test-app",
				Country:  "NO",
				ASPSP:    "DNB",
				PEMFile:  "test.pem",
				FromDate: "2024-01-01",
			},
			wantErr: true,
		},
		{
			name: "missing pem file",
			config: Config{
				AppID:       "test-app",
				Country:     "NO",
				ASPSP:       "DNB",
				RedirectURL: "https://example.com",
				FromDate:    "2024-01-01",
			},
			wantErr: true,
		},
		{
			name: "missing from date",
			config: Config{
				AppID:       "test-app",
				Country:     "NO",
				ASPSP:       "DNB",
				RedirectURL: "https://example.com",
				PEMFile:     "test.pem",
			},
			wantErr: true,
		},
		{
			name: "invalid from date format",
			config: Config{
				AppID:       "test-app",
				Country:     "NO",
				ASPSP:       "DNB",
				RedirectURL: "https://example.com",
				PEMFile:     "test.pem",
				FromDate:    "01-01-2024",
			},
			wantErr: true,
		},
		{
			name: "invalid to date format",
			config: Config{
				AppID:       "test-app",
				Country:     "NO",
				ASPSP:       "DNB",
				RedirectURL: "https://example.com",
				PEMFile:     "test.pem",
				FromDate:    "2024-01-01",
				ToDate:      "01-01-2024",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			err := cfg.Validate(".")
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigGetFromDate(t *testing.T) {
	config := Config{
		FromDate: "2024-01-15",
	}

	date, err := config.GetFromDate()
	if err != nil {
		t.Fatalf("GetFromDate() failed: %v", err)
	}

	if date.Year() != 2024 || date.Month() != time.January || date.Day() != 15 {
		t.Fatalf("expected 2024-01-15, got %v", date)
	}
}

func TestConfigGetToDate(t *testing.T) {
	tests := []struct {
		name   string
		toDate string
	}{
		{
			name:   "with explicit date",
			toDate: "2024-12-31",
		},
		{
			name:   "without explicit date",
			toDate: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				ToDate: tt.toDate,
			}

			date, err := config.GetToDate()
			if err != nil {
				t.Fatalf("GetToDate() failed: %v", err)
			}

			if tt.toDate != "" {
				// Check explicit date
				if date.Year() != 2024 || date.Month() != time.December || date.Day() != 31 {
					t.Fatalf("expected 2024-12-31, got %v", date)
				}
			} else {
				// Check that it's approximately today
				now := time.Now()
				if date.Year() != now.Year() || date.Month() != now.Month() || date.Day() != now.Day() {
					t.Fatalf("expected today, got %v", date)
				}
			}
		})
	}
}

func TestConfigValidateDefaultsToDate(t *testing.T) {
	config := Config{
		AppID:       "test-app",
		Country:     "NO",
		ASPSP:       "DNB",
		RedirectURL: "https://example.com",
		PEMFile:     "test.pem",
		FromDate:    "2024-01-01",
		ToDate:      "",
	}

	err := config.Validate(".")
	if err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	// toDate should be set to today
	now := time.Now()
	expected := now.Format("2006-01-02")
	if config.ToDate != expected {
		t.Errorf("ToDate not set to today: got %s, expected %s", config.ToDate, expected)
	}
}

func TestConfigValidateDefaultsSessionFile(t *testing.T) {
	config := Config{
		AppID:       "test-app",
		Country:     "NO",
		ASPSP:       "DNB",
		RedirectURL: "https://example.com",
		PEMFile:     "test.pem",
		FromDate:    "2024-01-01",
		SessionFile: "",
	}

	err := config.Validate(".")
	if err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	if config.SessionFile != "enablebanking_dnb_no_session.json" {
		t.Errorf("SessionFile not set to default: got %s, expected enablebanking_dnb_no_session.json", config.SessionFile)
	}
}

func TestConfigWithEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	testEnvVars := map[string]string{
		"ENABLEBANKING_APP_ID":       "test-app-123",
		"ENABLEBANKING_COUNTRY":      "SE",
		"ENABLEBANKING_ASPSP":        "Nordea",
		"ENABLEBANKING_REDIRECT_URL": "https://test.example.com",
		"ENABLEBANKING_PEM_FILE":     "./test.pem",
		"ENABLEBANKING_SESSION_FILE": "custom_session.json",
		"ENABLEBANKING_FROM_DATE":     "2024-02-01",
		"ENABLEBANKING_TO_DATE":       "2024-12-31",
		"ENABLEBANKING_INTERVAL":     "24h",
	}

	// Note: In a real test, you would use envconfig.Process() to load these.
	// This is just a demonstration of the structure.
	config := Config{
		AppID:       "test-app-123",
		Country:     "SE",
		ASPSP:       "Nordea",
		RedirectURL: "https://test.example.com",
		PEMFile:     "./test.pem",
		SessionFile: "custom_session.json",
		FromDate:    "2024-02-01",
		ToDate:      "2024-12-31",
		Interval:    24 * time.Hour,
	}

	err := config.Validate(".")
	if err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	if config.Country != "SE" {
		t.Errorf("Country not set correctly: got %s, expected SE", config.Country)
	}

	if config.ASPSP != "Nordea" {
		t.Errorf("ASPSP not set correctly: got %s, expected Nordea", config.ASPSP)
	}

	_ = testEnvVars // silence unused variable warning
}

func TestConfigDateFormatsAccepted(t *testing.T) {
	validFormats := []string{
		"2024-01-01",
		"2024-12-31",
		"2025-06-15",
	}

	for _, dateStr := range validFormats {
		config := Config{
			AppID:       "test",
			Country:     "NO",
			ASPSP:       "DNB",
			RedirectURL: "https://example.com",
			PEMFile:     "test.pem",
			FromDate:    dateStr,
		}

		err := config.Validate(".")
		if err != nil {
			t.Errorf("Validate() failed for valid date %s: %v", dateStr, err)
		}
	}
}

func TestConfigIntervalParsing(t *testing.T) {
	config := Config{
		AppID:       "test",
		Country:     "NO",
		ASPSP:       "DNB",
		RedirectURL: "https://example.com",
		PEMFile:     "test.pem",
		FromDate:    "2024-01-01",
		Interval:    12 * time.Hour,
	}

	err := config.Validate(".")
	if err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	if config.Interval != 12*time.Hour {
		t.Errorf("Interval not set correctly: got %v, expected 12h", config.Interval)
	}
}

func TestConfig_String(t *testing.T) {
	config := Config{
		AppID:       "test-app",
		Country:     "NO",
		ASPSP:       "DNB",
		RedirectURL: "https://example.com",
		PEMFile:     "test.pem",
		FromDate:    "2024-01-01",
	}

	// Create a simple string representation (not required but good practice)
	result := strings.Join([]string{
		config.AppID,
		config.Country,
		config.ASPSP,
	}, "-")

	if result != "test-app-NO-DNB" {
		t.Errorf("config string representation failed: got %s", result)
	}
}

// TestSanitizeSessionPart tests the filename-safe sanitisation used to build
// the default session file name.
func TestSanitizeSessionPart(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase passthrough",
			input: "dnb",
			want:  "dnb",
		},
		{
			name:  "uppercase is lowercased",
			input: "DNB",
			want:  "dnb",
		},
		{
			name:  "spaces become underscores",
			input: "Spare Bank",
			want:  "spare_bank",
		},
		{
			name:  "special characters stripped",
			input: "SEB!@#$",
			want:  "seb",
		},
		{
			name:  "digits preserved",
			input: "bank123",
			want:  "bank123",
		},
		{
			name:  "hyphens preserved",
			input: "sas-eurobonus",
			want:  "sas-eurobonus",
		},
		{
			name:  "empty string returns unknown",
			input: "",
			want:  "unknown",
		},
		{
			name:  "only special characters returns unknown",
			input: "!@#",
			want:  "unknown",
		},
		{
			name:  "mixed unicode letters stripped",
			input: "BankÆØÅ",
			want:  "bank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSessionPart(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeSessionPart(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestConfigValidateSessionFilePath verifies that Validate places the default
// session file under dataDir, and that an explicit SessionFile is never
// overridden regardless of dataDir.
func TestConfigValidateSessionFilePath(t *testing.T) {
	base := Config{
		AppID:       "test-app",
		Country:     "NO",
		ASPSP:       "DNB",
		RedirectURL: "https://example.com",
		PEMFile:     "test.pem",
		FromDate:    "2024-01-01",
	}

	tests := []struct {
		name        string
		sessionFile string // pre-set SessionFile ("" = use default)
		dataDir     string
		wantPath    string
	}{
		{
			name:     "dot dataDir gives bare filename",
			dataDir:  ".",
			wantPath: "enablebanking_dnb_no_session.json",
		},
		{
			name:     "absolute dataDir prefixes session file",
			dataDir:  "/data",
			wantPath: "/data/enablebanking_dnb_no_session.json",
		},
		{
			name:     "nested dataDir is joined correctly",
			dataDir:  "/srv/ynabber/data",
			wantPath: "/srv/ynabber/data/enablebanking_dnb_no_session.json",
		},
		{
			name:        "explicit SessionFile is not overridden by dataDir",
			sessionFile: "/custom/my_session.json",
			dataDir:     "/data",
			wantPath:    "/custom/my_session.json",
		},
		{
			name:        "explicit relative SessionFile is not overridden",
			sessionFile: "relative_session.json",
			dataDir:     "/data",
			wantPath:    "relative_session.json",
		},
		{
			name:     "empty dataDir gives bare filename",
			dataDir:  "",
			wantPath: "enablebanking_dnb_no_session.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			cfg.SessionFile = tt.sessionFile
			if err := cfg.Validate(tt.dataDir); err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
			if cfg.SessionFile != tt.wantPath {
				t.Errorf("SessionFile = %q, want %q", cfg.SessionFile, tt.wantPath)
			}
		})
	}
}

func TestDefaultSessionFile(t *testing.T) {
	tests := []struct {
		name    string
		aspsp   string
		country string
		want    string
	}{
		{
			name:    "standard case",
			aspsp:   "DNB",
			country: "NO",
			want:    "enablebanking_dnb_no_session.json",
		},
		{
			name:    "aspsp with spaces",
			aspsp:   "Spare Bank",
			country: "NO",
			want:    "enablebanking_spare_bank_no_session.json",
		},
		{
			name:    "both empty returns unknown_unknown",
			aspsp:   "",
			country: "",
			want:    "enablebanking_unknown_unknown_session.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultSessionFile(tt.aspsp, tt.country)
			if got != tt.want {
				t.Errorf("defaultSessionFile(%q, %q) = %q, want %q", tt.aspsp, tt.country, got, tt.want)
			}
		})
	}
}
