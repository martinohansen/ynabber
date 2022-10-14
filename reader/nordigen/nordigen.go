package nordigen

import (
	"errors"
	"fmt"
	"log"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

const timeLayout = "2006-01-02"

func accountParser(account string, accountMap map[string]string) (ynabber.Account, error) {
	for from, to := range accountMap {
		if account == from {
			return ynabber.Account{
				ID:   ynabber.ID(ynabber.IDFromString(to)),
				Name: from,
			}, nil
		}
	}
	return ynabber.Account{}, fmt.Errorf("account not found in map: %w", ynabber.ErrNotFound)
}

func BulkReader(cfg ynabber.Config) (t []ynabber.Transaction, err error) {
	c, err := nordigen.NewClient(cfg.Nordigen.SecretID, cfg.Nordigen.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	Authorization := Authorization{
		Client: *c,
		BankID: cfg.Nordigen.BankID,
		File:   fmt.Sprintf("%s/%s.json", cfg.DataDir, "ynabber"),
	}
	r, err := Authorization.Wrapper()
	if err != nil {
		return nil, fmt.Errorf("failed to authorize: %w", err)
	}

	log.Printf("Found %v accounts", len(r.Accounts))
	for _, account := range r.Accounts {
		accountMetadata, err := c.GetAccountMetadata(account)
		if err != nil {
			return nil, fmt.Errorf("failed to get account metadata: %w", err)
		}
		accountID := accountMetadata.Id
		accountName := accountMetadata.Iban

		account, err := accountParser(accountName, cfg.Nordigen.AccountMap)
		if err != nil {
			if errors.Is(err, ynabber.ErrNotFound) {
				log.Printf("No matching account found for: %s in: %v", accountName, cfg.Nordigen.AccountMap)
				break
			}
			return nil, err
		}

		log.Printf("Reading transactions from account: %s", accountName)

		transactions, err := c.GetAccountTransactions(accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get transactions: %w", err)
		}

		x, err := transactionsToYnabber(cfg, account, transactions)
		if err != nil {
			return nil, fmt.Errorf("failed to convert transaction: %w", err)
		}
		t = append(t, x...)
	}
	return t, nil
}
