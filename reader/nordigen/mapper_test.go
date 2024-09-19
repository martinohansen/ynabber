package nordigen

import (
	"fmt"
	"testing"

	"github.com/frieser/nordigen-go-lib/v2"
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		transaction nordigen.Transaction
		want        float64
		wantErr     bool
	}{
		{
			transaction: nordigen.Transaction{
				TransactionAmount: struct {
					Amount   string "json:\"amount,omitempty\""
					Currency string "json:\"currency,omitempty\""
				}{Amount: "328.18"},
			},
			want:    328.18,
			wantErr: false,
		},
		{
			transaction: nordigen.Transaction{
				TransactionAmount: struct {
					Amount   string "json:\"amount,omitempty\""
					Currency string "json:\"currency,omitempty\""
				}{Amount: "32818"},
			},
			want:    32818,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Amount: %s", tt.transaction.TransactionAmount.Amount), func(t *testing.T) {
			got, err := parseAmount(tt.transaction)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPayeeStripNonAlphanumeric(t *testing.T) {
	want := "Im just alphanumeric"
	got := payeeStripNonAlphanumeric("Im just alphanumeric")
	if want != got {
		t.Fatalf("alphanumeric: %s != %s", want, got)
	}

	want = "你好世界"
	got = payeeStripNonAlphanumeric("你好世界")
	if want != got {
		t.Fatalf("non-english: %s != %s", want, got)
	}

	want = "Im not j ust alphanumeric"
	got = payeeStripNonAlphanumeric("Im! not j.ust alphanumeric42 69")
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
