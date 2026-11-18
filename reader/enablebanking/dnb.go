package enablebanking

import (
	"strings"

	"github.com/martinohansen/ynabber"
)

// isDNB reports whether aspsp is DNB. ASPSP names are matched
// case-insensitively, as configured in ENABLEBANKING_ASPSP.
func isDNB(aspsp string) bool {
	return strings.EqualFold(strings.TrimSpace(aspsp), "DNB")
}

// dnbMapper always uses the synthetic transaction ID for DNB, ignoring
// entry_reference and reference_number. DNB changes the remittance text after
// booking (e.g. "Giro 5924, ..." becomes "Giro, ..."), and the same transaction
// can later arrive with a reference_number. Either change would create a new
// Ynab transaction, so the remittance is hashed with DNB_SYN_ID_REGEX removed.
func (r Reader) dnbMapper(account AccountInfo, tx EBTransaction) (*ynabber.Transaction, error) {
	transaction, err := r.defaultMapper(account, tx)
	if err != nil {
		return nil, err
	}
	if transaction == nil {
		return nil, nil
	}

	transaction.ID = ynabber.ID(syntheticTransactionIDWithRegex(tx, r.Config.DNBSynIDRegex))
	return transaction, nil
}
