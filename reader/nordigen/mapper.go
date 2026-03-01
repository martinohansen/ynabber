package nordigen

import (
	"fmt"
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

	// SPAREBANK SR BANK requires the proprietaryBankTransactionCode as the
	// primary identifier since the other Nordigen ids are unstable.
	case r.Config.BankID == "SPAREBANK_SR_BANK_SPRONO22":
		return r.srBankMapper(a, t)

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
	date, err := time.Parse(ynabber.DateFormat, t.BookingDate)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse string to time: %w", err)
	}
	return date, nil
}

// defaultMapper is generic and tries to identify the appropriate mapping
func (r Reader) defaultMapper(a ynabber.Account, t nordigen.Transaction) (*ynabber.Transaction, error) {
	payee := newPayee(t, r.Config.PayeeSource)
	TransactionID := r.Config.TransactionID // TODO(Martin): Parse into enum at startup

	amount, err := parseAmount(t)
	if err != nil {
		return nil, err
	}
	date, err := parseDate(t)
	if err != nil {
		return nil, err
	}

	// Remove elements in payee that is defined in config
	if r.Config.PayeeStrip != nil {
		payee.value = strip(payee.value, r.Config.PayeeStrip)
	}

	// Set the transaction ID according to config
	var id string
	switch TransactionID {
	case "InternalTransactionId":
		id = t.InternalTransactionId
	case "TransactionId":
		id = t.TransactionId
	case "ProprietaryBankTransactionCode":
		// In the current nordigen-go-lib implementation the proprietary
		// transaction code is mapped to BankTransactionCode.
		id = t.BankTransactionCode
	default:
		return nil, fmt.Errorf("unrecognized TransactionID: %s", TransactionID)
	}

	memo := payee.raw
	if trimmed := strings.TrimSpace(t.RemittanceInformationUnstructured); trimmed != "" {
		memo = trimmed
	}
	return &ynabber.Transaction{
		Account: a,
		ID:      ynabber.ID(id),
		Date:    date,
		Payee:   payee.value,
		Memo:    memo,
		Amount:  ynabber.MilliunitsFromAmount(amount),
	}, nil
}
