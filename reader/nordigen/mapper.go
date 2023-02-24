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
func (Default) Map(cfg ynabber.Config, account ynabber.Account, t nordigen.Transaction) (ynabber.Transaction, error) {
	amount, err := strconv.ParseFloat(t.TransactionAmount.Amount, 64)
	if err != nil {
		return ynabber.Transaction{}, fmt.Errorf("failed to convert string to float: %w", err)
	}

	date, err := time.Parse("2006-01-02", t.BookingDate)
	if err != nil {
		return ynabber.Transaction{}, fmt.Errorf("failed to parse string to time: %w", err)
	}

	// Get the Payee data source
	payee := ""
	for _, source := range cfg.Nordigen.PayeeSource {
		if payee == "" {
			switch source {
			case "name":
				// Creditor/debtor name can be used as is
				if t.CreditorName != "" {
					payee = t.CreditorName
				} else if t.DebtorName != "" {
					payee = t.DebtorName
				}
			case "unstructured":
				// Unstructured data may need some formatting
				payee = t.RemittanceInformationUnstructured
				payee = payeeStripNonAlphanumeric(payee)
			default:
				return ynabber.Transaction{}, fmt.Errorf("unrecognized PayeeSource: %s", source)
			}
		}
	}

	return ynabber.Transaction{
		Account: account,
		ID:      ynabber.ID(t.TransactionId),
		Date:    date,
		Payee:   ynabber.Payee(payee),
		Memo:    t.RemittanceInformationUnstructured,
		Amount:  ynabber.MilliunitsFromAmount(amount),
	}, nil
}
