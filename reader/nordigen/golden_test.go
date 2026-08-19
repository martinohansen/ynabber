package nordigen

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

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

	want := []ynabber.Transaction{
		{
			Account: account,
			ID:      "fixture-credit-20240115-001",
			Date:    time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC),
			Payee:   "Monthly salary",
			Memo:    "Monthly salary",
			Amount:  125750,
		},
		{
			Account: account,
			ID:      "fixture-debit-20240116-001",
			Date:    time.Date(2024, time.January, 16, 0, 0, 0, 0, time.UTC),
			Payee:   "Example Grocer",
			Memo:    "Example Grocer",
			Amount:  -27450,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("canonical transactions mismatch (-want +got):\n%s", diff)
	}
}
