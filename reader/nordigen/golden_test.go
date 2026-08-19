package nordigen

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"

	upstream "github.com/frieser/nordigen-go-lib/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/martinohansen/ynabber"
)

func TestTransactionGolden(t *testing.T) {
	fixture, err := os.ReadFile("testdata/transactions.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var response upstream.AccountTransactions
	if err := json.Unmarshal(fixture, &response); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	reader := Reader{
		Config: Config{
			PayeeSource:   PayeeGroups{{Remittance}, {Name}, {Additional}},
			TransactionID: "TransactionId",
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	account := ynabber.Account{
		ID:   "fixture-account",
		Name: "Fixture current account",
		IBAN: "XX000000000000000000",
	}

	got, err := reader.toYnabbers(account, response)
	if err != nil {
		t.Fatalf("map fixture: %v", err)
	}

	wantFixture, err := os.ReadFile("testdata/canonical.json")
	if err != nil {
		t.Fatalf("read canonical fixture: %v", err)
	}

	var want []ynabber.Transaction
	if err := json.Unmarshal(wantFixture, &want); err != nil {
		t.Fatalf("decode canonical fixture: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("canonical transactions mismatch (-want +got):\n%s", diff)
	}
}
