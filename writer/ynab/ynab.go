package ynab

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"

	"github.com/martinohansen/ynabber"
)

const maxMemoSize int = 200  // Max size of memo field in YNAB API
const maxPayeeSize int = 100 // Max size of payee field in YNAB API

var space = regexp.MustCompile(`\s+`) // Matches all whitespace characters

// Ytransaction is a single YNAB transaction
type Ytransaction struct {
	AccountID string `json:"account_id"`
	Date      string `json:"date"`
	Amount    string `json:"amount"`
	PayeeName string `json:"payee_name"`
	Memo      string `json:"memo"`
	ImportID  string `json:"import_id"`
	Cleared   string `json:"cleared"`
	Approved  bool   `json:"approved"`
}

// Ytransactions is multiple YNAB transactions
type Ytransactions struct {
	Transactions []Ytransaction `json:"transactions"`
}

// importIDMaker tries to return a unique YNAB import ID to avoid duplicate
// transactions.
func importIDMaker(cfg ynabber.Config, t ynabber.Transaction) string {
	// Common between versions
	date := t.Date.Format("2006-01-02")
	amount := t.Amount.String()

	// Version 1 uses the memo, amount and date from Ytransaction
	v1Cutover := time.Time(cfg.YNAB.ImportID.V1)
	v1 := func(t ynabber.Transaction) string {
		hash := sha256.Sum256([]byte(t.Memo))
		return fmt.Sprintf("YBBR:%s:%s:%x", amount, date, hash[:2])
	}

	// Version 2 uses, in order, the account IBAN, transaction ID, date, and
	// amount to build a hash of the transaction.
	v2Cutover := time.Time(cfg.YNAB.ImportID.V2)
	v2 := func(t ynabber.Transaction) string {
		s := [][]byte{
			[]byte(t.Account.IBAN),
			[]byte(t.ID),
			[]byte(date),
			[]byte(amount),
		}
		hash := sha256.Sum256(bytes.Join(s, []byte("")))
		return fmt.Sprintf("YBBR:%x", hash)[:32]
	}

	// Return the first generator from latest to oldest that have a cutover date
	// after or equal to the transaction date.
	if t.Date.After(v2Cutover) || t.Date.Equal(v2Cutover) {
		return v2(t)
	} else if t.Date.After(v1Cutover) || t.Date.Equal(v1Cutover) {
		return v1(t)
	} else {
		return v1(t)
	}
}

func ynabberToYNAB(cfg ynabber.Config, t ynabber.Transaction) (Ytransaction, error) {
	date := t.Date.Format("2006-01-02")
	amount := t.Amount.String()

	// Trim consecutive spaces from memo and truncate if too long
	memo := strings.TrimSpace(space.ReplaceAllString(t.Memo, " "))
	if len(memo) > maxMemoSize {
		log.Printf("Memo on account %s on date %s is too long - truncated to %d characters",
			t.Account.Name, date, maxMemoSize)
		memo = memo[0:(maxMemoSize - 1)]
	}

	// Trim consecutive spaces from payee and truncate if too long
	payee := strings.TrimSpace(space.ReplaceAllString(string(t.Payee), " "))
	if len(payee) > maxPayeeSize {
		log.Printf("Payee on account %s on date %s is too long - truncated to %d characters",
			t.Account.Name, date, maxPayeeSize)
		payee = payee[0:(maxPayeeSize - 1)]
	}

	return Ytransaction{
		ImportID:  importIDMaker(cfg, t),
		AccountID: string(t.Account.ID),
		Date:      date,
		Amount:    amount,
		PayeeName: payee,
		Memo:      memo,
		Cleared:   cfg.YNAB.Cleared,
		Approved:  false,
	}, nil
}

func BulkWriter(cfg ynabber.Config, t []ynabber.Transaction) error {
	if len(t) == 0 {
		log.Println("No transactions to write")
		return nil
	}

	// Build array of transactions to send to YNAB
	y := new(Ytransactions)
	for _, v := range t {
		transaction, err := ynabberToYNAB(cfg, v)
		if err != nil {
			// If we fail to parse a single transaction we log it but move on so
			// we don't halt the entire program.
			log.Printf("Failed to parse transaction: %s: %s", v, err)
			continue
		}
		y.Transactions = append(y.Transactions, transaction)
	}

	url := fmt.Sprintf("https://api.youneedabudget.com/v1/budgets/%s/transactions", cfg.YNAB.BudgetID)

	payload, err := json.Marshal(y)
	if err != nil {
		return err
	}

	client := &http.Client{}

	if cfg.Debug {
		log.Printf("Request to YNAB: %s\n", payload)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", cfg.YNAB.Token))

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if cfg.Debug {
		b, _ := httputil.DumpResponse(res, true)
		log.Printf("Response from YNAB: %s\n", b)
	}

	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to send request: %s", res.Status)
	} else {
		log.Printf("Successfully sent %v transaction(s) to YNAB", len(y.Transactions))
	}
	return nil
}
