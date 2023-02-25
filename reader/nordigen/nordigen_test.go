package nordigen

import (
	"testing"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

func TestTransactionToYnabber(t *testing.T) {

	var defaultConfig ynabber.Config
	_ = envconfig.Process("", &defaultConfig)

	type args struct {
		cfg     ynabber.Config
		account ynabber.Account
		t       nordigen.Transaction
	}
	tests := []struct {
		name    string
		args    args
		want    ynabber.Transaction
		wantErr bool
	}{
		{name: "milliunits a",
			args: args{cfg: ynabber.Config{}, account: ynabber.Account{}, t: nordigen.Transaction{
				BookingDate: "0001-01-01",
				TransactionAmount: struct {
					Amount   string "json:\"amount,omitempty\""
					Currency string "json:\"currency,omitempty\""
				}{
					Amount: "328.18",
				},
			}},
			want: ynabber.Transaction{
				Amount: ynabber.Milliunits(328180),
			},
			wantErr: false,
		},
		{name: "milliunits b",
			args: args{cfg: ynabber.Config{}, account: ynabber.Account{}, t: nordigen.Transaction{
				BookingDate: "0001-01-01",
				TransactionAmount: struct {
					Amount   string "json:\"amount,omitempty\""
					Currency string "json:\"currency,omitempty\""
				}{
					Amount: "32818",
				},
			}},
			want: ynabber.Transaction{
				Amount: ynabber.Milliunits(32818000),
			},
			wantErr: false,
		},
		{
			// Tests a common Nordigen transaction from NORDEA_NDEADKKK with the
			// default config to highlight any breaking changes.
			name: "NORDEA_NDEADKKK",
			args: args{
				cfg:     defaultConfig,
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: nordigen.Transaction{
					BookingDate:                       "0001-01-01",
					RemittanceInformationUnstructured: "Im unstructured and non-alphanumeric 99",
					TransactionAmount: struct {
						Amount   string "json:\"amount,omitempty\""
						Currency string "json:\"currency,omitempty\""
					}{
						Amount: "10",
					},
				},
			},
			want: ynabber.Transaction{
				Account: ynabber.Account{Name: "foo", IBAN: "bar"},
				Payee:   "Im unstructured and non alphanumeric",
				Memo:    "Im unstructured and non-alphanumeric 99",
				Amount:  ynabber.Milliunits(10000),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transactionToYnabber(tt.args.cfg, tt.args.account, tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %+v, wantErr %+v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got = \n%+v, want \n%+v", got, tt.want)
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

func TestPayeeStrip(t *testing.T) {
	type args struct {
		payee  string
		strips []string
	}
	tests := []struct {
		name  string
		args  args
		wantX string
	}{
		{name: "single",
			args:  args{payee: "Im not here", strips: []string{"not "}},
			wantX: "Im here",
		},
		{name: "multiple",
			args:  args{payee: "Im not really here", strips: []string{"not ", "really "}},
			wantX: "Im here",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotX := payeeStrip(tt.args.payee, tt.args.strips); gotX != tt.wantX {
				t.Errorf("payeeStrip() = %v, want %v", gotX, tt.wantX)
			}
		})
	}
}
