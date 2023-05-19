package nordigen

import (
	"fmt"
	"strconv"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

type Mapper interface {
	Map(ynabber.Config, ynabber.Account, nordigen.Transaction) ynabber.Transaction
}

// Default mapping for all banks unless a more specific mapping exists
type Default struct{}

// Map Nordigen transactions using the default mapper
func (Default) Map(cfg ynabber.Config, a ynabber.Account, t nordigen.Transaction) (ynabber.Transaction, error) {
	amount, err := strconv.ParseFloat(t.TransactionAmount.Amount, 64)
	if err != nil {
		return ynabber.Transaction{}, fmt.Errorf("failed to convert string to float: %w", err)
	}

	date, err := time.Parse("2006-01-02", t.BookingDate)
	if err != nil {
		return ynabber.Transaction{}, fmt.Errorf("failed to parse string to time: %w", err)
	}

	// Get the Payee from the first data source that returns data in the order
	// defined by config
	payee := ""
	for _, source := range cfg.Nordigen.PayeeSource {
		if payee == "" {
			switch source {
			// Unstructured should properly have been called "remittance" but
			// its not. Some banks use this field as Payee.
			case "unstructured":
				payee = t.RemittanceInformationUnstructured
				// Unstructured data may need some formatting, some banks
				// inserts the amount and date which will cause every
				// transaction to create a new Payee
				payee = payeeStripNonAlphanumeric(payee)

			// Name is using either creditor or debtor as the payee
			case "name":
				// Use either one
				if t.CreditorName != "" {
					payee = t.CreditorName
				} else if t.DebtorName != "" {
					payee = t.DebtorName
				}

			// Additional uses AdditionalInformation as payee
			case "additional":
				payee = t.AdditionalInformation
			default:
				return ynabber.Transaction{}, fmt.Errorf("unrecognized PayeeSource: %s", source)
			}
		}
	}

	// Get the ID from the first data source that returns data as defined in the
	// config
	var id string
	switch cfg.Nordigen.TransactionID {
	case "InternalTransactionId":
		id = t.InternalTransactionId
	default:
		id = t.TransactionId
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
