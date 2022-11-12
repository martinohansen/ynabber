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

func ynabberToYNAB(cfg ynabber.Config, t ynabber.Transaction) Ytransaction {
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

	// Generating YNAB compliant import ID, output example:
	// YBBR:-741000:2021-02-18:92f2beb1
	hash := sha256.Sum256([]byte(t.Memo))
	id := fmt.Sprintf("YBBR:%s:%s:%x", amount, date, hash[:2])

	return Ytransaction{
		AccountID: string(t.Account.ID),
		Date:      date,
		Amount:    amount,
		PayeeName: payee,
		Memo:      memo,
		ImportID:  id,
		Cleared:   cfg.YNAB.Cleared,
		Approved:  false,
	}
}

func BulkWriter(cfg ynabber.Config, t []ynabber.Transaction) error {
	if len(t) == 0 {
		log.Println("No transactions to write")
		return nil
	}

	// Build array of transactions to send to YNAB
	y := new(Ytransactions)
	for _, v := range t {
		y.Transactions = append(y.Transactions, ynabberToYNAB(cfg, v))
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
