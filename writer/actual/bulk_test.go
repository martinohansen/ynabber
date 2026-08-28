package actual

import (
	"context"
	"errors"
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
	handleImport func(context.Context, int, fakeCall) (client.ImportTransactionsResult, error)
}

type fakeCall struct {
	budgetID     string
	accountID    string
	transactions []client.Transaction
	options      client.ImportTransactionsOptions
}

func (f *fakeClient) ImportTransactions(ctx context.Context, budgetID, accountID string, transactions []client.Transaction, opts client.ImportTransactionsOptions) (client.ImportTransactionsResult, error) {
	callIndex := len(f.calls)
	call := fakeCall{
		budgetID:     budgetID,
		accountID:    accountID,
		transactions: append([]client.Transaction(nil), transactions...),
		options:      opts,
	}
	f.calls = append(f.calls, call)
	if f.handleImport != nil {
		return f.handleImport(ctx, callIndex, call)
	}
	return client.ImportTransactionsResult{}, nil
}

// withBatchDefaults fills in the batch limits that envconfig supplies at
// runtime so tests can state only the fields under test. Production code never
// relies on this: NewWriter rejects a non-positive limit and batchTransactions
// refuses to plan without one.
func withBatchDefaults(cfg Config) Config {
	if cfg.MaxRequestBytes == 0 {
		cfg.MaxRequestBytes = defaultMaxRequestBytes
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = defaultBatchSize
	}
	return cfg
}

func TestBulkGroupsTransactionsByAccount(t *testing.T) {
	fc := &fakeClient{}

	cfg := withBatchDefaults(Config{
		BudgetID:   "budget-1",
		AccountMap: AccountMap{"IBAN1": "account-1", "IBAN2": "account-2"},
		Cleared:    true,
	})

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
		Config: withBatchDefaults(Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
		}),
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
	fc := &fakeClient{handleImport: func(context.Context, int, fakeCall) (client.ImportTransactionsResult, error) {
		return client.ImportTransactionsResult{}, errors.New("boom")
	}}

	cfg := withBatchDefaults(Config{
		BudgetID:   "budget-1",
		AccountMap: AccountMap{"IBAN1": "account-1"},
	})

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
	fc := &fakeClient{handleImport: func(_ context.Context, _ int, call fakeCall) (client.ImportTransactionsResult, error) {
		if call.accountID == "account-1" {
			return client.ImportTransactionsResult{}, errors.New("boom")
		}
		return client.ImportTransactionsResult{}, nil
	}}

	writer := Writer{
		Config: withBatchDefaults(Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1", "IBAN2": "account-2"},
		}),
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

func TestBulkBatchesAndAggregatesResults(t *testing.T) {
	results := []client.ImportTransactionsResult{
		{Added: 2, Updated: 1},
		{Added: 1, Updated: 1},
	}
	fc := &fakeClient{handleImport: func(_ context.Context, callIndex int, _ fakeCall) (client.ImportTransactionsResult, error) {
		return results[callIndex], nil
	}}
	var logs strings.Builder
	writer := Writer{
		Config: Config{
			BudgetID:        "budget-1",
			AccountMap:      AccountMap{"IBAN1": "account-1"},
			MaxRequestBytes: defaultMaxRequestBytes,
			BatchSize:       2,
		},
		logger: slog.New(slog.NewTextHandler(&logs, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	txns := []ynabber.Transaction{
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "1", Date: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC), Amount: 1000},
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "2", Date: time.Date(2024, 5, 11, 0, 0, 0, 0, time.UTC), Amount: 2000},
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "3", Date: time.Date(2024, 5, 12, 0, 0, 0, 0, time.UTC), Amount: 3000},
	}

	if err := writer.Bulk(context.Background(), txns); err != nil {
		t.Fatalf("Bulk() error = %v", err)
	}
	if len(fc.calls) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(fc.calls))
	}
	if len(fc.calls[0].transactions) != 2 || len(fc.calls[1].transactions) != 1 {
		t.Fatalf("unexpected batch sizes %d and %d", len(fc.calls[0].transactions), len(fc.calls[1].transactions))
	}
	for i, call := range fc.calls {
		requestBytes, err := client.ImportTransactionsRequestSize(call.transactions, call.options)
		if err != nil {
			t.Fatalf("measuring call %d: %v", i+1, err)
		}
		if requestBytes > writer.Config.MaxRequestBytes {
			t.Fatalf("call %d is %d bytes, maximum is %d", i+1, requestBytes, writer.Config.MaxRequestBytes)
		}
	}

	logOutput := logs.String()
	for _, summary := range []string{"eligible=3", "attempted=3", "processed=3", "unattempted=0", "added=3", "updated=2", "import_errors=0"} {
		if !strings.Contains(logOutput, summary) {
			t.Fatalf("log output is missing %q: %s", summary, logOutput)
		}
	}
}

