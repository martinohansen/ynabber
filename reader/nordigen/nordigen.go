package nordigen

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
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

// payeeStrip returns payee with elements of strips removed
func payeeStrip(payee string, strips []string) (x string) {
	for _, strip := range strips {
		x = strings.ReplaceAll(payee, strip, "")
	}
	return strings.TrimSpace(x)
}

// payeeStripNonAlphanumeric removes all non-alphanumeric characters from payee
func payeeStripNonAlphanumeric(payee string) (x string) {
	reg := regexp.MustCompile(`[^\p{L}]+`)
	x = reg.ReplaceAllString(payee, " ")
	return strings.TrimSpace(x)
}

func transactionToYnabber(cfg ynabber.Config, account ynabber.Account, t nordigen.Transaction) (x ynabber.Transaction, err error) {
	memo := t.RemittanceInformationUnstructured

	amount, err := strconv.ParseFloat(t.TransactionAmount.Amount, 64)
	if err != nil {
		return ynabber.Transaction{}, fmt.Errorf("failed to convert string to float: %w", err)
	}
	milliunits := ynabber.MilliunitsFromAmount(amount)

	date, err := time.Parse(timeLayout, t.BookingDate)
	if err != nil {
		return ynabber.Transaction{}, fmt.Errorf("failed to parse string to time: %w", err)
	}

	// Get the Payee data source
	payee := ""
	for _, source := range cfg.Nordigen.PayeeSource {
		if payee == "" {
			switch source {
			case "name":
				// Creditor/debtor name can be used as is
				if t.CreditorName != "" {
					payee = t.CreditorName
				} else if t.DebtorName != "" {
					payee = t.DebtorName
				}
			case "unstructured":
				// Unstructured data may need some formatting
				payee = t.RemittanceInformationUnstructured

				// Parse Payee according the user specified strips and
				// remove non-alphanumeric
				if cfg.Nordigen.PayeeStrip != nil {
					payee = payeeStrip(payee, cfg.Nordigen.PayeeStrip)
				}
				payee = payeeStripNonAlphanumeric(payee)
			default:
				return ynabber.Transaction{}, fmt.Errorf("unrecognized PayeeSource: %s", source)
			}
		}
	}

	return ynabber.Transaction{
		Account: account,
		ID:      ynabber.ID(ynabber.IDFromString(t.TransactionId)),
		Date:    date,
		Payee:   ynabber.Payee(payee),
		Memo:    memo,
		Amount:  milliunits,
	}, nil
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

func BulkReader(cfg ynabber.Config) (t []ynabber.Transaction, err error) {
	c, err := nordigen.NewClient(cfg.Nordigen.SecretID, cfg.Nordigen.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Select persistent dataFile
	dataFileBankSpecific := fmt.Sprintf("%s/%s-%s.json", cfg.DataDir, "ynabber", cfg.Nordigen.BankID)
	dataFileGeneric := fmt.Sprintf("%s/%s.json", cfg.DataDir, "ynabber")
	dataFile := dataFileBankSpecific

	_, err = os.Stat(dataFileBankSpecific)
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

	Authorization := Authorization{
		Client: *c,
		BankID: cfg.Nordigen.BankID,
		File:   dataFile,
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
