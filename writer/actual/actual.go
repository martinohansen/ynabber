package actual

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/writer/actual/client"
)

const maxMemoSize int = 200  // Max size of notes field
const maxPayeeSize int = 200 // Max size of payee_name field
const maxIDSize int = 32     // Max size of imported_id field

var space = regexp.MustCompile(`\s+`)

type importer interface {
	ImportTransactions(ctx context.Context, budgetID, accountID string, transactions []client.Transaction, opts client.ImportTransactionsOptions) (client.ImportTransactionsResult, error)
}

// Writer sends ynabber transactions to Actual Budget.
type Writer struct {
	Config Config
	logger *slog.Logger
	now    func() time.Time
	client importer
}

// String returns the name of the writer.
func (w Writer) String() string {
	return "actual"
}

// NewWriter returns a new Actual writer.
func NewWriter() (Writer, error) {
	cfg := Config{}
	if err := envconfig.Process("", &cfg); err != nil {
		return Writer{}, fmt.Errorf("processing config: %w", err)
	}

	if cfg.BaseURL == "" {
		return Writer{}, errors.New("ACTUAL_BASE_URL is required")
	}
	if cfg.BudgetID == "" {
		return Writer{}, errors.New("ACTUAL_BUDGET_ID is required")
	}
	if len(cfg.AccountMap) == 0 {
		return Writer{}, errors.New("ACTUAL_ACCOUNTMAP is required")
	}
	if cfg.MaxRequestBytes <= 0 {
		return Writer{}, errors.New("ACTUAL_MAX_REQUEST_BYTES must be greater than zero")
	}
	if cfg.BatchSize <= 0 {
		return Writer{}, errors.New("ACTUAL_BATCH_SIZE must be greater than zero")
	}

	logger := slog.Default().With("writer", "actual", "budget_id", cfg.BudgetID)
	c := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.EncryptionPassword, &http.Client{Timeout: 30 * time.Second}, logger)

	return Writer{
		Config: cfg,
		logger: logger,
		now:    time.Now,
		client: c,
	}, nil
}

// importStats is the funnel a Bulk call moves transactions through, from the
// ones handed to it down to the ones Actual confirmed. Keeping it in one type
// means the summary is defined in a single place rather than reassembled from
// counters spread across the import.
type importStats struct {
	accounts     int // accounts with at least one eligible transaction
	eligible     int // mapped successfully and queued for import
	attempted    int // included in a request that was sent
	processed    int // included in a request that succeeded
	added        int // reported as newly created by Actual
	updated      int // reported as reconciled by Actual
	skipped      int // outside the configured date range
	failed       int // could not be mapped
	importErrors int // accounts that reported a planning or request failure
}

// unattempted reports eligible transactions that no request covered, either
// because planning failed for their account or because an earlier batch
// stopped it.
func (s importStats) unattempted() int { return s.eligible - s.attempted }

// add folds one account's outcome into the run totals.
func (s *importStats) add(res accountResult) {
	s.attempted += res.attempted
	s.processed += res.processed
	s.added += res.added
	s.updated += res.updated
}

// LogValue implements slog.LogValuer so the summary is emitted as one group
// instead of a dozen loose attributes.
func (s importStats) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("accounts", s.accounts),
		slog.Int("eligible", s.eligible),
		slog.Int("attempted", s.attempted),
		slog.Int("processed", s.processed),
		slog.Int("unattempted", s.unattempted()),
		slog.Int("added", s.added),
		slog.Int("updated", s.updated),
		slog.Int("skipped", s.skipped),
		slog.Int("failed", s.failed),
		slog.Int("import_errors", s.importErrors),
	)
}

// accountResult is what one account's import achieved. The counts describe
// whatever was committed even when err is set, because Actual can report
// changes alongside a failure.
type accountResult struct {
	attempted int
	processed int
	added     int
	updated   int
	err       error
}

// Bulk sends transactions to Actual Budget, grouped by account. Accounts and
// their batches are processed sequentially. If a request fails, earlier
// batches may already be committed; the remaining batches for that account are
// not attempted, but processing continues for other accounts.
func (w Writer) Bulk(ctx context.Context, transactions []ynabber.Transaction) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(transactions) == 0 {
		w.logger.Info("no transactions received")
		return nil
	}

	grouped, stats := w.group(transactions)
	if len(grouped) == 0 {
		w.logger.Info("all transactions filtered out", "skipped", stats.skipped, "failed", stats.failed)
		return nil
	}

	opts := client.ImportTransactionsOptions{
		DefaultCleared:  w.Config.Cleared,
		ReimportDeleted: w.Config.ReimportDeleted,
		DryRun:          w.Config.DryRun,
	}

	var importErrors []error
	// The summary describes what actually reached Actual, which is exactly what
	// an operator needs after an import was cut short. Emit it on every exit
	// path, including a cancelled context.
	defer func() {
		stats.importErrors = len(importErrors)
		w.logger.Info("transaction import summary", "stats", stats, "dry_run", opts.DryRun)
	}()

	for _, accountID := range slices.Sorted(maps.Keys(grouped)) {
		res := w.importAccount(ctx, accountID, grouped[accountID], opts)
		stats.add(res)
		if res.err == nil {
			continue
		}
		// A cancelled context ends the whole run and outranks whatever error
		// the in-flight request happened to report.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		importErrors = append(importErrors, res.err)
	}

	if len(importErrors) > 0 {
		return fmt.Errorf("failed to import into %d Actual account(s): %w", len(importErrors), errors.Join(importErrors...))
	}
	return nil
}

