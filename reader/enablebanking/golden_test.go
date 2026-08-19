package enablebanking

import (
	"encoding/json"
	"os"
	"testing"

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
