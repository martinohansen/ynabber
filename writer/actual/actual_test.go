package actual

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
	client "github.com/martinohansen/ynabber/writer/actual/actual-go"
)

type fakeClient struct {
	calls []fakeCall
	err   error
}

type fakeCall struct {
	budgetID     string
	accountID    string
	transactions []client.Transaction
	opts         client.Options
}

func (f *fakeClient) BatchTransactions(ctx context.Context, budgetID, accountID string, transactions []client.Transaction, opts client.Options) error {
	f.calls = append(f.calls, fakeCall{budgetID: budgetID, accountID: accountID, transactions: transactions, opts: opts})
	return f.err
}

func TestToActualAmount(t *testing.T) {
	tests := []struct {
		name    string
		input   ynabber.Milliunits
		want    int64
		wantErr bool
	}{
		{name: "zero", input: 0, want: 0},
		{name: "simple", input: ynabber.Milliunits(12340), want: 1234},
		{name: "negative", input: ynabber.Milliunits(-1000), want: -100},
		{name: "invalid", input: ynabber.Milliunits(5), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toActualAmount(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("toActualAmount() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("toActualAmount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWriterToActual(t *testing.T) {
	cfg := Config{
		AccountMap: AccountMap{"IBAN1": "account-1"},
		Cleared:    true,
	}
	writer := Writer{
		Config: cfg,
		now:    time.Now,
	}

	txn := ynabber.Transaction{
		Account: ynabber.Account{IBAN: "IBAN1"},
		ID:      "id-1",
		Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
		Payee:   "  Grocery Store  ",
		Memo:    "  some memo  ",
		Amount:  ynabber.Milliunits(12340),
	}

	payload, accountID, err := writer.toActual(txn)
	if err != nil {
		t.Fatalf("toActual() error = %v", err)
	}
	if accountID != "account-1" {
		t.Fatalf("expected account-1, got %s", accountID)
	}
	if payload.Amount != 1234 {
		t.Fatalf("expected amount 1234, got %d", payload.Amount)
	}
	if payload.PayeeName != "  Grocery Store  " {
		t.Fatalf("unexpected payee name %q", payload.PayeeName)
	}
	if payload.Notes != "  some memo  " {
		t.Fatalf("unexpected notes %q", payload.Notes)
	}
	if payload.ImportedPayee != "  Grocery Store  " {
		t.Fatalf("unexpected imported payee %q", payload.ImportedPayee)
	}
	if payload.ImportedID != "id-1" {
		t.Fatalf("unexpected imported id %q", payload.ImportedID)
	}
	if payload.Cleared == nil || !*payload.Cleared {
		t.Fatalf("expected cleared to be true")
	}
}

func TestBulkGroupsTransactionsByAccount(t *testing.T) {
	client := &fakeClient{}

	cfg := Config{
		BudgetID:        "budget-1",
		RunTransfers:    true,
		LearnCategories: true,
		AccountMap: AccountMap{
			"IBAN1": "account-1",
			"IBAN2": "account-2",
		},
	}

	writer := Writer{
		Config: cfg,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: client,
	}

	txns := []ynabber.Transaction{
		{
			Account: ynabber.Account{IBAN: "IBAN1"},
			ID:      "1",
			Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			Payee:   "Payee 1",
			Amount:  ynabber.Milliunits(1000),
		},
		{
			Account: ynabber.Account{IBAN: "IBAN2"},
			ID:      "2",
			Date:    time.Date(2024, 5, 11, 0, 0, 0, 0, time.UTC),
			Payee:   "Payee 2",
			Amount:  ynabber.Milliunits(2000),
		},
	}

	if err := writer.Bulk(context.Background(), txns); err != nil {
		t.Fatalf("Bulk() error = %v", err)
	}

	if len(client.calls) != 2 {
		t.Fatalf("expected 2 client calls, got %d", len(client.calls))
	}

	for _, call := range client.calls {
		if call.budgetID != "budget-1" {
			t.Fatalf("unexpected budget ID %s", call.budgetID)
		}
		if !call.opts.RunTransfers || !call.opts.LearnCategories {
			t.Fatalf("expected options to be true")
		}
		if len(call.transactions) != 1 {
			t.Fatalf("expected a single transaction per call")
		}
	}
}

func TestBulkReturnsClientError(t *testing.T) {
	client := &fakeClient{err: fmt.Errorf("boom")}

	cfg := Config{
		BudgetID:     "budget-1",
		AccountMap:   AccountMap{"IBAN1": "account-1"},
		RunTransfers: true,
	}

	writer := Writer{
		Config: cfg,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: client,
	}

	txns := []ynabber.Transaction{
		{
			Account: ynabber.Account{IBAN: "IBAN1"},
			ID:      "1",
			Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			Payee:   "Payee 1",
			Amount:  ynabber.Milliunits(1000),
		},
	}

	if err := writer.Bulk(context.Background(), txns); err == nil {
		t.Fatalf("expected error from client")
	}
}

func TestIsDateAllowed(t *testing.T) {
	writer := Writer{
		Config: Config{
			FromDate: Date(time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)),
			Delay:    48 * time.Hour,
		},
		now: func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
	}

	allowed := writer.isDateAllowed(time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC))
	if !allowed {
		t.Fatalf("expected date to be allowed")
	}

	if writer.isDateAllowed(time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected date before FromDate to be rejected")
	}

	if writer.isDateAllowed(time.Date(2024, 5, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected recent date within delay to be rejected")
	}
}
