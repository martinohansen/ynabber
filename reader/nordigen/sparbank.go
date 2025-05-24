package nordigen

import (
	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

// srBankMapper handles SPAREBANK_SR_BANK_SPRONO22 transactions specifically.
// The bank changes TransactionId over time, ProprietaryBankTransactionCode does
// not.
func (r Reader) srBankMapper(a ynabber.Account, t nordigen.Transaction) (*ynabber.Transaction, error) {
	transaction, err := r.defaultMapper(a, t)
	if err != nil {
		return nil, err
	}
	if transaction == nil {
		return nil, nil
	}

	// Override the ID with the ProprietaryBankTransactionCode field.
	transaction.ID = ynabber.ID(t.ProprietaryBankTransactionCode)

	return transaction, nil
}
