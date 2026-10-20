package enablebanking

import (
	"encoding/base64"
	"strings"

	"github.com/martinohansen/ynabber"
)

func (r Reader) dnbMapper(account AccountInfo, tx EBTransaction) (*ynabber.Transaction, error) {
	transactionID := ""
	if tx.Status == statusBooked {
		transactionID, _ = dnbTransactionID(tx.TransactionID)
	}
	if transactionID == "" {
		transactionID = resolveTransactionID(tx)
	}

	// Use ENABLEBANKING_PAYEE_STRIP or ENABLEBANKING_PAYEE_STRIP_REGEX to clean
	// changing DNB card metadata from displayed payees; these settings do not
	// affect transaction IDs.
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
