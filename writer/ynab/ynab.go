package ynab

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/internal/log"
)

const maxMemoSize int = 200  // Max size of memo field in YNAB API
const maxPayeeSize int = 100 // Max size of payee field in YNAB API
// maxResponseBodyBytes caps how much of the YNAB API response body we buffer.
const maxResponseBodyBytes = 10 * 1024 * 1024

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
	Config Config
	logger *slog.Logger
}

// String returns the name of the writer
func (w Writer) String() string {
	return "ynab"
}

// NewWriter returns a new YNAB writer
func NewWriter() (Writer, error) {
	cfg := Config{}
	err := envconfig.Process("", &cfg)
	if err != nil {
		return Writer{}, fmt.Errorf("processing config: %w", err)
	}

	return Writer{
		Config: cfg,
		logger: slog.Default().With(
			"writer", "ynab",
			"budget_id", cfg.BudgetID,
		),
	}, nil
}

// accountParser takes an Account and returns the matching YNAB account ID in
// accountMap. It tries to match by ID first (for enablebanking account_uid),
// then by IBAN (for nordigen or enablebanking with IBAN).
func accountParser(account ynabber.Account, accountMap map[string]string) (string, error) {
	// Try matching by Account ID first (for enablebanking account_uid)
	if account.ID != "" {
		if ynabID, ok := accountMap[string(account.ID)]; ok {
			return ynabID, nil
		}
	}

	// Fall back to matching by IBAN (for nordigen or enablebanking with IBAN)
	if account.IBAN != "" {
		if ynabID, ok := accountMap[account.IBAN]; ok {
			return ynabID, nil
		}
	}

	// If neither ID nor IBAN matched, return error with hint about both options
	identifier := string(account.ID)
	if identifier == "" {
		identifier = account.IBAN
	}
	return "", fmt.Errorf("no matching YNAB account for ID=%s IBAN=%s in map: %v", account.ID, account.IBAN, accountMap)
}

// makeID returns a unique YNAB import ID to avoid duplicate transactions.
// IBAN is preferred when available to preserve backward compatibility with
// existing Nordigen import IDs. Account ID is used only when IBAN is absent
// (e.g. account types that EnableBanking does not expose an IBAN for).
func makeID(t ynabber.Transaction) string {
	date := t.Date.Format("2006-01-02")
	amount := t.Amount.String()

	// Prefer IBAN for stable, backward-compatible import IDs. Both Nordigen
	// and EnableBanking populate IBAN for standard account types.
	accountIdentifier := t.Account.IBAN
	if accountIdentifier == "" {
		accountIdentifier = string(t.Account.ID)
	}

	s := [][]byte{
		[]byte(accountIdentifier),
		[]byte(t.ID),
		[]byte(date),
		[]byte(amount),
	}
	hash := sha256.Sum256(bytes.Join(s, []byte("")))
	return fmt.Sprintf("YBBR:%x", hash)[:32]
}

func (w Writer) toYNAB(source ynabber.Transaction) (Transaction, error) {
	accountID, err := accountParser(source.Account, w.Config.AccountMap)
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
	// SwapFlow can match by Account ID (enablebanking) or IBAN (nordigen)
	if w.Config.SwapFlow != nil {
		for _, account := range w.Config.SwapFlow {
			if account == source.Account.IBAN || account == string(source.Account.ID) {
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
		Cleared:   string(w.Config.Cleared),
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
	fromDate := time.Time(w.Config.FromDate)
	delay := w.Config.Delay

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

	url := fmt.Sprintf("https://api.youneedabudget.com/v1/budgets/%s/transactions", w.Config.BudgetID)

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
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", w.Config.Token))

	log.Trace(w.logger, "http request", "method", req.Method, "url", req.URL.String(), "body", payload)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	resPayload, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBodyBytes))
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	log.Trace(w.logger, "http response", "status", res.Status, "body", resPayload)

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

// Runner reads batches of transactions from in and writes them using Bulk.
func (w Writer) Runner(ctx context.Context, in <-chan []ynabber.Transaction) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case batch, ok := <-in:
			if !ok {
				return nil // Channel closed, normal termination
			}
			if err := w.Bulk(batch); err != nil {
				if w.logger != nil {
					w.logger.Error("bulk writing transactions", "error", err)
				}
				return err
			}
		}
	}
}
