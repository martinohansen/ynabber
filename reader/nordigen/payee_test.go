package nordigen

import (
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
