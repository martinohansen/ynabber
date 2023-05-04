package nordigen

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

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

// dataFile returns a persistent path
func dataFile(cfg ynabber.Config) string {
	dataFile := ""
	if cfg.Nordigen.Datafile != "" {
		if path.IsAbs(cfg.Nordigen.Datafile) {
			dataFile = cfg.Nordigen.Datafile
		} else {
			dataFile = fmt.Sprintf("%s/%s", cfg.DataDir, cfg.Nordigen.Datafile)
		}
	} else {
		dataFileBankSpecific := fmt.Sprintf("%s/%s-%s.json", cfg.DataDir, "ynabber", cfg.Nordigen.BankID)
		dataFileGeneric := fmt.Sprintf("%s/%s.json", cfg.DataDir, "ynabber")
		dataFile = dataFileBankSpecific
		_, err := os.Stat(dataFileBankSpecific)
		if errors.Is(err, os.ErrNotExist) {
			_, err := os.Stat(dataFileGeneric)
			if errors.Is(err, os.ErrNotExist) {
				// If bank specific does not exists and neither does generic, use dataFileBankSpecific
				dataFile = dataFileBankSpecific
			} else {
				// Generic dataFile exists(old naming) but not the bank specific, use dataFileGeneric
				dataFile = dataFileGeneric
			}
		}
	}
	return dataFile
}

func BulkReader(cfg ynabber.Config) (t []ynabber.Transaction, err error) {
	c, err := nordigen.NewClient(cfg.Nordigen.SecretID, cfg.Nordigen.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	Authorization := Authorization{
		Client: *c,
		BankID: cfg.Nordigen.BankID,
		File:   dataFile(cfg),
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

		// Handle expired, or suspended accounts by recreating the
		// requisition.
		switch accountMetadata.Status {
		case "EXPIRED", "SUSPENDED":
			log.Printf(
				"Account: %s is %s. Going to recreate the requisition...",
				account,
				accountMetadata.Status,
			)
			Authorization.CreateAndSave()
		}

		account := ynabber.Account{
			ID:   ynabber.ID(accountMetadata.Id),
			Name: accountMetadata.Iban,
			IBAN: accountMetadata.Iban,
		}

		log.Printf("Reading transactions from account: %s", account.Name)

		transactions, err := c.GetAccountTransactions(string(account.ID))
		if err != nil {
			return nil, fmt.Errorf("failed to get transactions: %w", err)
		}

		if cfg.Debug {
			log.Printf("Transactions received from Nordigen: %+v", transactions)
		}

		x, err := transactionsToYnabber(cfg, account, transactions)
		if err != nil {
			return nil, fmt.Errorf("failed to convert transaction: %w", err)
		}
		t = append(t, x...)
	}
	return t, nil
}
