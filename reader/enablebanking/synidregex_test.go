package enablebanking

import (
	"strings"
	"testing"
)

func TestSynIDRegexDecode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{name: "empty", value: ""},
		{name: "single", value: `^Visa`, want: []string{`^Visa`}},
		{name: "multiple", value: `^Visa;Aut\.\s*\d+`, want: []string{`^Visa`, `Aut\.\s*\d+`}},
		{name: "trim and ignore blanks", value: ` ^Visa ; ; Aut `, want: []string{`^Visa`, `Aut`}},
		{name: "invalid", value: `(unclosed`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var patterns SynIDRegex
			err := patterns.Decode(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decode(%q) err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(patterns) != len(tt.want) {
				t.Fatalf("len=%d, want %d", len(patterns), len(tt.want))
			}
			for i, pattern := range patterns {
				if pattern.String() != tt.want[i] {
					t.Errorf("pattern[%d]=%q, want %q", i, pattern.String(), tt.want[i])
				}
			}
			if got := patterns.String(); got != strings.Join(tt.want, ";") {
				t.Errorf("String()=%q, want %q", got, strings.Join(tt.want, ";"))
			}
		})
	}
}