package enablebanking

import (
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/martinohansen/ynabber"
)

var (
	dnbVisaSequence               = regexp.MustCompile(`(?i)\b(Visa)\s+\d{6}\b`)
	dnbVolatileRemittancePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bArk\.Ref\s*\*?\s*\d+`),
		regexp.MustCompile(`(?i)\bAut\.\s*\d+`),
		regexp.MustCompile(`(?i)\bVersjon\s+\d+`),
		regexp.MustCompile(`(?i)\bKl\.\s*\d{1,2}[.:]\d{2}`),
		regexp.MustCompile(`(?i)\bDato\s+\d{1,2}[.:]\d{2}(\s+kl\.\s*\d{1,2}[.:]\d{2})?`),
	}
	dnbRemittanceWhitespace = regexp.MustCompile(`\s+`)
	dnbRemittanceCommaSpace = regexp.MustCompile(`\s*,\s*`)
)

func (r Reader) dnbMapper(account AccountInfo, tx EBTransaction) (*ynabber.Transaction, error) {
	mapped, err := r.defaultMapper(account, tx)
	if err != nil || mapped == nil || tx.Status != statusBooked {
		return mapped, err
	}
	if stringFromInterface(tx.EntryReference) != "" || stringFromInterface(tx.ReferenceNumber) != "" {
		return mapped, nil
	}
	if transactionID, ok := dnbTransactionID(tx.TransactionID); ok {
		mapped.ID = ynabber.ID(transactionID)
		return mapped, nil
	}

	tx.RemittanceInformation = normalizeDNBRemittance(tx.RemittanceInformation)
	mapped.ID = ynabber.ID(syntheticTransactionID(tx))
	return mapped, nil
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

func normalizeDNBRemittance(remittance []string) []string {
	normalized := make([]string, len(remittance))
	for i, value := range remittance {
		value = dnbVisaSequence.ReplaceAllString(value, "$1")
		for _, pattern := range dnbVolatileRemittancePatterns {
			value = pattern.ReplaceAllString(value, "")
		}
		value = dnbRemittanceWhitespace.ReplaceAllString(value, " ")
		value = dnbRemittanceCommaSpace.ReplaceAllString(value, ", ")
		for strings.Contains(value, ", ,") {
			value = strings.ReplaceAll(value, ", ,", ",")
		}
		normalized[i] = strings.Trim(strings.TrimSpace(value), ",")
	}
	return normalized
}
