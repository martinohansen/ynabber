package nordigen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

// Mapper uses the most specific mapper for the bank in question
func (r Reader) Mapper(a ynabber.Account, t nordigen.Transaction) (*ynabber.Transaction, error) {
	switch {
	case r.Config.BankID == "NORDEA_NDEADKKK":
		return r.nordeaMapper(a, t)

	default:
		return r.defaultMapper(a, t)
	}
}

func parseAmount(t nordigen.Transaction) (float64, error) {
	amount, err := strconv.ParseFloat(t.TransactionAmount.Amount, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to convert string to float: %w", err)
	}
	return amount, nil
}

func parseDate(t nordigen.Transaction) (time.Time, error) {
	date, err := time.Parse("2006-01-02", t.BookingDate)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse string to time: %w", err)
	}
	return date, nil
}

// payeeStripNonAlphanumeric removes all non-alphanumeric characters from payee
func payeeStripNonAlphanumeric(payee string) (x string) {
	reg := regexp.MustCompile(`[^\p{L}]+`)
	x = reg.ReplaceAllString(payee, " ")
	return strings.TrimSpace(x)
}

// Strip removes each string in strips from s
func strip(s string, strips []string) string {
	for _, strip := range strips {
		s = strings.ReplaceAll(s, strip, "")
	}
	return strings.TrimSpace(s)
}

// payeeFinder returns the first group of sources which yields a result. Each
// source in a group is concatenated with a space. If no sources yields a result
// an empty string is returned.
func payeeFinder(t nordigen.Transaction, sources PayeeSources) (payee string) {
	values := make([]string, 0)
	for _, group := range sources {
		for _, source := range group {
			value := getSourceValue(t, source)
			if value != "" {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			// Return the first group of sources which yields a result
			return strings.Join(values, " ")
		}
	}
	return "" // No sources yielded a result
}

// getSourceValue returns the value of source from t
func getSourceValue(t nordigen.Transaction, source PayeeSource) string {
	switch source {
	case Unstructured:
		var payee string

		// Use first unstructured string or array that is defined
		if t.RemittanceInformationUnstructured != "" {
			payee = t.RemittanceInformationUnstructured
		} else if t.RemittanceInformationUnstructuredArray != nil {
			payee = strings.Join(t.RemittanceInformationUnstructuredArray, " ")
		} else {
			return ""
		}

		// Unstructured data may need some formatting, some banks
		// inserts the amount and date which will cause every
		// transaction to create a new Payee
		return payeeStripNonAlphanumeric(payee)

	case Name:
		// Use either creditor or debtor as the payee
		if t.CreditorName != "" {
			return t.CreditorName
		} else if t.DebtorName != "" {
			return t.DebtorName
		}
		return ""

	case Additional:
		// Use AdditionalInformation as payee
		return t.AdditionalInformation
	}
	return ""
}

// defaultMapper is generic and tries to identify the appropriate mapping
func (r Reader) defaultMapper(a ynabber.Account, t nordigen.Transaction) (*ynabber.Transaction, error) {
	PayeeSource := r.Config.PayeeSource
	TransactionID := r.Config.TransactionID

	amount, err := parseAmount(t)
	if err != nil {
		return nil, err
	}
	date, err := parseDate(t)
	if err != nil {
		return nil, err
	}

	payee := payeeFinder(t, PayeeSource)

	// Remove elements in payee that is defined in config
	if r.Config.PayeeStrip != nil {
		payee = strip(payee, r.Config.PayeeStrip)
	}

	// Set the transaction ID according to config
	var id string
	switch TransactionID {
	case "InternalTransactionId":
		id = t.InternalTransactionId
	case "TransactionId":
		id = t.TransactionId
	default:
		return nil, fmt.Errorf("unrecognized TransactionID: %s", TransactionID)
	}

	return &ynabber.Transaction{
		Account: a,
		ID:      ynabber.ID(id),
		Date:    date,
		Payee:   ynabber.Payee(payee),
		Memo:    t.RemittanceInformationUnstructured,
		Amount:  ynabber.MilliunitsFromAmount(amount),
	}, nil
}

// nordeaMapper handles Nordea transactions specifically
func (r Reader) nordeaMapper(a ynabber.Account, t nordigen.Transaction) (*ynabber.Transaction, error) {
	// They now maintain two transactions for every actual transaction. First
	// they show up prefixed with a ID prefixed with a H, sometime later another
	// transaction describing the same transactions shows up with a new ID
	// prefixed with a P instead. The H transaction matches the date which its
	// visible in my account so i will discard the P transactions for now.
	if strings.HasPrefix(t.TransactionId, "P") {
		return nil, nil
	}

	return r.defaultMapper(a, t)
}
