package nordigen

import (
	"testing"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

func TestTransactionToYnabber(t *testing.T) {
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
				TransactionId: "00000000-0000-0000-0000-000000000000",
				BookingDate:   "0001-01-01",
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
				TransactionId: "00000000-0000-0000-0000-000000000000",
				BookingDate:   "0001-01-01",
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

func TestAccountParser(t *testing.T) {
	type args struct {
		account    string
		accountMap map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    ynabber.Account
		wantErr bool
	}{
		{name: "match",
			args:    args{account: "N1", accountMap: map[string]string{"N1": "Y1"}},
			want:    ynabber.Account{Name: "Y1"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := accountParser(tt.args.account, tt.args.accountMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Name != tt.args.account {
				t.Errorf("got = %v, want %v", got.Name, tt.args.account)
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
	want := "Im here"
	got := payeeStrip("Im not here", []string{"not "})
	if want != got {
		t.Fatalf("strip words: %s != %s", want, got)
	}

}
