package nordigen

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

const timeLayout = "2006-01-02"

type AccountMap map[string]string

func readAccountMap() (AccountMap, error) {
	accountMapFile, found := os.LookupEnv("NORDIGEN_ACCOUNTMAP")
	if !found {
		return nil, fmt.Errorf("env variable NORDIGEN_ACCOUNTMAP: %w", ynabber.ErrNotFound)
	}
	var accountMap AccountMap
	err := json.Unmarshal([]byte(accountMapFile), &accountMap)
	if err != nil {
		return nil, err
	}
	return accountMap, nil
}

func accountParser(account string, accountMap AccountMap) (ynabber.Account, error) {
	for nordea, ynab := range accountMap {
		if account == nordea {
			return ynabber.Account{
				ID:   ynabber.ID(ynabber.IDFromString(ynab)),
				Name: nordea,
			}, nil
		}
	}
	return ynabber.Account{}, fmt.Errorf("account not found in map: %w", ynabber.ErrNotFound)
}

func BulkReader() (t []ynabber.Transaction, err error) {
	// Read required config values
	secretID := ynabber.ConfigLookup("NORDIGEN_SECRET_ID", "")
	secretKey := ynabber.ConfigLookup("NORDIGEN_SECRET_KEY", "")
	bankId, found := os.LookupEnv("NORDIGEN_BANKID")
	if !found {
		return nil, fmt.Errorf("env variable NORDIGEN_BANKID: %w", ynabber.ErrNotFound)
	}

	// Read optional config values
	store := ynabber.ConfigLookup("NORDIGEN_STORE", "disk")
	endUserID := ynabber.ConfigLookup("NORDIGEN_ENDUSERID", "ynabber")

	c, err := nordigen.NewClient(secretID, secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	Authorization := Authorization{
		Client:    *c,
		BankID:    bankId,
		EndUserId: endUserID,
		Store:     store,
	}
	r, err := Authorization.Wrapper()
	if err != nil {
		return nil, fmt.Errorf("failed to authorize: %w", err)
	}

	accountMap, err := readAccountMap()
	if err != nil {
		return nil, fmt.Errorf("failed to read account map: %w", err)
	}

	log.Printf("Found %v accounts", len(r.Accounts))
	for _, account := range r.Accounts {
		accountMetadata, err := c.GetAccountMetadata(account)
		if err != nil {
			return nil, fmt.Errorf("failed to get account metadata: %w", err)
		}
		accountID := accountMetadata.Id
		accountName := accountMetadata.Iban

		log.Printf("Reading transactions from account: %s", accountName)
		transactions, err := c.GetAccountTransactions(accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get transactions: %w", err)
		}
		for _, v := range transactions.Transactions.Booked {
			memo := v.RemittanceInformationUnstructured
			amount, err := ynabber.MilliunitsFromString(v.TransactionAmount.Amount, ".")
			if err != nil {
				return nil, fmt.Errorf("failed to convert string to milliunits: %w", err)
			}

			date, err := time.Parse(timeLayout, v.BookingDate)
			if err != nil {
				return nil, fmt.Errorf("failed to parse string to time: %w", err)
			}

			account, err := accountParser(accountName, accountMap)
			if err != nil {
				if errors.Is(err, ynabber.ErrNotFound) {
					log.Printf("No matching account found for: %v", accountName)
					break
				}
				return nil, err
			}

			// Append transation
			x := ynabber.Transaction{
				Account: account,
				ID:      ynabber.ID(ynabber.IDFromString(v.TransactionId)),
				Date:    date,
				Payee:   ynabber.Payee(memo),
				Memo:    memo,
				Amount:  amount,
			}
			t = append(t, x)
		}
	}
	return t, nil
}
