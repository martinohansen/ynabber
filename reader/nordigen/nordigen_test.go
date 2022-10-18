package nordigen

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

func TestTransactionsToYnabber(t *testing.T) {
	var dummy_transactions nordigen.AccountTransactions

	file, err := os.ReadFile("testdata/nordigen-transactions.json")
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal([]byte(file), &dummy_transactions)

	var cfg ynabber.Config
	cfg.Nordigen.PayeeSource = append(cfg.Nordigen.PayeeSource, "unstructured")

	parsed, err := transactionsToYnabber(cfg, ynabber.Account{}, dummy_transactions)
	if err != nil {
		t.Fatal(err)
	}

	want := ynabber.Milliunits(328180)
	got := parsed[0].Amount
	if got != want {
		t.Fatalf("failed to parse amount: %s != %s", got, want)
	}

	want = ynabber.Milliunits(32818000)
	got = parsed[1].Amount
	if got != want {
		t.Fatalf("failed to parse amount: %s != %s", got, want)
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
				t.Errorf("accountParser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Name != tt.args.account {
				t.Errorf("accountParser() = %v, want %v", got.Name, tt.args.account)
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
