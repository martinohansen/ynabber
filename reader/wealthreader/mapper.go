package wealthreader

import (
	"fmt"
	"strings"
	"time"

	"github.com/martinohansen/ynabber"
)

// Mapper turns one Wealth Reader movement into a ynabber.Transaction.
func (r Reader) Mapper(account Account, tx Transaction) (*ynabber.Transaction, error) {
	transactionID := strings.TrimSpace(tx.UUID)
	if transactionID == "" {
		return nil, fmt.Errorf("missing transaction uuid")
	}

	dateStr, err := resolveDate(tx)
	if err != nil {
		return nil, err
	}
	date, err := parseDateFlexible(dateStr)
	if err != nil {
		return nil, fmt.Errorf("parsing date: %w", err)
	}

	amount, err := ynabber.MilliunitsFromString(tx.Amount.String())
	if err != nil {
		return nil, fmt.Errorf("parsing amount: %w", err)
	}

	payee := r.extractPayee(tx)
	memo := r.extractMemo(tx)

	if r.Config.PayeeStrip != nil {
		payee = strip(payee, r.Config.PayeeStrip)
	}
	if len(r.Config.PayeeStripRegex) > 0 {
		payee = stripRegex(payee, r.Config.PayeeStripRegex)
	}
	if runes := []rune(payee); len(runes) > 200 {
		payee = strings.TrimSpace(string(runes[:200]))
	}

	return &ynabber.Transaction{
		Account: ynabber.Account{
			ID:   ynabber.ID(account.UUID),
			Name: account.Name,
			IBAN: account.Code,
		},
		ID:     ynabber.ID(transactionID),
		Date:   date,
		Payee:  payee,
		Memo:   memo,
		Amount: amount,
	}, nil
}

func resolveDate(tx Transaction) (string, error) {
	if tx.OperationDate != "" {
		return tx.OperationDate, nil
	}
	if tx.ValueDate != "" {
		return tx.ValueDate, nil
	}
	return "", fmt.Errorf("missing operation_date and value_date")
}

func parseDateFlexible(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("failed to parse date: empty value")
	}
	formats := []string{
		dateFormat,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	var lastErr error
	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse date: %w", lastErr)
}

// extractPayee prefers the bank description, then the SEPA counterpart.
func (r Reader) extractPayee(tx Transaction) string {
	if desc := strings.TrimSpace(tx.Description); desc != "" {
		return desc
	}
	if name := strings.TrimSpace(tx.TransferDetails.SenderReceiver); name != "" {
		return name
	}
	return tx.UUID
}

// extractMemo keeps the SEPA concept when present, otherwise the description
// so the unstripped text is still visible after PayeeStrip.
func (r Reader) extractMemo(tx Transaction) string {
	if concept := strings.TrimSpace(tx.TransferDetails.Concept); concept != "" {
		return concept
	}
	return strings.TrimSpace(tx.Description)
}
