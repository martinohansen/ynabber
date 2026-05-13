package enablebanking

import (
	"regexp"
	"testing"
)

func TestPayeeRegexDecode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{name: "empty", value: "", want: nil},
		{name: "single", value: `^Dk-Nota\S+\s+`, want: []string{`^Dk-Nota\S+\s+`}},
		{name: "multiple", value: `^foo,^bar`, want: []string{`^foo`, `^bar`}},
		{name: "trim+blank-segments", value: ` ^foo , , ^bar `, want: []string{`^foo`, `^bar`}},
		{name: "invalid", value: `(unclosed`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pr PayeeRegex
			err := pr.Decode(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decode(%q) err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(pr) != len(tt.want) {
				t.Fatalf("len=%d, want %d (%v)", len(pr), len(tt.want), pr)
			}
			for i, re := range pr {
				if re.String() != tt.want[i] {
					t.Errorf("pattern[%d]=%q, want %q", i, re.String(), tt.want[i])
				}
			}
		})
	}
}

func TestStripRegex(t *testing.T) {
	mustCompile := func(patterns ...string) PayeeRegex {
		var pr PayeeRegex
		for _, p := range patterns {
			pr = append(pr, regexp.MustCompile(p))
		}
		return pr
	}

	tests := []struct {
		name    string
		s       string
		regexes PayeeRegex
		want    string
	}{
		{
			name:    "dk-nota-prefix",
			s:       "Dk-Nota118-2 Clustercf.Dk",
			regexes: mustCompile(`^Dk-Nota\S+\s+`),
			want:    "Clustercf.Dk",
		},
		{
			name:    "dk-nota-with-spaces",
			s:       "Dk-Nota61240 365 Hørning",
			regexes: mustCompile(`^Dk-Nota\S+\s+`),
			want:    "365 Hørning",
		},
		{
			name:    "dk-nota-jumbo",
			s:       "Dk-Notac0285 Jumbo Bakery&Eater",
			regexes: mustCompile(`^Dk-Nota\S+\s+`),
			want:    "Jumbo Bakery&Eater",
		},
		{
			name:    "no-match",
			s:       "Some Payee",
			regexes: mustCompile(`^Dk-Nota\S+\s+`),
			want:    "Some Payee",
		},
		{
			name:    "multiple-patterns",
			s:       "Dk-Nota61221 Visa Remouladen",
			regexes: mustCompile(`^Dk-Nota\S+\s+`, `\bVisa\s+`),
			want:    "Remouladen",
		},
		{
			name:    "collapses-spaces-after-removal",
			s:       "foo CODE bar",
			regexes: mustCompile(`CODE`),
			want:    "foo bar",
		},
		{
			name:    "no-regexes",
			s:       "foo",
			regexes: nil,
			want:    "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripRegex(tt.s, tt.regexes); got != tt.want {
				t.Errorf("stripRegex() = %q, want %q", got, tt.want)
			}
		})
	}
}
