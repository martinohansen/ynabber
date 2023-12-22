package nordigen

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

type Reader struct {
	Config *ynabber.Config

	Client *nordigen.Client
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
	}
}

// payeeStripNonAlphanumeric removes all non-alphanumeric characters from payee
func payeeStripNonAlphanumeric(payee string) (x string) {
	reg := regexp.MustCompile(`[^\p{L}]+`)
	x = reg.ReplaceAllString(payee, " ")
	return strings.TrimSpace(x)
}

func transactionToYnabber(cfg ynabber.Config, account ynabber.Account, t nordigen.Transaction) (y ynabber.Transaction, err error) {
	// Pick an appropriate mapper based on the BankID provided or fallback to
	// our default best effort mapper.
	switch cfg.Nordigen.BankID {
	default:
		y, err = Default{}.Map(cfg, account, t)
	}

	// Return now if any of the mappings resulted in error
	if err != nil {
		return y, err
	}

	// Execute strip method on payee if defined in config
	if cfg.Nordigen.PayeeStrip != nil {
		y.Payee = y.Payee.Strip(cfg.Nordigen.PayeeStrip)
	}

	return y, err
}

func transactionsToYnabber(cfg ynabber.Config, account ynabber.Account, t nordigen.AccountTransactions) (x []ynabber.Transaction, err error) {
	for _, v := range t.Transactions.Booked {
		transaction, err := transactionToYnabber(cfg, account, v)
		if err != nil {
			return nil, err
		}
		// Append transaction
		x = append(x, transaction)
	}
	return x, nil
}

func (r Reader) Bulk() (t []ynabber.Transaction, err error) {
	req, err := r.Requisition()
	if err != nil {
		return nil, fmt.Errorf("failed to authorize: %w", err)
	}

	log.Printf("Found %v accounts", len(req.Accounts))
	for _, account := range req.Accounts {
		accountMetadata, err := r.Client.GetAccountMetadata(account)
		if err != nil {
			return nil, fmt.Errorf("failed to get account metadata: %w", err)
		}

		// Handle expired, or suspended accounts by recreating the
		// requisition.
		switch accountMetadata.Status {
		case "EXPIRED", "SUSPENDED":
			log.Printf(
				"Account: %s is %s. Going to recreate the requisition...",
				account,
				accountMetadata.Status,
			)
			r.createRequisition()
		}

		account := ynabber.Account{
			ID:   ynabber.ID(accountMetadata.Id),
			Name: accountMetadata.Iban,
			IBAN: accountMetadata.Iban,
		}

		log.Printf("Reading transactions from account: %s", account.Name)

		transactions, err := r.Client.GetAccountTransactions(string(account.ID))
		if err != nil {
			return nil, fmt.Errorf("failed to get transactions: %w", err)
		}

		if r.Config.Debug {
			log.Printf("Transactions received from Nordigen: %+v", transactions)
		}

		x, err := transactionsToYnabber(*r.Config, account, transactions)
		if err != nil {
			return nil, fmt.Errorf("failed to convert transaction: %w", err)
		}
		t = append(t, x...)
	}
	return t, nil
}
