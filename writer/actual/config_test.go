package actual

import (
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
