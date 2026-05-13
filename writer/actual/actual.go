package actual

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
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

	logger := slog.Default().With("writer", "actual", "budget_id", cfg.BudgetID)
	c := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.EncryptionPassword, &http.Client{Timeout: 30 * time.Second}, logger)

	return Writer{
		Config: cfg,
		logger: logger,
		now:    time.Now,
		client: c,
	}, nil
}

// Bulk sends a batch of transactions to Actual Budget, grouped by account.
func (w Writer) Bulk(ctx context.Context, transactions []ynabber.Transaction) error {
	if len(transactions) == 0 {
		w.logger.Info("no transactions received")
		return nil
	}

	skipped := 0
	failed := 0

	grouped := make(map[string][]client.Transaction)

	for _, src := range transactions {
		if !w.isDateAllowed(src.Date) {
			w.logger.Debug("date out of range", "transaction", src)
			skipped++
			continue
		}

		payload, accountID, err := w.toActual(src)
		if err != nil {
			// Mapping failures are intentionally non-fatal so a bad batch
			// cannot take down the writer. Individual failures are logged.
			w.logger.Error("mapping transaction", "transaction", src, "error", err)
			failed++
			continue
		}

		grouped[accountID] = append(grouped[accountID], payload)
	}

	if len(grouped) == 0 {
		w.logger.Info("all transactions filtered out", "skipped", skipped, "failed", failed)
		return nil
	}

	accountIDs := make([]string, 0, len(grouped))
	for accountID := range grouped {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Strings(accountIDs)

	opts := client.ImportTransactionsOptions{
		DefaultCleared:  w.Config.Cleared,
		ReimportDeleted: w.Config.ReimportDeleted,
		DryRun:          w.Config.DryRun,
	}
	submitted := 0
	added := 0
	updated := 0
	var importErrors []error
	for _, accountID := range accountIDs {
		payloads := grouped[accountID]
		result, err := w.client.ImportTransactions(ctx, w.Config.BudgetID, accountID, payloads, opts)
		if err != nil {
			importErrors = append(importErrors, fmt.Errorf("account %s: %w", accountID, err))
			continue
		}
		submitted += len(payloads)
		added += result.Added
		updated += result.Updated
	}
	if len(importErrors) > 0 {
		return fmt.Errorf("failed to import into %d Actual account(s): %w", len(importErrors), errors.Join(importErrors...))
	}

	w.logger.Info(
		"sent transactions",
		"accounts", len(grouped),
		"submitted", submitted,
		"added", added,
		"updated", updated,
		"skipped", skipped,
		"failed", failed,
	)
	return nil
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

	return "", fmt.Errorf("no matching Actual account for ID=%q IBAN=%q", account.ID, account.IBAN)
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
		w.logger.Warn("payee too long", "transaction", src, "max_size", maxPayeeSize)
		payee = strings.TrimSpace(string(r[:maxPayeeSize]))
	}

	memo := strings.TrimSpace(space.ReplaceAllString(src.Memo, " "))
	if r := []rune(memo); len(r) > maxMemoSize {
		w.logger.Warn("memo too long", "transaction", src, "max_size", maxMemoSize)
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

	w.logger.Debug("mapped transaction", "from", src, "to", payload)
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
