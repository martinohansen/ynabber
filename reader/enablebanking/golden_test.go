package enablebanking

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/martinohansen/ynabber"
)

func TestTransactionGolden(t *testing.T) {
	fixture, err := os.ReadFile("testdata/transactions.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var response TransactionsResponse
	if err := json.Unmarshal(fixture, &response); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	reader := Reader{}
	account := AccountInfo{
		UID:         "fixture-session-account",
		AccountID:   AccountID{IBAN: "NO0000000000001"},
		DisplayName: "Fixture current account",
	}
	got := make([]ynabber.Transaction, 0, len(response.Transactions))
	for i, transaction := range response.Transactions {
		mapped, err := reader.Mapper(account, transaction)
		if err != nil {
			t.Fatalf("map fixture transaction %d: %v", i, err)
		}
		if mapped == nil {
			t.Fatalf("map fixture transaction %d: got nil", i)
		}
		got = append(got, *mapped)
	}

	canonicalAccount := ynabber.Account{
		ID:   "fixture-session-account",
		Name: "Fixture current account",
		IBAN: "NO0000000000001",
	}
	want := []ynabber.Transaction{
		{
			Account: canonicalAccount,
			ID:      "fixture-credit-001",
			Date:    time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC),
			Payee:   "Monthly salary",
			Memo:    "Monthly salary",
			Amount:  1250500,
		},
		{
			Account: canonicalAccount,
			ID:      "fixture-debit-001",
			Date:    time.Date(2024, time.February, 2, 0, 0, 0, 0, time.UTC),
			Payee:   "Example Grocer",
			Memo:    "Card purchase",
			Amount:  -27450,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("canonical transactions mismatch (-want +got):\n%s", diff)
	}
}
