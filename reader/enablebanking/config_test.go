package enablebanking

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// setConfigEnv isolates both envconfig field names and explicit environment
// names, then supplies the required Enable Banking settings.
func setConfigEnv(t *testing.T) {
	t.Helper()
	configType := reflect.TypeFor[Config]()
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		for _, key := range []string{strings.ToUpper(field.Name), field.Tag.Get("envconfig")} {
			unsetConfigEnv(t, key)
		}
	}
	for key, value := range map[string]string{
		"ENABLEBANKING_APP_ID":    "test-app",
		"ENABLEBANKING_COUNTRY":   "NO",
		"ENABLEBANKING_ASPSP":     "DNB",
		"ENABLEBANKING_PEM_FILE":  "test.pem",
		"ENABLEBANKING_FROM_DATE": "2024-01-01",
	} {
		t.Setenv(key, value)
	}
}

func unsetConfigEnv(t *testing.T, key string) {
	t.Helper()
	// Setenv registers restoration and prevents parallel environment changes.
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
}

// TestEnvconfigRequiredFields verifies that required settings cannot be absent.
// envconfig permits empty strings; the Date decoder rejects an empty date.
func TestEnvconfigRequiredFields(t *testing.T) {
	for _, key := range []string{
		"ENABLEBANKING_APP_ID", "ENABLEBANKING_COUNTRY", "ENABLEBANKING_ASPSP",
		"ENABLEBANKING_PEM_FILE", "ENABLEBANKING_FROM_DATE",
	} {
		t.Run(key, func(t *testing.T) {
			setConfigEnv(t)
			unsetConfigEnv(t, key)
			var cfg Config
			if err := envconfig.Process("", &cfg); err == nil {
				t.Fatalf("expected an error for missing %s", key)
			}
		})
	}
}

func TestEnvconfigPSUSettings(t *testing.T) {
	setConfigEnv(t)

	var defaults Config
	if err := envconfig.Process("", &defaults); err != nil {
		t.Fatalf("load default PSU settings: %v", err)
	}
	if defaults.PSUIPAddress != "" {
		t.Errorf("PSUIPAddress = %q, want no default", defaults.PSUIPAddress)
	}
	if defaults.PSUUserAgent != "Mozilla/5.0 (compatible; Ynabber/1.0)" {
		t.Errorf("PSUUserAgent = %q, want default user agent", defaults.PSUUserAgent)
	}
	if defaults.PSUHeaders != nil {
		t.Errorf("PSUHeaders = %v, want nil when unset", *defaults.PSUHeaders)
	}

	t.Setenv("ENABLEBANKING_PSU_IP_ADDRESS", "203.0.113.10")
	t.Setenv("ENABLEBANKING_PSU_USER_AGENT", "custom-agent")
	t.Setenv("ENABLEBANKING_PSU_HEADERS", "false")

	var configured Config
	if err := envconfig.Process("", &configured); err != nil {
		t.Fatalf("load explicit PSU settings: %v", err)
	}
	if configured.PSUIPAddress != "203.0.113.10" {
		t.Errorf("PSUIPAddress = %q, want configured value", configured.PSUIPAddress)
	}
	if configured.PSUUserAgent != "custom-agent" {
		t.Errorf("PSUUserAgent = %q, want configured value", configured.PSUUserAgent)
	}
	if configured.PSUHeaders == nil || *configured.PSUHeaders {
		t.Errorf("PSUHeaders = %v, want false", configured.PSUHeaders)
	}
}

func TestConfigGetFromDate(t *testing.T) {
	config := Config{
		FromDate: mustDate(t, "2024-01-15"),
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
		name    string
		toDate  Date
		wantNow bool
	}{
		{
			name:   "with explicit date",
			toDate: mustDate(t, "2024-12-31"),
		},
		{
			name:    "zero Date resolves to current UTC date",
			toDate:  Date{},
			wantNow: true,
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

			if tt.wantNow {
				now := time.Now().UTC()
				if !sameUTCDate(date, now) {
					t.Errorf("expected current UTC date, got %v (now %v)", date, now)
				}
				return
			}

			// Check explicit date
			if date.Year() != 2024 || date.Month() != time.December || date.Day() != 31 {
				t.Fatalf("expected 2024-12-31, got %v", date)
			}
		})
	}
}

