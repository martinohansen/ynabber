package enablebanking

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/martinohansen/ynabber"
)

// Mapper delegates all transaction mapping to the default mapper,
// which applies configuration-driven payee stripping and truncation.
func (r Reader) Mapper(account AccountInfo, tx EBTransaction) (*ynabber.Transaction, error) {
	return r.defaultMapper(account, tx)
}

// defaultMapper is the generic mapper for EnableBanking transactions
func (r Reader) defaultMapper(account AccountInfo, tx EBTransaction) (*ynabber.Transaction, error) {
	// Skip transactions with missing required fields
	transactionID, err := resolveTransactionID(tx)
	if err != nil {
		return nil, err
	}

	bookingDateStr, err := resolveBookingDate(tx)
	if err != nil {
		return nil, err
	}

	// Parse booking date
	date, err := parseDateFlexible(bookingDateStr)
	if err != nil {
		return nil, fmt.Errorf("parsing booking date: %w", err)
	}

	// Parse amount
	amount, err := parseAmount(tx.TransactionAmount.Amount)
	if err != nil {
		return nil, fmt.Errorf("parsing amount: %w", err)
	}

	// Adjust sign based on credit/debit indicator
	if tx.CreditDebitIndicator == "DBIT" {
		amount = -amount
	}

	// Extract payee and memo
	payee := r.extractPayee(tx)
	memo := r.extractMemo(tx)

	// Remove elements in payee that is defined in config
	if r.Config.PayeeStrip != nil {
		payee = strip(payee, r.Config.PayeeStrip)
	}

	// Truncate payee to 200 characters (YNAB API maximum length)
	// If payee exceeds this limit, the API ignores the field and it displays as blank in YNAB.
	if len(payee) > 200 {
		payee = strings.TrimSpace(payee[:200])
	}

	return &ynabber.Transaction{
		Account: ynabber.Account{
			ID:   ynabber.ID(account.UID),
			Name: account.DisplayName,
			IBAN: account.IBAN,
		},
		ID:     ynabber.ID(transactionID),
		Date:   date,
		Payee:  payee,
		Memo:   memo,
		Amount: ynabber.MilliunitsFromAmount(amount),
	}, nil
}

func resolveTransactionID(tx EBTransaction) (string, error) {
	if tx.TransactionID != "" {
		return tx.TransactionID, nil
	}
	if value := stringFromInterface(tx.EntryReference); value != "" {
		return value, nil
	}
	if value := stringFromInterface(tx.ReferenceNumber); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("missing transaction ID")
}

func resolveBookingDate(tx EBTransaction) (string, error) {
	if tx.BookingDate != "" {
		return tx.BookingDate, nil
	}
	if tx.ValueDate != "" {
		return tx.ValueDate, nil
	}
	if value := stringFromInterface(tx.TransactionDate); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("missing booking date")
}

// parseAmount parses a string amount to float64
func parseAmount(amountStr string) (float64, error) {
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse amount: %w", err)
	}
	return amount, nil
}

// parseDate parses a YYYY-MM-DD formatted date string
func parseDateFlexible(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("failed to parse date: empty value")
	}
	formats := []string{
		"2006-01-02",
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

func stringFromInterface(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

// extractPayee extracts payee information from a transaction
// Priority: remittance info > debtor name > creditor name > transaction ID
func (r Reader) extractPayee(tx EBTransaction) string {
	// Try remittance information first
	if len(tx.RemittanceInformation) > 0 {
		for _, info := range tx.RemittanceInformation {
			if info != "" {
				return info
			}
		}
	}

	// Try debtor name
	if debtor, ok := tx.Debtor.(map[string]interface{}); ok {
		if name, ok := debtor["name"].(string); ok && name != "" {
			return name
		}
	}

	// Try creditor name
	if creditor, ok := tx.Creditor.(map[string]interface{}); ok {
		if name, ok := creditor["name"].(string); ok && name != "" {
			return name
		}
	}

	// Fallback to transaction ID
	return tx.TransactionID
}

// extractMemo extracts memo/description from a transaction.
//
// Priority:
//  1. RemittanceInformation[1:] joined — when the bank provides multiple
//     strings, the first becomes the Payee and the rest become the Memo.
//  2. RemittanceInformation[0] — when there is only one string it is used for
//     both Payee (after PayeeStrip) and Memo (raw), matching the Nordigen
//     reader behaviour so that the full unstripped text is always visible in
//     YNAB.
//  3. Note field — some banks populate this instead of remittance info.
func (r Reader) extractMemo(tx EBTransaction) string {
	if len(tx.RemittanceInformation) > 1 {
		return strings.Join(tx.RemittanceInformation[1:], " ")
	}
	if len(tx.RemittanceInformation) == 1 && tx.RemittanceInformation[0] != "" {
		return tx.RemittanceInformation[0]
	}
	if note, ok := tx.Note.(string); ok && note != "" {
		return note
	}
	return ""
}

// strip removes each string in strips from s
func strip(s string, strips []string) string {
	for _, strip := range strips {
		s = strings.ReplaceAll(s, strip, "")
	}
	return strings.TrimSpace(s)
}