// group filters transactions and maps the survivors into per-account Actual
// payloads. Out-of-range dates and mapping failures are counted rather than
// returned, so a single unmappable transaction cannot stop an import.
func (w Writer) group(transactions []ynabber.Transaction) (map[string][]client.Transaction, importStats) {
	grouped := make(map[string][]client.Transaction)
	var stats importStats

	for _, src := range transactions {
		if !w.isDateAllowed(src.Date) {
			w.logger.Debug("date out of range", "import_id", makeID(src))
			stats.skipped++
			continue
		}

		payload, accountID, err := w.toActual(src)
		if err != nil {
			// Mapping failures are intentionally non-fatal so a bad batch
			// cannot take down the writer. Individual failures are logged.
			w.logger.Error("mapping transaction", "import_id", makeID(src), "error", err)
			stats.failed++
			continue
		}

		grouped[accountID] = append(grouped[accountID], payload)
		stats.eligible++
	}

	stats.accounts = len(grouped)
	return grouped, stats
}

// importAccount plans and sends one account's transactions in order. The first
// failure stops the account: earlier batches may already be committed, and
// retrying them is safe because Actual reconciles by imported_id, but
// continuing past a failure would only hammer an endpoint that just failed.
func (w Writer) importAccount(ctx context.Context, accountID string, payloads []client.Transaction, opts client.ImportTransactionsOptions) accountResult {
	batches, err := batchTransactions(payloads, opts, w.Config.MaxRequestBytes, w.Config.BatchSize)
	if err != nil {
		return accountResult{err: fmt.Errorf("account %s: batching transactions: %w", accountID, err)}
	}
	if opts.DryRun && len(batches) > 1 {
		// Each dry-run request sees the unchanged budget. A real sequential
		// import lets later batches reconcile against earlier committed ones.
		w.logger.Warn(
			"dry run spans multiple request batches; aggregate results may differ from a real import",
			"account_id", accountID,
			"batches", len(batches),
		)
	}

	var res accountResult
	for i, batch := range batches {
		if err := ctx.Err(); err != nil {
			res.err = err
			return res
		}

		started := time.Now()
		res.attempted += len(batch.transactions)
		result, err := w.client.ImportTransactions(ctx, w.Config.BudgetID, accountID, batch.transactions, opts)
		// Actual may report changes alongside an error, so bank the counts
		// before deciding whether the batch succeeded.
		res.added += result.Added
		res.updated += result.Updated

		logArgs := []any{
			"account_id", accountID,
			"batch", i + 1,
			"batches", len(batches),
			"transactions", len(batch.transactions),
			"request_bytes", batch.requestBytes,
			"max_request_bytes", w.Config.MaxRequestBytes,
			"duration", time.Since(started),
			"added", result.Added,
			"updated", result.Updated,
		}
		if err != nil {
			w.logger.Error("sending transaction batch", append(logArgs, "error", err)...)
			res.err = fmt.Errorf("account %s batch %d/%d: %w", accountID, i+1, len(batches), err)
			return res
		}

		res.processed += len(batch.transactions)
		w.logger.Info("sent transaction batch", logArgs...)
	}

	return res
}

// Runner reads batches of transactions from in and writes them using Bulk.
func (w Writer) Runner(ctx context.Context, in <-chan []ynabber.Transaction) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case batch, ok := <-in:
			if !ok {
				return nil
			}
			if err := w.Bulk(ctx, batch); err != nil {
				w.logger.Error("bulk writing transactions", "error", err)
				return err
			}
		}
	}
}

// isDateAllowed checks if a transaction's date is within allowed bounds.
// It rejects zero dates, dates before FromDate (inclusive boundary — a
// transaction on exactly FromDate is allowed, unlike the YNAB writer which
// uses an exclusive boundary), and dates within the Delay window or in the
// future. Future dates are always rejected regardless of whether Delay is
// configured.
func (w Writer) isDateAllowed(date time.Time) bool {
	if date.IsZero() {
		return false
	}

	now := w.now()
	if !w.Config.FromDate.Time().IsZero() && date.Before(w.Config.FromDate.Time()) {
		return false
	}

	if date.After(now.Add(-w.Config.Delay)) {
		return false
	}

	return true
}

