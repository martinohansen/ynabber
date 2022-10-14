package nordigen

import (
	"fmt"
	"strconv"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

// nordigenTransaction is a transient type with the purpose of mapping Nordigen
// fields to Ynabber
type nordigenTransaction struct {
	id     string
	date   string
	payee  string
	memo   string
	amount string
}

// bankToNordigen maps t to nordigenTransaction based on BankID
func bankToNordigen(cfg ynabber.Config, t nordigen.AccountTransactions) (x []nordigenTransaction) {
	for _, v := range t.Transactions.Booked {
		switch cfg.Nordigen.BankID {

		case "NORDEA_NDEADKKK":
			x = append(x, nordigenTransaction{
				id:     v.TransactionId,
				date:   v.BookingDate,
				payee:  v.RemittanceInformationUnstructured,
				memo:   v.RemittanceInformationUnstructured,
				amount: v.TransactionAmount.Amount,
			})

		case "S_PANKKI_SBANFIHH":
			memo := v.RemittanceInformationUnstructured
			payee := memo
			if v.DebtorName != "" {
				payee = v.DebtorName
			} else if v.CreditorName != "" {
				payee = v.CreditorName
			}
			x = append(x, nordigenTransaction{
				id:     v.TransactionId,
				date:   v.BookingDate,
				payee:  payee,
				memo:   memo,
				amount: v.TransactionAmount.Amount,
			})

		// Default to the original implementation for now, even is this is not
		// optimal it will bring no breaking changes. We can always change the
		// default at a later time if we want.
		default:
			x = append(x, nordigenTransaction{
				id:     v.TransactionId,
				date:   v.BookingDate,
				payee:  v.RemittanceInformationUnstructured,
				memo:   v.RemittanceInformationUnstructured,
				amount: v.TransactionAmount.Amount,
			})
		}
	}
	return x
}

func transactionsToYnabber(cfg ynabber.Config, account ynabber.Account, t nordigen.AccountTransactions) (x []ynabber.Transaction, err error) {
	transactions := bankToNordigen(cfg, t)
	for _, v := range transactions {

		memo := v.memo

		amount, err := strconv.ParseFloat(v.amount, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert string to float: %w", err)
		}
		milliunits := ynabber.MilliunitsFromAmount(amount)

		date, err := time.Parse(timeLayout, v.date)
		if err != nil {
			return nil, fmt.Errorf("failed to parse string to time: %w", err)
		}

		// Append transaction
		x = append(x, ynabber.Transaction{
			Account: account,
			ID:      ynabber.ID(ynabber.IDFromString(v.id)),
			Date:    date,
			Payee:   ynabber.Payee(memo),
			Memo:    memo,
			Amount:  milliunits,
		})
	}
	return x, nil
}
