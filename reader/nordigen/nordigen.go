package nordigen

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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

func transactionsToYnabber(config ynabber.Config, account ynabber.Account, t nordigen.AccountTransactions) (x []ynabber.Transaction, err error) {
	for _, v := range t.Transactions.Booked {
		memo := v.RemittanceInformationUnstructured

		amount, err := strconv.ParseFloat(v.TransactionAmount.Amount, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert string to float: %w", err)
		}
		milliunits := ynabber.MilliunitsFromAmount(amount)

		date, err := time.Parse(timeLayout, v.BookingDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse string to time: %w", err)
		}

		// Find payee
		payee := ynabber.Payee("")
		for _, source := range config.Nordigen.PayeeSource {
			if payee == "" {
				if source == "name" {
					// Creditor/debtor name can be used as is
					if v.CreditorName != "" {
						payee = ynabber.Payee(v.CreditorName)
					} else if v.DebtorName != "" {
						payee = ynabber.Payee(v.DebtorName)
					}
				} else if source == "unstructured" {
					// Unstructured data may need some formatting
					payee = ynabber.Payee(v.RemittanceInformationUnstructured)
					s, err := payee.Parsed(config.Nordigen.PayeeStrip)
					if err != nil {
						log.Printf("Failed to parse payee: %s: %s", string(payee), err)
					}
					payee = ynabber.Payee(s)
				} else {
					return nil, fmt.Errorf("Unrecognized payee data source %s - fix configuration", source)
				}
			}
		}

		// Append transaction
		x = append(x, ynabber.Transaction{
			Account: account,
			ID:      ynabber.ID(ynabber.IDFromString(v.TransactionId)),
			Date:    date,
			Payee:   payee,
			Memo:    memo,
			Amount:  milliunits,
		})
	}
	return x, nil
}

func BulkReader(cfg ynabber.Config) (t []ynabber.Transaction, err error) {
	c, err := nordigen.NewClient(cfg.Nordigen.SecretID, cfg.Nordigen.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Select persistent datafile
	datafile_bankspecific := fmt.Sprintf("%s/%s-%s.json", cfg.DataDir, "ynabber", cfg.Nordigen.BankID)
	datafile_generic := fmt.Sprintf("%s/%s.json", cfg.DataDir, "ynabber")
	datafile := datafile_bankspecific

	_, err = os.Stat(datafile_bankspecific)
	if errors.Is(err, os.ErrNotExist) {
		_, err := os.Stat(datafile_generic)
		if errors.Is(err, os.ErrNotExist) {
			// If bank specific does not exists and neither does generic, use bankspecific
			datafile = datafile_bankspecific
		} else {
			// Generic datafile exists(old naming) but not the bank speific, use old file
			datafile = datafile_generic
			log.Printf("Using non-bank specific persistent datafile %s, consider moving to %s\n", datafile_generic, datafile_bankspecific)
		}
	}

	Authorization := Authorization{
		Client: *c,
		BankID: cfg.Nordigen.BankID,
		File:   datafile,
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
				continue
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
