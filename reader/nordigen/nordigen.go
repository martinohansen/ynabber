package nordigen

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/internal/log"
)

type Reader struct {
	Config Config
	Client *nordigen.Client
	logger *slog.Logger
	// bulkFn and afterFn let runner tests exercise the production loop without
	// making network requests or waiting on wall-clock timers.
	bulkFn  func() ([]ynabber.Transaction, error)
	afterFn func(time.Duration) <-chan time.Time

	// TODO(Martin): Move into Nordigen config struct
	DataDir string
}

func (r Reader) String() string {
	return "nordigen"
}

// NewReader returns a new nordigen reader or panics
func NewReader(dataDir string) (Reader, error) {
	logger := slog.Default().With("reader", "nordigen")

	cfg := Config{}
	err := envconfig.Process("", &cfg)
	if err != nil {
		return Reader{}, fmt.Errorf("processing config: %w", err)
	}
	logger.Debug("config loaded", "config", &cfg)

	client, err := nordigen.NewClient(cfg.SecretID, cfg.SecretKey)
	if err != nil {
		// The library flattens token initialization errors into strings, so
		// their HTTP status and cause cannot be recovered reliably.
		log.Trace(logger, "nordigen client initialization failed", "error", err)
		return Reader{}, &responseError{message: "initializing nordigen client", cause: err}
	}

	return Reader{
		Client:  client,
		Config:  cfg,
		DataDir: dataDir,
		logger:  logger,
	}, nil
}

func (r Reader) toYnabbers(a ynabber.Account, t nordigen.AccountTransactions) ([]ynabber.Transaction, error) {
	logger := r.logger.With("account", log.MaskedBankIdentifier(a.IBAN))
	skipped := 0
	y := []ynabber.Transaction{}
	for _, v := range t.Transactions.Booked {
		transaction, err := r.Mapper(a, v)
		if err != nil {
			return nil, err
		}

		// Append transaction
		if transaction != nil {
			logger.Debug("mapped transaction", "transaction_id", v.TransactionId, "import_id", transaction.ID)
			log.Trace(logger, "mapped transaction data", "from", v, "to", transaction)
			y = append(y, *transaction)
		} else {
			skipped++
			logger.Debug("skipping transaction", "transaction_id", v.TransactionId)
			log.Trace(logger, "skipped transaction data", "transaction", v)
		}

	}
	logger.Info("read transactions", "total", len(y)+skipped, "skipped", skipped)
	return y, nil
}

func (r Reader) Bulk() (t []ynabber.Transaction, err error) {
	req, err := r.Requisition()
	if err != nil {
		return nil, fmt.Errorf("getting requisition: %w", err)
	}

	r.logger.Info("loaded requisition", "accounts", len(req.Accounts))
	for _, account := range req.Accounts {
		accountMetadata, err := r.Client.GetAccountMetadata(account)
		if err != nil {
			return nil, fmt.Errorf("getting account metadata: %w", apiResponseError(err))
		}
		logger := r.logger.With("iban", log.MaskedBankIdentifier(accountMetadata.Iban))
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
			return t, fmt.Errorf("getting transactions: %w", apiResponseError(err))
		}
		log.Trace(logger, "account transactions",
			"account", account,
			"transactions", transactions,
			"booked", len(transactions.Transactions.Booked),
			"pending", len(transactions.Transactions.Pending),
		)

		x, err := r.toYnabbers(account, transactions)
		if err != nil {
			return nil, fmt.Errorf("converting transaction: %w", err)
		}
		t = append(t, x...)
	}
	return t, nil
}
