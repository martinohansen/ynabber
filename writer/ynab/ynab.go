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
	"time"

	"github.com/martinohansen/ynabber"
)

const maxMemoSize int = 200  // Max size of memo field in YNAB API
const maxPayeeSize int = 100 // Max size of payee field in YNAB API

type Writer struct {
	Config *ynabber.Config
}

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

// accountParser takes IBAN and returns the matching YNAB account ID in
// accountMap
func accountParser(iban string, accountMap map[string]string) (string, error) {
	for from, to := range accountMap {
		if iban == from {
			return to, nil
		}
	}
	return "", fmt.Errorf("no account for: %s in map: %s", iban, accountMap)
}

// makeID returns a unique YNAB import ID to avoid duplicate transactions.
func makeID(cfg ynabber.Config, t ynabber.Transaction) string {
	date := t.Date.Format("2006-01-02")
	amount := t.Amount.String()

	s := [][]byte{
		[]byte(t.Account.IBAN),
		[]byte(t.ID),
		[]byte(date),
		[]byte(amount),
	}
	hash := sha256.Sum256(bytes.Join(s, []byte("")))
	return fmt.Sprintf("YBBR:%x", hash)[:32]
}

func ynabberToYNAB(cfg ynabber.Config, t ynabber.Transaction) (Ytransaction, error) {
	accountID, err := accountParser(t.Account.IBAN, cfg.YNAB.AccountMap)
	if err != nil {
		return Ytransaction{}, err
	}

	date := t.Date.Format("2006-01-02")

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

	// If SwapFlow is defined check if the account is configured to swap inflow
	// to outflow. If so swap it by using the Negate method.
	if cfg.YNAB.SwapFlow != nil {
		for _, account := range cfg.YNAB.SwapFlow {
			if account == t.Account.IBAN {
				t.Amount = t.Amount.Negate()
			}
		}
	}

	return Ytransaction{
		ImportID:  makeID(cfg, t),
		AccountID: accountID,
		Date:      date,
		Amount:    t.Amount.String(),
		PayeeName: payee,
		Memo:      memo,
		Cleared:   cfg.YNAB.Cleared,
		Approved:  false,
	}, nil
}

func (w Writer) Bulk(t []ynabber.Transaction) error {
	// skipped and failed counters
	skipped := 0
	failed := 0

	// Build array of transactions to send to YNAB
	y := new(Ytransactions)
	for _, v := range t {
		// Skip transaction if the date is before FromDate
		if v.Date.Before(time.Time(w.Config.YNAB.FromDate)) {
			skipped += 1
			continue
		}

		transaction, err := ynabberToYNAB(*w.Config, v)
		if err != nil {
			// If we fail to parse a single transaction we log it but move on so
			// we don't halt the entire program.
			log.Printf("Failed to parse transaction: %s: %s", v, err)
			failed += 1
			continue
		}
		y.Transactions = append(y.Transactions, transaction)
	}

	if len(t) == 0 || len(y.Transactions) == 0 {
		log.Println("No transactions to write")
		return nil
	}

	if w.Config.Debug {
		log.Printf("Request to YNAB: %+v", y)
	}

	url := fmt.Sprintf("https://api.youneedabudget.com/v1/budgets/%s/transactions", w.Config.YNAB.BudgetID)

	payload, err := json.Marshal(y)
	if err != nil {
		return err
	}

	client := &http.Client{}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.Config.YNAB.Token))

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if w.Config.Debug {
		b, _ := httputil.DumpResponse(res, true)
		log.Printf("Response from YNAB: %s", b)
	}

	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to send request: %s", res.Status)
	} else {
		log.Printf(
			"Successfully sent %v transaction(s) to YNAB. %d got skipped and %d failed.",
			len(y.Transactions),
			skipped,
			failed,
		)
	}
	return nil
}
