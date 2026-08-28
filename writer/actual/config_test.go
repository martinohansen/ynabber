package actual

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDateDecode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "empty string yields zero time",
			value:   "",
			want:    time.Time{},
			wantErr: false,
		},
		{
			name:    "valid date",
			value:   "2024-05-10",
			want:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "malformed date",
			value:   "not-a-date",
			want:    time.Time{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Date{}
			err := d.Decode(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Date.Decode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && d.Time() != tt.want {
				t.Errorf("Date.Decode() got = %v, want %v", d.Time(), tt.want)
			}
		})
	}
}

func TestAccountMapDecode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    AccountMap
		wantErr bool
	}{
		{
			name:    "empty string yields empty map",
			value:   "",
			want:    AccountMap{},
			wantErr: false,
		},
		{
			name:    "valid JSON",
			value:   `{"IBAN1":"account-1","IBAN2":"account-2"}`,
			want:    AccountMap{"IBAN1": "account-1", "IBAN2": "account-2"},
			wantErr: false,
		},
		{
			name:    "malformed JSON",
			value:   `{invalid}`,
			want:    AccountMap{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AccountMap{}
			err := a.Decode(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("AccountMap.Decode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(*a) != len(tt.want) {
					t.Errorf("AccountMap.Decode() got %d entries, want %d", len(*a), len(tt.want))
				}
				for k, v := range tt.want {
					if got, ok := (*a)[k]; !ok || got != v {
						t.Errorf("AccountMap.Decode() key %q = %q, want %q", k, got, v)
					}
				}
			}
		})
	}
}

func TestNewWriterRejectsInvalidBatchLimits(t *testing.T) {
	tests := []struct {
		name      string
		maxBytes  string
		batchSize string
		want      string
	}{
		{name: "zero maximum bytes", maxBytes: "0", batchSize: "100", want: "ACTUAL_MAX_REQUEST_BYTES"},
		{name: "negative maximum bytes", maxBytes: "-1", batchSize: "100", want: "ACTUAL_MAX_REQUEST_BYTES"},
		{name: "zero batch size", maxBytes: "81920", batchSize: "0", want: "ACTUAL_BATCH_SIZE"},
		{name: "negative batch size", maxBytes: "81920", batchSize: "-1", want: "ACTUAL_BATCH_SIZE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ACTUAL_BASE_URL", "https://actual.example.com")
			t.Setenv("ACTUAL_BUDGET_ID", "budget-1")
			t.Setenv("ACTUAL_ACCOUNTMAP", `{"source":"account-1"}`)
			t.Setenv("ACTUAL_MAX_REQUEST_BYTES", tt.maxBytes)
			t.Setenv("ACTUAL_BATCH_SIZE", tt.batchSize)

			_, err := NewWriter()
			if err == nil {
				t.Fatal("expected configuration error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestNewWriterAppliesBatchDefaults(t *testing.T) {
	t.Setenv("ACTUAL_BASE_URL", "https://actual.example.com")
	t.Setenv("ACTUAL_BUDGET_ID", "budget-1")
	t.Setenv("ACTUAL_ACCOUNTMAP", `{"source":"account-1"}`)
	unsetEnv(t, "ACTUAL_MAX_REQUEST_BYTES")
	unsetEnv(t, "ACTUAL_BATCH_SIZE")

	writer, err := NewWriter()
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if writer.Config.MaxRequestBytes != defaultMaxRequestBytes {
		t.Fatalf("MaxRequestBytes = %d, want %d", writer.Config.MaxRequestBytes, defaultMaxRequestBytes)
	}
	if writer.Config.BatchSize != defaultBatchSize {
		t.Fatalf("BatchSize = %d, want %d", writer.Config.BatchSize, defaultBatchSize)
	}
}

func TestNewWriterAppliesBatchOverrides(t *testing.T) {
	t.Setenv("ACTUAL_BASE_URL", "https://actual.example.com")
	t.Setenv("ACTUAL_BUDGET_ID", "budget-1")
	t.Setenv("ACTUAL_ACCOUNTMAP", `{"source":"account-1"}`)
	t.Setenv("ACTUAL_MAX_REQUEST_BYTES", "4096")
	t.Setenv("ACTUAL_BATCH_SIZE", "25")

	writer, err := NewWriter()
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if writer.Config.MaxRequestBytes != 4096 {
		t.Fatalf("MaxRequestBytes = %d, want 4096", writer.Config.MaxRequestBytes)
	}
	if writer.Config.BatchSize != 25 {
		t.Fatalf("BatchSize = %d, want 25", writer.Config.BatchSize)
	}
}

// unsetEnv preserves the caller's original environment through t.Setenv's
// cleanup while presenting an actually absent variable to envconfig.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetting %s: %v", key, err)
	}
}
