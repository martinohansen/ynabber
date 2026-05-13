package nordigen

import (
	"regexp"
	"testing"

	"github.com/frieser/nordigen-go-lib/v2"
)

func TestStripNonAlphanumeric(t *testing.T) {
	want := "Im just alphanumeric"
	got := stripNonAlphanumeric("Im just alphanumeric")
	if want != got {
		t.Fatalf("alphanumeric: %s != %s", want, got)
	}

	want = "你好世界"
	got = stripNonAlphanumeric("你好世界")
	if want != got {
		t.Fatalf("non-english: %s != %s", want, got)
	}

	want = "Im not j ust alphanumeric"
	got = stripNonAlphanumeric("Im! not j.ust alphanumeric42 69")
	if want != got {
		t.Fatalf("non-alphanumeric: %s != %s", want, got)
	}
}

func TestStrip(t *testing.T) {
	type args struct {
		s      string
		strips []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "single",
			args: args{s: "Im not here", strips: []string{"not "}},
			want: "Im here",
		},
		{name: "multiple",
			args: args{s: "Im not really here", strips: []string{"not ", "really "}},
			want: "Im here",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strip(tt.args.s, tt.args.strips); got != tt.want {
				t.Errorf("Payee.Strip() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

func TestNewPayee(t *testing.T) {
	type args struct {
		t      nordigen.Transaction
		groups PayeeGroups
	}
	tests := []struct {
		name string
		args args
		want Payee
	}{
		// First source that yields a result should be used
		{
			name: "name,unstructured,additional",
			args: args{
				t: nordigen.Transaction{
					CreditorName:                      "",
					RemittanceInformationUnstructured: "",
					AdditionalInformation:             "baz",
				},
				groups: PayeeGroups{
					{Name},
					{Remittance},
					{Additional},
				},
			},
			want: Payee{
				value: "baz",
				raw:   "baz",
			},
		},
		// The "+" operator should concat fields
		{
			name: "name+unstructured,additional",
			args: args{
				t: nordigen.Transaction{
					CreditorName:                      "foo",
					RemittanceInformationUnstructured: "bar",
					AdditionalInformation:             "baz",
				},
				groups: PayeeGroups{
					{Name, Remittance},
					{Additional},
				},
			},
			want: Payee{
				value: "foo bar",
				raw:   "foo bar",
			},
		},
		// Raw is not transformed
		{
			name: "raw",
			args: args{
				t: nordigen.Transaction{
					RemittanceInformationUnstructured: "Visa køb DKK 54,90 REMA1000 APP 426 F Den 21.05",
				},
				groups: PayeeGroups{
					{Remittance},
				},
			},
			want: Payee{
				value: "Visa køb DKK REMA APP F Den",
				raw:   "Visa køb DKK 54,90 REMA1000 APP 426 F Den 21.05",
			},
		},
		// No sources yields a result
		{
			name: "empty",
			args: args{
				t: nordigen.Transaction{
					CreditorName:                      "",
					RemittanceInformationUnstructured: "",
					AdditionalInformation:             "",
				},
				groups: PayeeGroups{
					{Name},
					{Remittance},
					{Additional},
				},
			},
			want: Payee{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newPayee(tt.args.t, tt.args.groups)
			if got != tt.want {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}