func TestConfigWithEnvironmentVariables(t *testing.T) {
	setConfigEnv(t)
	for key, value := range map[string]string{
		"ENABLEBANKING_APP_ID":       "test-app-123",
		"ENABLEBANKING_COUNTRY":      "SE",
		"ENABLEBANKING_ASPSP":        "Nordea",
		"ENABLEBANKING_PEM_FILE":     "./test.pem",
		"ENABLEBANKING_SESSION_FILE": "custom_session.json",
		"ENABLEBANKING_FROM_DATE":    "2024-02-01",
		"ENABLEBANKING_TO_DATE":      "2024-12-31",
		"ENABLEBANKING_INTERVAL":     "24h",
	} {
		t.Setenv(key, value)
	}
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.AppID != "test-app-123" || cfg.Country != "SE" || cfg.ASPSP != "Nordea" ||
		cfg.PEMFile != "./test.pem" || cfg.SessionFile != "custom_session.json" ||
		cfg.FromDate != mustDate(t, "2024-02-01") || cfg.ToDate != mustDate(t, "2024-12-31") ||
		cfg.Interval != 24*time.Hour {
		t.Fatalf("unexpected parsed configuration: %+v", cfg)
	}
}

func TestConfigIntervalParsing(t *testing.T) {
	for _, value := range []string{"12h", "invalid"} {
		t.Run(value, func(t *testing.T) {
			setConfigEnv(t)
			t.Setenv("ENABLEBANKING_INTERVAL", value)
			var cfg Config
			err := envconfig.Process("", &cfg)
			if value == "invalid" {
				if err == nil {
					t.Fatal("expected invalid duration to fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Interval != 12*time.Hour {
				t.Fatalf("Interval = %v, want 12h", cfg.Interval)
			}
		})
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

// TestNewReaderSessionFilePath checks derived defaults and explicit overrides.
func TestNewReaderSessionFilePath(t *testing.T) {

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
			setConfigEnv(t)
			t.Setenv("ENABLEBANKING_SESSION_FILE", tt.sessionFile)
			t.Setenv("ENABLEBANKING_PSU_HEADERS", "false")
			reader, err := NewReader(slog.New(slog.NewTextHandler(io.Discard, nil)), tt.dataDir)
			if err != nil {
				t.Fatal(err)
			}
			if reader.Config.SessionFile != tt.wantPath {
				t.Errorf("SessionFile = %q, want %q", reader.Config.SessionFile, tt.wantPath)
			}
			if reader.Auth.Config.SessionFile != tt.wantPath || reader.Client.config.SessionFile != tt.wantPath {
				t.Fatal("session path was not propagated to auth and client")
			}
			if !time.Time(reader.Config.ToDate).IsZero() {
				t.Fatal("omitted ToDate must remain unset after reader construction")
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

func TestEnvconfigPSUType(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		omitted bool
		want    PSUType
		wantErr bool
	}{
		{name: "omitted", omitted: true, want: psuTypePersonal},
		{name: "empty", want: psuTypePersonal},
		{name: "whitespace", value: " \t ", want: psuTypePersonal},
		{name: "personal", value: "personal", want: psuTypePersonal},
		{name: "business", value: "business", want: psuTypeBusiness},
		{name: "mixed case", value: "Business", want: psuTypeBusiness},
		{name: "surrounding space", value: " personal ", want: psuTypePersonal},
		{name: "invalid", value: "corporate", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setConfigEnv(t)
			if !tt.omitted {
				t.Setenv("ENABLEBANKING_PSU_TYPE", tt.value)
			}
			var cfg Config
			err := envconfig.Process("", &cfg)
			if tt.wantErr {
				var parseErr *envconfig.ParseError
				if !errors.As(err, &parseErr) {
					t.Fatalf("expected envconfig.ParseError, got %v", err)
				}
				if parseErr.FieldName != "PSUType" || parseErr.Value != tt.value {
					t.Fatalf("unexpected parse error: %+v", parseErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.PSUType != tt.want {
				t.Errorf("PSUType = %q, want %q", cfg.PSUType, tt.want)
			}
		})
	}
}

type unexpectedConfigTransport struct{ t *testing.T }

func (tr unexpectedConfigTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.t.Errorf("unexpected HTTP request before config rejection: %s", req.URL)
	return nil, errors.New("unexpected HTTP request")
}

func TestNewReaderRejectsInvalidPSUTypeBeforeDiscovery(t *testing.T) {
	setConfigEnv(t)
	t.Setenv("ENABLEBANKING_PSU_TYPE", "corporate")
	t.Setenv("ENABLEBANKING_PSU_HEADERS", "true")
	previous := http.DefaultTransport
	http.DefaultTransport = unexpectedConfigTransport{t: t}
	t.Cleanup(func() { http.DefaultTransport = previous })

	_, err := NewReader(slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	var parseErr *envconfig.ParseError
	if !errors.As(err, &parseErr) || parseErr.FieldName != "PSUType" {
		t.Fatalf("expected PSUType parse error, got %v", err)
	}
	if !strings.HasPrefix(err.Error(), "loading config:") {
		t.Fatalf("unexpected error context: %v", err)
	}
}