func TestBulkStopsFailedAccountAndContinuesOtherAccounts(t *testing.T) {
	fc := &fakeClient{handleImport: func(_ context.Context, callIndex int, _ fakeCall) (client.ImportTransactionsResult, error) {
		if callIndex == 1 {
			return client.ImportTransactionsResult{}, errors.New("boom")
		}
		return client.ImportTransactionsResult{}, nil
	}}
	writer := Writer{
		Config: Config{
			BudgetID:        "budget-1",
			AccountMap:      AccountMap{"IBAN1": "account-1", "IBAN2": "account-2"},
			MaxRequestBytes: defaultMaxRequestBytes,
			BatchSize:       1,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	txns := []ynabber.Transaction{
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "1", Date: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC), Amount: 1000},
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "2", Date: time.Date(2024, 5, 11, 0, 0, 0, 0, time.UTC), Amount: 2000},
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "3", Date: time.Date(2024, 5, 12, 0, 0, 0, 0, time.UTC), Amount: 3000},
		{Account: ynabber.Account{IBAN: "IBAN2"}, ID: "4", Date: time.Date(2024, 5, 13, 0, 0, 0, 0, time.UTC), Amount: 4000},
	}

	if err := writer.Bulk(context.Background(), txns); err == nil {
		t.Fatal("expected import error")
	}
	if len(fc.calls) != 3 {
		t.Fatalf("expected two attempts for account-1 and one for account-2, got %d", len(fc.calls))
	}
	if fc.calls[0].accountID != "account-1" || fc.calls[1].accountID != "account-1" || fc.calls[2].accountID != "account-2" {
		t.Fatalf("unexpected account call order: %s, %s, %s", fc.calls[0].accountID, fc.calls[1].accountID, fc.calls[2].accountID)
	}
}

func TestBulkStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fc := &fakeClient{handleImport: func(context.Context, int, fakeCall) (client.ImportTransactionsResult, error) {
		cancel()
		return client.ImportTransactionsResult{}, nil
	}}
	writer := Writer{
		Config: Config{
			BudgetID:        "budget-1",
			AccountMap:      AccountMap{"IBAN1": "account-1"},
			MaxRequestBytes: defaultMaxRequestBytes,
			BatchSize:       1,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	txns := []ynabber.Transaction{
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "1", Date: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC), Amount: 1000},
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "2", Date: time.Date(2024, 5, 11, 0, 0, 0, 0, time.UTC), Amount: 2000},
	}

	err := writer.Bulk(ctx, txns)
	if err != context.Canceled {
		t.Fatalf("Bulk() error = %v, want context.Canceled", err)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("expected cancellation after one request, got %d", len(fc.calls))
	}
}

func TestBulkPreCanceledContextMakesNoClientCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fc := &fakeClient{}
	writer := Writer{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    time.Now,
		client: fc,
	}

	err := writer.Bulk(ctx, []ynabber.Transaction{{}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Bulk() error = %v, want context.Canceled", err)
	}
	if len(fc.calls) != 0 {
		t.Fatalf("expected no client calls, got %d", len(fc.calls))
	}
}

