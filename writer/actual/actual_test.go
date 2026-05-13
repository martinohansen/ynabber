package actual

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/writer/actual/client"
)

type fakeClient struct {
	calls        []fakeCall
	err          error
	errByAccount map[string]error
}

type fakeCall struct {
	budgetID     string
	accountID    string
	transactions []client.Transaction
	options      client.ImportTransactionsOptions
}

func (f *fakeClient) ImportTransactions(ctx context.Context, budgetID, accountID string, transactions []client.Transaction, opts client.ImportTransactionsOptions) (client.ImportTransactionsResult, error) {
	f.calls = append(f.calls, fakeCall{budgetID: budgetID, accountID: accountID, transactions: transactions, options: opts})
	// errByAccount takes precedence over err when present.
	if err := f.errByAccount[accountID]; err != nil {
		return client.ImportTransactionsResult{}, err
	}
	return client.ImportTransactionsResult{}, f.err
}

func TestMakeID(t *testing.T) {
	type args struct {
		t ynabber.Transaction
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "with source transaction ID",
			args: args{
				ynabber.Transaction{
					ID:     "txn-123",
					Date:   time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
					Amount: ynabber.Milliunits(1000),
					Account: ynabber.Account{
						IBAN: "NO1234567890",
					},
				},
			},
			want: "YA:05cb693d9c83e7c19683c30f7ec19",
		},
		{
			name: "with IBAN",
			args: args{
				ynabber.Transaction{
					Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
					Account: ynabber.Account{
						IBAN: "NO1234567890",
					},
				},
			},
			want: "YA:0672e63551eba3e6f8ec889059d05",
		},
		{
			name: "with Account ID and IBAN (IBAN preferred)",
			args: args{
				ynabber.Transaction{
					Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
					Account: ynabber.Account{
						ID:   "account-uid-123",
						IBAN: "NO1234567890",
					},
				},
			},
			want: "YA:0672e63551eba3e6f8ec889059d05",
		},
		{
			name: "with Account ID only (no IBAN)",
			args: args{
				ynabber.Transaction{
					Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
					Account: ynabber.Account{
						ID: "account-uid-123",
					},
				},
			},
			want: "YA:21b255f7d0f758109c4309494e17b",
		},
		{
			name: "without ID or IBAN",
			args: args{
				ynabber.Transaction{Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC)},
			},
			want: "YA:80631d08d2ced9968ba40ef39bb1d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeID(tt.args.t)
			if len(got) > maxIDSize {
				t.Errorf("makeID() = %v chars long, max length is %v", len(got), maxIDSize)
			}
			if got != tt.want {
				t.Errorf("makeID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeIDIsSensitiveToIDAndAmount(t *testing.T) {
	base := ynabber.Transaction{
		Date:   time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
		ID:     "txn-123",
		Amount: ynabber.Milliunits(1000),
		Account: ynabber.Account{
			IBAN: "NO1234567890",
		},
	}

	baseID := makeID(base)

	withDifferentID := base
	withDifferentID.ID = "txn-456"
	if makeID(withDifferentID) == baseID {
		t.Error("makeID should be sensitive to ID")
	}

	withDifferentAmount := base
	withDifferentAmount.Amount = ynabber.Milliunits(2000)
	if makeID(withDifferentAmount) == baseID {
		t.Error("makeID should be sensitive to Amount")
	}
}

func TestMakeIDWithoutSourceIDUsesPayeeAndMemo(t *testing.T) {
	base := ynabber.Transaction{
		Date:   time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
		Amount: ynabber.Milliunits(1000),
		Payee:  "Payee 1",
		Memo:   "Memo 1",
		Account: ynabber.Account{
			IBAN: "NO1234567890",
		},
	}

	withDifferentPayee := base
	withDifferentPayee.Payee = "Payee 2"
	if makeID(withDifferentPayee) == makeID(base) {
		t.Fatal("makeID should use payee when transaction ID is empty")
	}

	withDifferentMemo := base
	withDifferentMemo.Memo = "Memo 2"
	if makeID(withDifferentMemo) == makeID(base) {
		t.Fatal("makeID should use memo when transaction ID is empty")
	}
}

func TestAccountParser(t *testing.T) {
	type args struct {
		account    ynabber.Account
		accountMap map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "match by ID (enablebanking)",
			args: args{
				account:    ynabber.Account{ID: "account-uid-123", IBAN: "NO1234567890"},
				accountMap: map[string]string{"account-uid-123": "A1"},
			},
			want:    "A1",
			wantErr: false,
		},
		{
			name: "match by IBAN when ID not in map (nordigen)",
			args: args{
				account:    ynabber.Account{ID: "nordigen-id-456", IBAN: "NO9876543210"},
				accountMap: map[string]string{"NO9876543210": "A2"},
			},
			want:    "A2",
			wantErr: false,
		},
		{
			name: "ID takes precedence over IBAN",
			args: args{
				account:    ynabber.Account{ID: "account-uid-789", IBAN: "NO1234567890"},
				accountMap: map[string]string{"account-uid-789": "A1", "NO1234567890": "A2"},
			},
			want:    "A1",
			wantErr: false,
		},
		{
			name: "no match",
			args: args{
				account:    ynabber.Account{ID: "unknown-id", IBAN: "NO9999999999"},
				accountMap: map[string]string{"foo": "bar"},
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := accountParser(tt.args.account, tt.args.accountMap)
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
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	b := strings.Repeat("p", maxPayeeSize+1)
	m := strings.Repeat("m", maxMemoSize+1)

	tests := []struct {
		name               string
		input              ynabber.Transaction
		wantPayee          string
		wantMemo           string
		wantImportedPayee  string
		wantTruncatedPayee bool
		wantTruncatedMemo  bool
		wantErr            bool
	}{
		{
			name: "default happy path",
			input: ynabber.Transaction{
				Account: ynabber.Account{IBAN: "IBAN1"},
				ID:      "id-1",
				Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
				Payee:   "Grocery Store",
				Memo:    "some memo",
				Amount:  ynabber.Milliunits(12340),
			},
			wantPayee:         "Grocery Store",
			wantMemo:          "some memo",
			wantImportedPayee: "some memo",
		},
		{
			name: "whitespace normalisation",
			input: ynabber.Transaction{
				Account: ynabber.Account{IBAN: "IBAN1"},
				ID:      "id-1",
				Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
				Payee:   "  Grocery   Store  ",
				Memo:    "some   extra\t spaces",
				Amount:  ynabber.Milliunits(12340),
			},
			wantPayee:         "Grocery Store",
			wantMemo:          "some extra spaces",
			wantImportedPayee: "some extra spaces",
		},
		{
			name: "truncation",
			input: ynabber.Transaction{
				Account: ynabber.Account{IBAN: "IBAN1"},
				ID:      "id-1",
				Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
				Payee:   b,
				Memo:    m,
				Amount:  ynabber.Milliunits(1000),
			},
			wantTruncatedPayee: true,
			wantTruncatedMemo:  true,
			wantImportedPayee:  strings.Repeat("m", maxPayeeSize),
		},
		{
			name: "unknown account returns error",
			input: ynabber.Transaction{
				Account: ynabber.Account{IBAN: "UNKNOWN"},
				ID:      "id-x",
				Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
				Payee:   "any",
				Amount:  ynabber.Milliunits(1000),
			},
			wantErr: true,
		},
		{
			name: "imported payee falls back to payee when memo is empty",
			input: ynabber.Transaction{
				Account: ynabber.Account{IBAN: "IBAN1"},
				ID:      "id-1",
				Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
				Payee:   "Grocery Store",
				Memo:    "",
				Amount:  ynabber.Milliunits(12340),
			},
			wantPayee:         "Grocery Store",
			wantMemo:          "",
			wantImportedPayee: "Grocery Store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, accountID, err := writer.toActual(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("toActual() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if accountID != "account-1" {
				t.Fatalf("expected account-1, got %s", accountID)
			}
			expectedAmount := int64(tt.input.Amount) / 10
			if payload.Amount != expectedAmount {
				t.Fatalf("expected amount %d, got %d", expectedAmount, payload.Amount)
			}
			if tt.wantPayee != "" && payload.PayeeName != tt.wantPayee {
				t.Fatalf("unexpected payee name %q, want %q", payload.PayeeName, tt.wantPayee)
			}
			if tt.wantMemo != "" && payload.Notes != tt.wantMemo {
				t.Fatalf("unexpected notes %q, want %q", payload.Notes, tt.wantMemo)
			}
			if tt.wantImportedPayee != "" && payload.ImportedPayee != tt.wantImportedPayee {
				t.Fatalf("unexpected imported payee %q, want %q", payload.ImportedPayee, tt.wantImportedPayee)
			}
			if payload.ImportedID == "" {
				t.Fatalf("expected imported id to be set")
			}
			if payload.Cleared != nil {
				t.Fatalf("expected per-transaction Cleared to be nil so DefaultCleared applies, got %v", *payload.Cleared)
			}
			if tt.wantTruncatedPayee && len([]rune(payload.PayeeName)) != maxPayeeSize {
				t.Fatalf("expected payee truncated to %d, got %d", maxPayeeSize, len([]rune(payload.PayeeName)))
			}
			if tt.wantTruncatedMemo && len([]rune(payload.Notes)) != maxMemoSize {
				t.Fatalf("expected memo truncated to %d, got %d", maxMemoSize, len([]rune(payload.Notes)))
			}
		})
	}
}

func TestBulkGroupsTransactionsByAccount(t *testing.T) {
	fc := &fakeClient{}

	cfg := Config{
		BudgetID:   "budget-1",
		AccountMap: AccountMap{"IBAN1": "account-1", "IBAN2": "account-2"},
		Cleared:    true,
	}

	writer := Writer{
		Config: cfg,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
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

	if len(fc.calls) != 2 {
		t.Fatalf("expected 2 client calls, got %d", len(fc.calls))
	}

	for _, call := range fc.calls {
		if call.budgetID != "budget-1" {
			t.Fatalf("unexpected budget ID %s", call.budgetID)
		}
		if !call.options.DefaultCleared {
			t.Fatalf("expected default cleared option")
		}
		if call.options.ReimportDeleted {
			t.Fatalf("expected reimport deleted to default false")
		}
		if len(call.transactions) != 1 {
			t.Fatalf("expected a single transaction per call")
		}
	}
}

func TestBulkSkipsMappingErrorsAndSendsValid(t *testing.T) {
	fc := &fakeClient{}

	writer := Writer{
		Config: Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	txns := []ynabber.Transaction{
		{
			Account: ynabber.Account{IBAN: "IBAN1"},
			ID:      "1",
			Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			Amount:  ynabber.Milliunits(1000),
		},
		{
			Account: ynabber.Account{IBAN: "UNKNOWN"},
			ID:      "2",
			Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			Amount:  ynabber.Milliunits(1000),
		},
	}

	if err := writer.Bulk(context.Background(), txns); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("expected 1 client call for valid transaction, got %d", len(fc.calls))
	}
	if fc.calls[0].accountID != "account-1" {
		t.Fatalf("expected call to account-1, got %s", fc.calls[0].accountID)
	}
	if len(fc.calls[0].transactions) != 1 {
		t.Fatalf("expected 1 transaction in call, got %d", len(fc.calls[0].transactions))
	}
}

func TestBulkReturnsClientError(t *testing.T) {
	fc := &fakeClient{err: fmt.Errorf("boom")}

	cfg := Config{
		BudgetID:   "budget-1",
		AccountMap: AccountMap{"IBAN1": "account-1"},
	}

	writer := Writer{
		Config: cfg,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
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

func TestBulkAttemptsAllAccountsWhenOneImportFails(t *testing.T) {
	fc := &fakeClient{errByAccount: map[string]error{"account-1": fmt.Errorf("boom")}}

	writer := Writer{
		Config: Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1", "IBAN2": "account-2"},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	txns := []ynabber.Transaction{
		{
			Account: ynabber.Account{IBAN: "IBAN1"},
			ID:      "1",
			Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			Amount:  ynabber.Milliunits(1000),
		},
		{
			Account: ynabber.Account{IBAN: "IBAN2"},
			ID:      "2",
			Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			Amount:  ynabber.Milliunits(2000),
		},
	}

	if err := writer.Bulk(context.Background(), txns); err == nil {
		t.Fatalf("expected import error")
	}
	if len(fc.calls) != 2 {
		t.Fatalf("expected both accounts to be attempted, got %d calls", len(fc.calls))
	}
	if fc.calls[0].accountID != "account-1" || fc.calls[1].accountID != "account-2" {
		t.Fatalf("expected deterministic account order, got %s then %s", fc.calls[0].accountID, fc.calls[1].accountID)
	}
}

func TestBulkPassesDryRunOption(t *testing.T) {
	fc := &fakeClient{}

	writer := Writer{
		Config: Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
			DryRun:     true,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	txns := []ynabber.Transaction{
		{
			Account: ynabber.Account{IBAN: "IBAN1"},
			ID:      "1",
			Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
			Amount:  ynabber.Milliunits(1000),
		},
	}

	if err := writer.Bulk(context.Background(), txns); err != nil {
		t.Fatalf("Bulk() error = %v", err)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fc.calls))
	}
	if !fc.calls[0].options.DryRun {
		t.Fatalf("expected DryRun to be true")
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

	if writer.isDateAllowed(time.Time{}) {
		t.Fatalf("expected zero date to be rejected")
	}

	if !writer.isDateAllowed(time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected date exactly on FromDate to be allowed (inclusive boundary)")
	}
}

func TestIsDateAllowedRejectsFutureDateWithZeroDelay(t *testing.T) {
	writer := Writer{
		Config: Config{Delay: 0},
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
	}

	if writer.isDateAllowed(time.Date(2024, 5, 21, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected future date to be rejected with Delay=0")
	}

	if !writer.isDateAllowed(time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected date equal to now to be allowed with Delay=0")
	}

	if !writer.isDateAllowed(time.Date(2024, 5, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected past date to be allowed with Delay=0")
	}
}

func TestBulkNoTransactions(t *testing.T) {
	fc := &fakeClient{}

	writer := Writer{
		Config: Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    time.Now,
		client: fc,
	}

	if err := writer.Bulk(context.Background(), nil); err != nil {
		t.Fatalf("Bulk(nil) error = %v", err)
	}
	if len(fc.calls) != 0 {
		t.Fatalf("expected no client calls, got %d", len(fc.calls))
	}
}

func TestBulkAllFiltered(t *testing.T) {
	fc := &fakeClient{}

	writer := Writer{
		Config: Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	txns := []ynabber.Transaction{
		{
			Account: ynabber.Account{IBAN: "IBAN1"},
			ID:      "1",
			Date:    time.Time{},
			Amount:  ynabber.Milliunits(1000),
		},
	}

	if err := writer.Bulk(context.Background(), txns); err != nil {
		t.Fatalf("Bulk() error = %v", err)
	}
	if len(fc.calls) != 0 {
		t.Fatalf("expected no client calls when all filtered, got %d", len(fc.calls))
	}
}
