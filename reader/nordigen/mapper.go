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
	case r.Config.Nordigen.BankID == "NORDEA_NDEADKKK":
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

// defaultMapper is generic and tries to identify the appropriate mapping
func (r Reader) defaultMapper(a ynabber.Account, t nordigen.Transaction) (*ynabber.Transaction, error) {
	PayeeSource := r.Config.Nordigen.PayeeSource
	TransactionID := r.Config.Nordigen.TransactionID

	amount, err := parseAmount(t)
	if err != nil {
		return nil, err
	}
	date, err := parseDate(t)
	if err != nil {
		return nil, err
	}

	// Get the Payee from the first data source that returns data in the order
	// defined by config
	payee := ""
	for _, source := range PayeeSource {
		if payee == "" {
			switch source {
			case "unstructured":
				// Use first unstructured string or array that is defied
				if t.RemittanceInformationUnstructured != "" {
					payee = t.RemittanceInformationUnstructured
				} else if t.RemittanceInformationUnstructuredArray != nil {
					payee = strings.Join(t.RemittanceInformationUnstructuredArray, " ")
				}

				// Unstructured data may need some formatting, some banks
				// inserts the amount and date which will cause every
				// transaction to create a new Payee
				payee = payeeStripNonAlphanumeric(payee)

			case "name":
				// Use either creditor or debtor as the payee
				if t.CreditorName != "" {
					payee = t.CreditorName
				} else if t.DebtorName != "" {
					payee = t.DebtorName
				}

			case "additional":
				// Use AdditionalInformation as payee
				payee = t.AdditionalInformation

			default:
				// Return an error if source is not recognized
				return nil, fmt.Errorf("unrecognized PayeeSource: %s", source)
			}
		}
	}

	// Remove elements in payee that is defined in config
	if r.Config.Nordigen.PayeeStrip != nil {
		payee = strip(payee, r.Config.Nordigen.PayeeStrip)
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
