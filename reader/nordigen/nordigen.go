package nordigen

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

const timeLayout = "2006-01-02"

// payeeStrip returns payee with elements of strips removed
func payeeStrip(payee string, strips []string) (x string) {
	x = payee
	for _, strip := range strips {
		x = strings.ReplaceAll(x, strip, "")
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
	id := t.TransactionId
	if id == "" {
		log.Printf("Transaction ID is empty, this might cause duplicate entires in YNAB")
	}

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
		ID:      ynabber.ID(id),
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
		ok, reason := accountReady(accountMetadata)
		if !ok {
			log.Printf(
				"Account: %s is not ok: %s. Going to recreate the requisition...",
				account,
				reason,
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
			log.Printf("Transactions received from Nordigen: %s\n", transactions)
		}

		x, err := transactionsToYnabber(cfg, account, transactions)
		if err != nil {
			return nil, fmt.Errorf("failed to convert transaction: %w", err)
		}
		t = append(t, x...)
	}
	return t, nil
}
