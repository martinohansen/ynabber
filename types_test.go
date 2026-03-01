package ynabber

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMilliunitsFromAmount(t *testing.T) {
	want := Milliunits(123930)
	got := MilliunitsFromAmount(123.93)
	if want != got {
		t.Fatalf("period separated: %s != %s", want, got)
	}

	want = Milliunits(4924340)
	got = MilliunitsFromAmount(4924.34)
	if want != got {
		t.Fatalf("comma separated: %s != %s", want, got)
	}

	want = Milliunits(-2990)
	got = MilliunitsFromAmount(-2.99)
	if want != got {
		t.Fatalf("negative amount: %s != %s", want, got)
	}

	want = Milliunits(-3899000)
	got = MilliunitsFromAmount(-3899)
	if want != got {
		t.Fatalf("amount with no separator: %s != %s", want, got)
	}

	want = Milliunits(328180)
	got = MilliunitsFromAmount(328.18)
	if want != got {
		t.Fatalf("amount with no separator: %s != %s", want, got)
	}

	want = Milliunits(32818000)
	got = MilliunitsFromAmount(32818)
	if want != got {
		t.Fatalf("amount with no separator: %s != %s", want, got)
	}
}

func TestDateMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		date    Date
		want    string
		wantErr bool
	}{
		{
			name: "standard date",
			date: Date(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)),
			want: `"2024-01-15"`,
		},
		{
			name: "local time is normalised to UTC",
			date: Date(time.Date(2024, 6, 1, 23, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))),
			want: `"2024-06-01"`, // UTC+2 23:00 = UTC 21:00, still same UTC date
		},
		{
			name: "zero value",
			date: Date{},
			want: `"0001-01-01"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.date)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if string(got) != tt.want {
				t.Errorf("MarshalJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDateUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "standard date",
			input: `"2024-01-15"`,
			want:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "null is a no-op",
			input: `null`,
			want:  time.Time{},
		},
		{
			name:    "invalid format",
			input:   `"15-01-2024"`,
			wantErr: true,
		},
		{
			name:    "not a string",
			input:   `20240115`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Date
			err := json.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && time.Time(got) != tt.want {
				t.Errorf("UnmarshalJSON() = %v, want %v", time.Time(got), tt.want)
			}
		})
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	original := Date(time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC))

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored Date
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if time.Time(original) != time.Time(restored) {
		t.Errorf("round-trip mismatch: got %v, want %v", time.Time(restored), time.Time(original))
	}
}
