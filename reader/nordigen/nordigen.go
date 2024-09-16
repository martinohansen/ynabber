package nordigen

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

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
		logger: slog.Default().With(
			"reader", "nordigen",
			"bank_id", cfg.Nordigen.BankID,
		),
	}
}

// payeeStripNonAlphanumeric removes all non-alphanumeric characters from payee
func payeeStripNonAlphanumeric(payee string) (x string) {
	reg := regexp.MustCompile(`[^\p{L}]+`)
	x = reg.ReplaceAllString(payee, " ")
	return strings.TrimSpace(x)
}

func (r Reader) toYnabber(a ynabber.Account, t nordigen.Transaction) (ynabber.Transaction, error) {
	transaction, err := r.Mapper().Map(a, t)
	if err != nil {
		return ynabber.Transaction{}, err
	}

	// Execute strip method on payee if defined in config
	if r.Config.Nordigen.PayeeStrip != nil {
		transaction.Payee = transaction.Payee.Strip(r.Config.Nordigen.PayeeStrip)
	}

	return transaction, nil
}

func (r Reader) toYnabbers(a ynabber.Account, t nordigen.AccountTransactions) ([]ynabber.Transaction, error) {
	y := []ynabber.Transaction{}
	for _, v := range t.Transactions.Booked {
		transaction, err := r.toYnabber(a, v)
		if err != nil {
			return nil, err
		}

		// Append transaction
		r.logger.Debug("mapped transaction", "from", v, "to", transaction)
		y = append(y, transaction)
	}
	return y, nil
}

func (r Reader) Bulk() (t []ynabber.Transaction, err error) {
	req, err := r.Requisition()
	if err != nil {
		return nil, fmt.Errorf("failed to authorize: %w", err)
	}

	r.logger.Info("", "accounts", len(req.Accounts))
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

		logger.Info("reading transactions")
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
