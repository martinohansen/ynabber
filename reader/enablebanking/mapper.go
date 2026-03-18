package enablebanking

import (
	"crypto/sha256"
	"encoding/json"
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

// statusBooked is the only transaction status imported into YNAB.
// Transactions with any other non-empty status (e.g. "PDNG") are discarded
// because their IDs are unstable and their payee fields are often absent.
const statusBooked = "BOOK"

// defaultMapper is the generic mapper for EnableBanking transactions
func (r Reader) defaultMapper(account AccountInfo, tx EBTransaction) (*ynabber.Transaction, error) {
	if tx.Status != "" && tx.Status != statusBooked {
		return nil, nil
	}

	// Resolve a stable transaction ID.
	transactionID := resolveTransactionID(tx)

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

	// Truncate payee to 200 characters (YNAB API maximum length) If payee
	// exceeds this limit, the API ignores the field and it displays as blank in
	// YNAB.
	if r := []rune(payee); len(r) > 200 {
		payee = strings.TrimSpace(string(r[:200]))
	}

	return &ynabber.Transaction{
		Account: ynabber.Account{
			ID:   ynabber.ID(account.UID),
			Name: account.DisplayName,
			IBAN: account.StableID(),
		},
		ID:     ynabber.ID(transactionID),
		Date:   date,
		Payee:  payee,
		Memo:   memo,
		Amount: ynabber.MilliunitsFromAmount(amount),
	}, nil
}

// resolveTransactionID returns a stable identifier for tx, trying candidates
// in order of reliability:
//
//  1. entry_reference — bank-assigned, stable across sessions
//     (present for Sparebanken and credit-card ASPSPs).
//  2. reference_number — bank-assigned, stable across sessions
//     (null for all currently observed ASPSPs; included for future-proofing).
//  3. syntheticTransactionID — deterministic SHA-256 hash of the mandatory
//     transaction fields (fallback for DNB and any ASPSP that sets both
//     entry_reference and reference_number to null).
//
// transaction_id is intentionally excluded: for DNB it is a session-scoped
// token (base64 of a counter-prefixed string) that changes on every
// re-authorisation, and for other ASPSPs it is merely a re-encoding of
// entry_reference.
//
// The function always returns a non-empty string.
func resolveTransactionID(tx EBTransaction) string {
	if value := stringFromInterface(tx.EntryReference); value != "" {
		return value
	}
	if value := stringFromInterface(tx.ReferenceNumber); value != "" {
		return value
	}
	return syntheticTransactionID(tx)
}

// syntheticTransactionID derives a deterministic, session-independent
// transaction identifier for ASPSPs that provide no stable bank-assigned
// reference (e.g. DNB, where entry_reference is null and transaction_id is a
// session-scoped opaque token that changes on re-authorisation).
//
// The hash input is the JSON encoding of the always-present EnableBanking
// fields, which provides unambiguous field boundaries without manual separator
// logic:
//
//	[booking_date, amount, currency, credit_debit_indicator, remittance_information...]
//
// The "synth:" prefix distinguishes synthetic IDs from bank-assigned IDs in
// logs and makes the origin of the value unambiguous.
//
// Note: remittance_information is included in the hash. Some ASPSPs (e.g. DNB)
// enrich this field asynchronously after booking, which changes the hash and
// causes duplicates in YNAB. Use YNAB_DELAY=12h to avoid importing transactions
// before their remittance information has stabilised.
func syntheticTransactionID(tx EBTransaction) string {
	parts := []string{
		tx.BookingDate,
		tx.TransactionAmount.Amount,
		tx.TransactionAmount.Currency,
		tx.CreditDebitIndicator,
	}
	parts = append(parts, tx.RemittanceInformation...)
	b, _ := json.Marshal(parts)
	h := sha256.Sum256(b)
	return fmt.Sprintf("synth:%x", h)
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
