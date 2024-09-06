package nordigen

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

type Mapper interface {
	Map(ynabber.Account, nordigen.Transaction) (ynabber.Transaction, error)
}

// Mapper returns a mapper to transform the banks transaction to Ynabber
func (r Reader) Mapper() Mapper {
	switch r.Config.Nordigen.BankID {
	case "NORDEA_NDEADKKK":
		return Nordea{}

	default:
		return Default{
			PayeeSource:   r.Config.Nordigen.PayeeSource,
			TransactionID: r.Config.Nordigen.TransactionID,
		}
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

// Default mapping for all banks unless a more specific mapping exists
type Default struct {
	PayeeSource   []string
	TransactionID string
}

// Map t using the default mapper
func (mapper Default) Map(a ynabber.Account, t nordigen.Transaction) (ynabber.Transaction, error) {
	amount, err := parseAmount(t)
	if err != nil {
		return ynabber.Transaction{}, err
	}
	date, err := parseDate(t)
	if err != nil {
		return ynabber.Transaction{}, err
	}

	// Get the Payee from the first data source that returns data in the order
	// defined by config
	payee := ""
	for _, source := range mapper.PayeeSource {
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
				return ynabber.Transaction{}, fmt.Errorf("unrecognized PayeeSource: %s", source)
			}
		}
	}

	// Set the transaction ID according to config
	var id string
	switch mapper.TransactionID {
	case "InternalTransactionId":
		id = t.InternalTransactionId
	case "TransactionId":
		id = t.TransactionId
	default:
		return ynabber.Transaction{}, fmt.Errorf("unrecognized TransactionID: %s", mapper.TransactionID)
	}

	return ynabber.Transaction{
		Account: a,
		ID:      ynabber.ID(id),
		Date:    date,
		Payee:   ynabber.Payee(payee),
		Memo:    t.RemittanceInformationUnstructured,
		Amount:  ynabber.MilliunitsFromAmount(amount),
	}, nil
}

// Nordea implements a specific mapper for Nordea
type Nordea struct{}

// Map t using the Nordea mapper
func (mapper Nordea) Map(a ynabber.Account, t nordigen.Transaction) (ynabber.Transaction, error) {
	amount, err := parseAmount(t)
	if err != nil {
		return ynabber.Transaction{}, err
	}
	date, err := parseDate(t)
	if err != nil {
		return ynabber.Transaction{}, err
	}

	return ynabber.Transaction{
		Account: a,
		ID:      ynabber.ID(t.InternalTransactionId),
		Date:    date,
		Payee:   ynabber.Payee(payeeStripNonAlphanumeric(t.RemittanceInformationUnstructured)),
		Memo:    t.RemittanceInformationUnstructured,
		Amount:  ynabber.MilliunitsFromAmount(amount),
	}, nil
}
