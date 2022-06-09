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

	parsed, err := transactionsToYnabber(ynabber.Account{}, dummy_transactions)
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