func TestBulkPrefersContextErrorWhenClientFailsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	clientErr := errors.New("client failure")
	fc := &fakeClient{handleImport: func(context.Context, int, fakeCall) (client.ImportTransactionsResult, error) {
		cancel()
		return client.ImportTransactionsResult{}, clientErr
	}}
	writer := Writer{
		Config: withBatchDefaults(Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
		}),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	err := writer.Bulk(ctx, []ynabber.Transaction{{
		Account: ynabber.Account{IBAN: "IBAN1"},
		ID:      "1",
		Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
		Amount:  1000,
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Bulk() error = %v, want context.Canceled", err)
	}
	if errors.Is(err, clientErr) {
		t.Fatalf("Bulk() error = %v, want cancellation to take precedence", err)
	}
	if len(fc.calls) != 1 {
		t.Fatalf("expected one client call, got %d", len(fc.calls))
	}
}

func TestBulkReportsPartialImportResult(t *testing.T) {
	importErr := errors.New("partial import")
	fc := &fakeClient{handleImport: func(context.Context, int, fakeCall) (client.ImportTransactionsResult, error) {
		return client.ImportTransactionsResult{Added: 1}, importErr
	}}
	var logs strings.Builder
	writer := Writer{
		Config: withBatchDefaults(Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
		}),
		logger: slog.New(slog.NewTextHandler(&logs, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	err := writer.Bulk(context.Background(), []ynabber.Transaction{{
		Account: ynabber.Account{IBAN: "IBAN1"},
		ID:      "1",
		Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
		Amount:  1000,
	}})
	if !errors.Is(err, importErr) {
		t.Fatalf("Bulk() error = %v, want wrapped import error", err)
	}

	logOutput := logs.String()
	for _, summary := range []string{"eligible=1", "attempted=1", "processed=0", "unattempted=0", "added=1", "import_errors=1"} {
		if !strings.Contains(logOutput, summary) {
			t.Fatalf("log output is missing %q: %s", summary, logOutput)
		}
	}
}

func TestBulkReportsOversizedTransaction(t *testing.T) {
	txn := ynabber.Transaction{
		Account: ynabber.Account{IBAN: "IBAN1"},
		ID:      "1",
		Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
		Memo:    strings.Repeat("x", maxMemoSize),
		Amount:  1000,
	}
	measuringWriter := Writer{
		Config: Config{AccountMap: AccountMap{"IBAN1": "account-1"}},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	payload, _, err := measuringWriter.toActual(txn)
	if err != nil {
		t.Fatalf("mapping fixture: %v", err)
	}
	requestBytes, err := client.ImportTransactionsRequestSize([]client.Transaction{payload}, client.ImportTransactionsOptions{})
	if err != nil {
		t.Fatalf("measuring fixture: %v", err)
	}

	fc := &fakeClient{}
	writer := Writer{
		Config: Config{
			BudgetID:        "budget-1",
			AccountMap:      AccountMap{"IBAN1": "account-1"},
			MaxRequestBytes: requestBytes - 1,
			BatchSize:       100,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	err = writer.Bulk(context.Background(), []ynabber.Transaction{txn})
	if err == nil {
		t.Fatal("expected oversized transaction error")
	}
	for _, want := range []string{"account account-1", fmt.Sprintf("requires %d bytes", requestBytes)} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
	if len(fc.calls) != 0 {
		t.Fatalf("expected no HTTP calls, got %d", len(fc.calls))
	}
}

func TestBulkPassesDryRunOptionAndWarnsAcrossBatches(t *testing.T) {
	fc := &fakeClient{}
	var logs strings.Builder

	writer := Writer{
		Config: withBatchDefaults(Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
			DryRun:     true,
			BatchSize:  1,
		}),
		logger: slog.New(slog.NewTextHandler(&logs, nil)),
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
			Account: ynabber.Account{IBAN: "IBAN1"},
			ID:      "2",
			Date:    time.Date(2024, 5, 11, 0, 0, 0, 0, time.UTC),
			Amount:  ynabber.Milliunits(2000),
		},
	}

	if err := writer.Bulk(context.Background(), txns); err != nil {
		t.Fatalf("Bulk() error = %v", err)
	}
	if len(fc.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(fc.calls))
	}
	for i, call := range fc.calls {
		if !call.options.DryRun {
			t.Fatalf("call %d has DryRun=false", i+1)
		}
	}
	if got := logs.String(); !strings.Contains(got, "aggregate results may differ from a real import") {
		t.Fatalf("expected multi-batch dry-run warning, got: %s", got)
	}
}

// TestBulkLogsSummaryAfterCancellation covers the summary being emitted on
// every exit path. A cancelled import is precisely when an operator needs to
// know how much of it was already committed.
func TestBulkLogsSummaryAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fc := &fakeClient{handleImport: func(context.Context, int, fakeCall) (client.ImportTransactionsResult, error) {
		cancel()
		return client.ImportTransactionsResult{Added: 1}, nil
	}}
	var logs strings.Builder
	writer := Writer{
		Config: withBatchDefaults(Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
			BatchSize:  1,
		}),
		logger: slog.New(slog.NewTextHandler(&logs, nil)),
		now:    func() time.Time { return time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC) },
		client: fc,
	}

	txns := []ynabber.Transaction{
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "1", Date: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC), Amount: 1000},
		{Account: ynabber.Account{IBAN: "IBAN1"}, ID: "2", Date: time.Date(2024, 5, 11, 0, 0, 0, 0, time.UTC), Amount: 2000},
	}

	if err := writer.Bulk(ctx, txns); !errors.Is(err, context.Canceled) {
		t.Fatalf("Bulk() error = %v, want context.Canceled", err)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "transaction import summary") {
		t.Fatalf("cancelled import logged no summary: %s", logOutput)
	}
	// The first batch committed before the cancellation took effect; the second
	// was never attempted.
	for _, want := range []string{"eligible=2", "attempted=1", "processed=1", "unattempted=1", "added=1"} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("summary is missing %q: %s", want, logOutput)
		}
	}
}

// TestBulkRejectsUnconfiguredBatchLimits pins the guard that replaced the
// silent zero-value fallback: a Writer built without batch limits reports the
// misconfiguration instead of quietly importing at a default size.
func TestBulkRejectsUnconfiguredBatchLimits(t *testing.T) {
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

	err := writer.Bulk(context.Background(), []ynabber.Transaction{{
		Account: ynabber.Account{IBAN: "IBAN1"},
		ID:      "1",
		Date:    time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC),
		Amount:  1000,
	}})
	if err == nil {
		t.Fatal("expected an error for unconfigured batch limits")
	}
	if !strings.Contains(err.Error(), "maximum request bytes") {
		t.Fatalf("error = %q, want it to mention the maximum request bytes", err)
	}
	if len(fc.calls) != 0 {
		t.Fatalf("expected no client calls, got %d", len(fc.calls))
	}
}

func TestBulkNoTransactions(t *testing.T) {
	fc := &fakeClient{}

	writer := Writer{
		Config: withBatchDefaults(Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
		}),
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
		Config: withBatchDefaults(Config{
			BudgetID:   "budget-1",
			AccountMap: AccountMap{"IBAN1": "account-1"},
		}),
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
