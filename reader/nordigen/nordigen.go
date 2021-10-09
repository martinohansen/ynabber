package nordigen

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/frieser/nordigen-go-lib"
	"github.com/martinohansen/ynabber"
)

const timeLayout = "2006-01-02"

type AccountMap map[string]string

var ErrNotFound = errors.New("not found")

func readAccountMap(file string) (AccountMap, error) {
	accountMapFile, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var accountMap AccountMap
	err = json.Unmarshal(accountMapFile, &accountMap)
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
	return ynabber.Account{}, fmt.Errorf("account not found in map: %w", ErrNotFound)
}

func BulkReader() (t []ynabber.Transaction, err error) {
	token, found := os.LookupEnv("NORDIGEN_TOKEN")
	if !found {
		return nil, fmt.Errorf("environment variable NORDIGEN_TOKEN not found")
	}
	bankId, found := os.LookupEnv("NORDIGEN_BANKID")
	if !found {
		return nil, fmt.Errorf("environment variable NORDIGEN_BANKID not found")
	}
	accountMapFile, found := os.LookupEnv("NORDIGEN_ACCOUNTMAP")
	if !found {
		accountMapFile = "nordigen-accountmap.json"
	}

	c := nordigen.NewClient(token)
	r, err := AuthorizationWrapper(c, bankId, "ynabber")
	if err != nil {
		return nil, err
	}

	accountMapPath := fmt.Sprintf("%s/%s", ynabber.DataDir(), accountMapFile)
	accountMap, err := readAccountMap(accountMapPath)
	if err != nil {
		return nil, err
	}

	for _, account := range r.Accounts {
		accountMetadata, err := c.GetAccountMetadata(account)
		if err != nil {
			return nil, err
		}
		accountID := accountMetadata.Id
		accountName := accountMetadata.Iban

		log.Printf("Reading transactions from account: %s", accountName)
		transactions, _ := c.GetAccountTransactions(accountID)
		for _, v := range transactions.Transactions.Booked {
			memo := v.RemittanceInformationUnstructured
			amount, err := ynabber.MilliunitsFromString(v.TransactionAmount.Amount, ".")
			if err != nil {
				log.Printf("failed to convert string to milliunits: %s", err)
				return nil, err
			}

			date, err := time.Parse(timeLayout, v.BookingDate)
			if err != nil {
				log.Printf("failed to parse string to time: %s", err)
				return nil, err
			}

			account, err := accountParser(accountName, accountMap)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					log.Printf("No matching account found for: %s", accountName)
					break
				}
				return nil, err
			}

			// Append transation
			x := ynabber.Transaction{
				Account: account,
				ID:     ynabber.ID(ynabber.IDFromString(v.TransactionId)),
				Date:   date,
				Payee:  ynabber.Payee(memo),
				Memo:   memo,
				Amount: amount,
			}
			t = append(t, x)
		}
	}
	return t, nil
}
