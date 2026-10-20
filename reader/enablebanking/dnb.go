package enablebanking

import (
	"encoding/base64"
	"strings"

	"github.com/martinohansen/ynabber"
)

// dnbMapper uses EnableBanking's documented reference fields first, then a
// validated booked transaction_id. The API describes transaction_id as
// potentially volatile; observed booked DNB IDs remained stable across fetches.
// See https://enablebanking.com/docs/api/reference/#transaction.
// dnbMapper chooses the DNB transaction identifier and delegates shared
// transaction mapping, including configured payee cleanup, to defaultMapper.
// For cleaner displayed payees, consider ENABLEBANKING_PAYEE_STRIP=Varekjøp
// with semicolon-delimited ENABLEBANKING_PAYEE_STRIP_REGEX rules for Visa and
// changing authorization/date annotations. Payee cleanup does not affect IDs.
func (r Reader) dnbMapper(account AccountInfo, tx EBTransaction) (*ynabber.Transaction, error) {
	transactionID := stringFromInterface(tx.EntryReference)
	if transactionID == "" {
		transactionID = stringFromInterface(tx.ReferenceNumber)
	}
	if transactionID == "" && tx.Status == statusBooked {
		transactionID, _ = dnbTransactionID(tx.TransactionID)
	}
	if transactionID == "" {
		transactionID = syntheticTransactionID(tx)
	}

	return r.defaultMapper(account, tx, transactionID)
}

func dnbTransactionID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(value)
	}
	if err != nil {
		return "", false
	}
	parts := strings.Split(string(decoded), "_")
	if len(parts) != 3 || parts[0] != "0116" || !asciiDigits(parts[1]) || !asciiDigits(parts[2]) {
		return "", false
	}
	return value, true
}

func asciiDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
