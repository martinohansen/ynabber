package ynab

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/martinohansen/ynabber"
)

const maxMemoSize int = 200  // Max size of memo field in YNAB API
const maxPayeeSize int = 100 // Max size of payee field in YNAB API

var space = regexp.MustCompile(`\s+`) // Matches all whitespace characters

// Transaction is a single YNAB transaction
type Transaction struct {
	AccountID string `json:"account_id"`
	Date      string `json:"date"`
	Amount    string `json:"amount"`
	PayeeName string `json:"payee_name"`
	Memo      string `json:"memo"`
	ImportID  string `json:"import_id"`
	Cleared   string `json:"cleared"`
	Approved  bool   `json:"approved"`
}

// Transactions is multiple YNAB transactions
type Transactions struct {
	Transactions []Transaction `json:"transactions"`
}

type Writer struct {
	Config *ynabber.Config
	logger *slog.Logger
}

// NewWriter returns a new YNAB writer
func NewWriter(cfg *ynabber.Config) Writer {
	return Writer{
		Config: cfg,
		logger: slog.Default().With(
			"writer", "ynab",
			"budget_id", cfg.YNAB.BudgetID,
		),
	}
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
func makeID(t ynabber.Transaction) string {
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

func (w Writer) toYNAB(source ynabber.Transaction) (Transaction, error) {
	accountID, err := accountParser(source.Account.IBAN, w.Config.YNAB.AccountMap)
	if err != nil {
		return Transaction{}, err
	}

	date := source.Date.Format("2006-01-02")

	// Trim consecutive spaces from memo and truncate if too long
	memo := strings.TrimSpace(space.ReplaceAllString(source.Memo, " "))
	if len(memo) > maxMemoSize {
		w.logger.Warn("memo too long", "transaction", source, "max_size", maxMemoSize)
		memo = memo[0:(maxMemoSize - 1)]
	}

	// Trim consecutive spaces from payee and truncate if too long
	payee := strings.TrimSpace(space.ReplaceAllString(string(source.Payee), " "))
	if len(payee) > maxPayeeSize {
		w.logger.Warn("payee too long", "transaction", source, "max_size", maxPayeeSize)
		payee = payee[0:(maxPayeeSize - 1)]
	}

	// If SwapFlow is defined check if the account is configured to swap inflow
	// to outflow. If so swap it by using the Negate method.
	if w.Config.YNAB.SwapFlow != nil {
		for _, account := range w.Config.YNAB.SwapFlow {
			if account == source.Account.IBAN {
				source.Amount = source.Amount.Negate()
			}
		}
	}

	transaction := Transaction{
		ImportID:  makeID(source),
		AccountID: accountID,
		Date:      date,
		Amount:    source.Amount.String(),
		PayeeName: payee,
		Memo:      memo,
		Cleared:   string(w.Config.YNAB.Cleared),
		Approved:  false,
	}
	w.logger.Debug("mapped transaction", "from", source, "to", transaction)
	return transaction, nil
}

// checkTransactionDateValidity checks if date is within the limits of YNAB and
// ynabber.Config.
func (w Writer) checkTransactionDateValidity(date time.Time) bool {
	now := time.Now()
	fiveYearsAgo := now.AddDate(-5, 0, 0)
	fromDate := time.Time(w.Config.YNAB.FromDate)
	delay := w.Config.YNAB.Delay

	return date.After(fiveYearsAgo) && date.After(fromDate) && date.Before(now.Add(-delay))
}

func (w Writer) Bulk(t []ynabber.Transaction) error {
	// skipped and failed counters
	skipped := 0
	failed := 0

	// Build array of transactions to send to YNAB
	y := new(Transactions)
	for _, v := range t {
		// Skip transactions that are not within the valid date range.
		if !w.checkTransactionDateValidity(v.Date) {
			w.logger.Debug("date out of range", "transaction", v)
			skipped += 1
			continue
		}

		transaction, err := w.toYNAB(v)
		if err != nil {
			// If we fail to parse a single transaction we log it but move on so
			// we don't halt the entire program.
			w.logger.Error("mapping to YNAB", "transaction", transaction, "err", err)
			failed += 1
			continue
		}
		y.Transactions = append(y.Transactions, transaction)
	}

	if len(t) == 0 || len(y.Transactions) == 0 {
		w.logger.Info("no transactions to write")
		return nil
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

	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to send request: %s", res.Status)
	} else {
		w.logger.Info(
			"sent transactions",
			"status",
			res.StatusCode,
			"transactions",
			len(y.Transactions),
			"skipped",
			skipped,
			"failed",
			failed,
		)
	}
	return nil
}
