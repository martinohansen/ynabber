package ynabber

import "testing"

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

func TestMilliunitsFromAmountFloatPrecision(t *testing.T) {
	tests := []struct {
		input float64
		want  Milliunits
	}{
		{65.02, 65020},
		{-65.02, -65020},
		{0.29, 290},
		{19.99, 19990},
		{0.1, 100},
		{1.10, 1100},
	}
	for _, tt := range tests {
		got := MilliunitsFromAmount(tt.input)
		if got != tt.want {
			t.Errorf("MilliunitsFromAmount(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestMilliunitsFromString(t *testing.T) {
	tests := []struct {
		input   string
		want    Milliunits
		wantErr bool
	}{
		{"65.02", 65020, false},
		{"-65.02", -65020, false},
		{"0.00", 0, false},
		{"0.01", 10, false},
		{"123.45", 123450, false},
		{"1000.00", 1000000, false},
		{"1", 1000, false},
		{"-2.99", -2990, false},
		{"4924.34", 4924340, false},
		{"0.1", 100, false},
		{"0.29", 290, false},
		{"1.10", 1100, false},
		{"19.99", 19990, false},
		{"+5.50", 5500, false},
		{"  10.00  ", 10000, false},
		{"", 0, true},
		{"abc", 0, true},
		{"65.0195", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := MilliunitsFromString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("MilliunitsFromString(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("MilliunitsFromString(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
