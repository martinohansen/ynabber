package actual

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	client "github.com/martinohansen/ynabber/writer/actual/actual-go"
)

type actual interface {
	BatchTransactions(ctx context.Context, budgetID, accountID string, transactions []client.Transaction, opts client.Options) error
}

type Writer struct {
	Config Config
	logger *slog.Logger
	now    func() time.Time
	client actual
}

func (w Writer) String() string {
	return "actual"
}

func NewWriter() (Writer, error) {
	cfg := Config{}
	if err := envconfig.Process("", &cfg); err != nil {
		return Writer{}, fmt.Errorf("processing config: %w", err)
	}

	if cfg.BaseURL == "" {
		return Writer{}, fmt.Errorf("ACTUAL_BASE_URL is required")
	}
	if cfg.BudgetID == "" {
		return Writer{}, fmt.Errorf("ACTUAL_BUDGET_ID is required")
	}
	if len(cfg.AccountMap) == 0 {
		return Writer{}, fmt.Errorf("ACTUAL_ACCOUNTMAP is required")
	}

	logger := slog.Default().With("writer", "actual", "budget_id", cfg.BudgetID)
	client := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.EncryptionPassword, &http.Client{Timeout: 15 * time.Second}, logger)

	writer := Writer{
		Config: cfg,
		logger: logger,
		now:    time.Now,
		client: client,
	}

	return writer, nil
}

func (w Writer) Bulk(ctx context.Context, transactions []ynabber.Transaction) error {
	skipped := 0
	failed := 0

	grouped := make(map[string][]client.Transaction)

	for _, src := range transactions {
		if !w.isDateAllowed(src.Date) {
			skipped++
			w.logger.Debug("date out of range", "transaction", src)
			continue
		}

		payload, accountID, err := w.toActual(src)
		if err != nil {
			failed++
			w.logger.Error("mapping transaction", "transaction", src, "error", err)
			continue
		}

		grouped[accountID] = append(grouped[accountID], payload)
	}

	if len(transactions) == 0 || len(grouped) == 0 {
		w.logger.Info("no transactions to write")
		return nil
	}

	total := 0
	for accountID, payloads := range grouped {
		total += len(payloads)
		if err := w.client.BatchTransactions(ctx, w.Config.BudgetID, accountID, payloads, client.Options{
			RunTransfers:    w.Config.RunTransfers,
			LearnCategories: w.Config.LearnCategories,
		}); err != nil {
			return err
		}
	}

	w.logger.Info("sent transactions", "accounts", len(grouped), "transactions", total, "skipped", skipped, "failed", failed)
	return nil
}

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
				if w.logger != nil {
					w.logger.Error("bulk writing transactions", "error", err)
				}
				return err
			}
		}
	}
}

func (w Writer) isDateAllowed(date time.Time) bool {
	if date.IsZero() {
		return false
	}

	now := w.now()
	if !w.Config.FromDate.Time().IsZero() && date.Before(w.Config.FromDate.Time()) {
		return false
	}

	if w.Config.Delay > 0 && date.After(now.Add(-w.Config.Delay)) {
		return false
	}

	return true
}

func (w Writer) toActual(src ynabber.Transaction) (client.Transaction, string, error) {
	if src.Account.IBAN == "" {
		return client.Transaction{}, "", fmt.Errorf("transaction missing account IBAN")
	}

	accountID, ok := w.Config.AccountMap[src.Account.IBAN]
	if !ok {
		return client.Transaction{}, "", fmt.Errorf("no account mapping for %s", src.Account.IBAN)
	}

	amount, err := toActualAmount(src.Amount)
	if err != nil {
		return client.Transaction{}, "", err
	}

	cleared := w.Config.Cleared

	payload := client.Transaction{
		Account:       accountID,
		Date:          src.Date.Format(dateLayout),
		Amount:        amount,
		PayeeName:     src.Payee,
		Notes:         src.Memo,
		ImportedPayee: src.Payee,
		ImportedID:    string(src.ID),
		Cleared:       &cleared,
	}

	return payload, accountID, nil
}

func toActualAmount(m ynabber.Milliunits) (int64, error) {
	amount := int64(m)
	if amount%10 != 0 {
		return 0, fmt.Errorf("amount %d cannot be represented in cents", amount)
	}
	return amount / 10, nil
}
