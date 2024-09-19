package nordigen

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

const rateLimitExceededStatusCode = 429

type Reader struct {
	Config *ynabber.Config
	Client *nordigen.Client
	logger *slog.Logger
}

// NewReader returns a new nordigen reader or panics
func NewReader(cfg *ynabber.Config) Reader {
	client, err := nordigen.NewClient(cfg.Nordigen.SecretID, cfg.Nordigen.SecretKey)
	if err != nil {
		panic("Failed to create nordigen client")
	}

	return Reader{
		Config: cfg,
		Client: client,
		logger: slog.Default().With("reader", "nordigen"),
	}
}

func (r Reader) toYnabbers(a ynabber.Account, t nordigen.AccountTransactions) ([]ynabber.Transaction, error) {
	logger := r.logger.With("account", a.IBAN)

	skipped := 0
	y := []ynabber.Transaction{}
	for _, v := range t.Transactions.Booked {
		transaction, err := r.Mapper(a, v)
		if err != nil {
			return nil, err
		}

		// Append transaction
		if transaction != nil {
			logger.Debug("mapped transaction", "from", v, "to", transaction)
			y = append(y, *transaction)
		} else {
			skipped++
			logger.Debug("skipping", "transaction", v)
		}

	}
	logger.Info("read transactions", "total", len(y)+skipped, "skipped", skipped)
	return y, nil
}

func (r Reader) Bulk() (t []ynabber.Transaction, err error) {
	req, err := r.Requisition()
	if err != nil {
		return nil, fmt.Errorf("failed to authorize: %w", err)
	}

	r.logger.Info("loaded requisition", "accounts", len(req.Accounts))
	for _, account := range req.Accounts {
		accountMetadata, err := r.Client.GetAccountMetadata(account)
		if err != nil {
			return nil, fmt.Errorf("failed to get account metadata: %w", err)
		}
		logger := r.logger.With("iban", accountMetadata.Iban)

		// Handle expired, or suspended accounts by recreating the
		// requisition.
		switch accountMetadata.Status {
		case "EXPIRED", "SUSPENDED":
			logger.Info("recreating requisition", "status", accountMetadata.Status)
			r.createRequisition()
		}

		account := ynabber.Account{
			ID:   ynabber.ID(accountMetadata.Id),
			Name: accountMetadata.Iban,
			IBAN: accountMetadata.Iban,
		}

		transactions, err := r.Client.GetAccountTransactions(string(account.ID))
		if err != nil {
			var apiErr *nordigen.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == rateLimitExceededStatusCode {
				logger.Warn("rate limit exceeded, skipping account")
				continue
			}
			return nil, fmt.Errorf("failed to get transactions: %w", err)
		}

		x, err := r.toYnabbers(account, transactions)
		if err != nil {
			return nil, fmt.Errorf("failed to convert transaction: %w", err)
		}
		t = append(t, x...)
	}
	return t, nil
}