// accountParser takes an Account and returns the matching Actual account ID in
// accountMap. It tries to match by ID first (for enablebanking account_uid),
// then by IBAN (for nordigen or enablebanking with IBAN).
func accountParser(account ynabber.Account, accountMap map[string]string) (string, error) {
	if account.ID != "" {
		if actualID, ok := accountMap[string(account.ID)]; ok {
			return actualID, nil
		}
	}

	if account.IBAN != "" {
		if actualID, ok := accountMap[account.IBAN]; ok {
			return actualID, nil
		}
	}

	return "", fmt.Errorf("no matching Actual account for ID=%q IBAN=%s", account.ID, maskIBAN(account.IBAN))
}

// maskIBAN reduces an IBAN to a form that still identifies which account failed
// to map, so a misconfigured ACTUAL_ACCOUNTMAP is diagnosable, without writing
// the full number into logs. An absent IBAN is reported as such to distinguish
// it from a masked one.
func maskIBAN(iban string) string {
	const visible = 4
	switch {
	case iban == "":
		return "<none>"
	case len(iban) <= visible:
		return strings.Repeat("*", len(iban))
	default:
		return strings.Repeat("*", len(iban)-visible) + iban[len(iban)-visible:]
	}
}

// makeID returns a unique import ID to avoid duplicate transactions.
//
// IBAN is preferred when available to produce import IDs that are consistent
// with the YNAB writer's hash order, so users running both writers see stable
// identifiers across budgets.
//
// The hash input uses a NUL byte separator ([]byte{0}) to prevent field
// collisions (unlike an empty separator where "ab"+"cd" == "a"+"bcd").
// The "YA:" prefix denotes "Ynabber Actual" to distinguish from the YNAB
// writer's "YBBR:" prefix. The result is truncated to maxIDSize (32) chars.
func makeID(t ynabber.Transaction) string {
	date := t.Date.Format(time.DateOnly)
	amount := t.Amount.String()
	sourceID := string(t.ID)

	accountIdentifier := t.Account.IBAN
	if accountIdentifier == "" {
		accountIdentifier = string(t.Account.ID)
	}

	parts := [][]byte{[]byte(accountIdentifier)}
	if sourceID != "" {
		parts = append(parts, []byte(sourceID), []byte(date), []byte(amount))
	} else {
		parts = append(parts, []byte(date), []byte(amount), []byte(t.Payee), []byte(t.Memo))
	}
	hash := sha256.Sum256(bytes.Join(parts, []byte{0}))
	return fmt.Sprintf("YA:%x", hash)[:maxIDSize]
}

// toActual converts a ynabber transaction to an Actual transaction.
func (w Writer) toActual(src ynabber.Transaction) (client.Transaction, string, error) {
	accountID, err := accountParser(src.Account, w.Config.AccountMap)
	if err != nil {
		return client.Transaction{}, "", err
	}

	amount, err := toActualAmount(src.Amount)
	if err != nil {
		return client.Transaction{}, "", err
	}

	payee := strings.TrimSpace(space.ReplaceAllString(src.Payee, " "))
	if r := []rune(payee); len(r) > maxPayeeSize {
		w.logger.Warn("payee too long", "import_id", makeID(src), "max_size", maxPayeeSize)
		payee = strings.TrimSpace(string(r[:maxPayeeSize]))
	}

	memo := strings.TrimSpace(space.ReplaceAllString(src.Memo, " "))
	if r := []rune(memo); len(r) > maxMemoSize {
		w.logger.Warn("memo too long", "import_id", makeID(src), "max_size", maxMemoSize)
		memo = strings.TrimSpace(string(r[:maxMemoSize]))
	}

	// imported_payee holds the raw bank text (sourced from Memo) so Actual's
	// payee-renaming rules can match against the full remittance information
	// rather than the already-stripped Payee field.
	importedPayee := memo
	if importedPayee == "" {
		importedPayee = payee
	}

	payload := client.Transaction{
		Account:       accountID,
		Date:          src.Date.Format(time.DateOnly),
		Amount:        amount,
		PayeeName:     payee,
		Notes:         memo,
		ImportedPayee: importedPayee,
		ImportedID:    makeID(src),
	}

	w.logger.Debug("mapped transaction", "import_id", payload.ImportedID, "account_id", accountID)
	return payload, accountID, nil
}

// toActualAmount converts ynabber milliunits to Actual integer cents.
// Returns an error if the amount cannot be represented exactly in cents
// (i.e. is not a multiple of 10 milliunits) — this prevents silent
// rounding errors at the cost of failing a sub-cent transaction.
func toActualAmount(m ynabber.Milliunits) (int64, error) {
	amount := int64(m)
	if amount%10 != 0 {
		return 0, fmt.Errorf("amount %d cannot be represented in cents", amount)
	}
	return amount / 10, nil
}
