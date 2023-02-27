package ynabber

import (
	"testing"
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
}

func TestPayee_Strip(t *testing.T) {
	type args struct {
		s []string
	}
	tests := []struct {
		name string
		p    Payee
		args args
		want Payee
	}{
		{name: "single",
			p:    Payee("Im not here"),
			args: args{s: []string{"not "}},
			want: "Im here",
		},
		{name: "multiple",
			p:    Payee("Im not really here"),
			args: args{s: []string{"not ", "really "}},
			want: "Im here",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Strip(tt.args.s); got != tt.want {
				t.Errorf("Payee.Strip() = %v, want %v", got, tt.want)
			}
		})
	}
}
