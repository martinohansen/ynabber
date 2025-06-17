package nordigen

import (
	"regexp"
	"strings"

	"github.com/frieser/nordigen-go-lib/v2"
)

// Payee represents the payee of a transaction. It can come from multiple fields
// based on the bank and the configuration by the user.
type Payee struct {
	value string

	// Raw is value before any transformations
	raw string
}

// Combine p and other by concatenating value and raw with s
func (p Payee) Combine(other Payee, s string) Payee {
	p.value = strings.TrimSpace(p.value + s + other.value)
	p.raw = strings.TrimSpace(p.raw + s + other.raw)
	return p
}

// newPayee returns the first group of sources which yields a result. Each
// source in a group is concatenated with a space. If no sources yields a result
// an empty string is returned.
func newPayee(t nordigen.Transaction, groups PayeeGroups) Payee {
	payees := []Payee{}
	for _, group := range groups {
		for _, source := range group {
			value := payeeValue(t, source)
			if value == "" {
				continue
			}

			switch source {
			case Remittance:
				// Strip non-alphanumeric characters and new lines from
				// remittance payee values. These have been notoriously messy.
				payees = append(payees, Payee{
					value: strings.ReplaceAll(stripNonAlphanumeric(value), "\n", " "),
					raw:   value,
				})
			default:
				payees = append(payees, Payee{
					value: value,
					raw:   value,
				})
			}

		}
		if len(payees) > 0 {
			// Combine payees in a group
			for i := 1; i < len(payees); i++ {
				payees[0] = payees[0].Combine(payees[i], " ")
			}
			return payees[0]
		}
	}
	return Payee{}
}

// payeeValue returns the value of the given source
func payeeValue(t nordigen.Transaction, source PayeeSource) string {
	switch source {
	case Remittance:
		// The remittance info can be in one or more of four fields, i've not
		// seen it in all of them yet but it has varied between which field over
		// time for my bank. Prefer structured over unstructured and array last
		// in each case.
		if t.RemittanceInformationStructured != "" {
			return t.RemittanceInformationStructured
		} else if t.RemittanceInformationStructuredArray != nil {
			return strings.Join(t.RemittanceInformationStructuredArray, " ")
		}
		if t.RemittanceInformationUnstructured != "" {
			return t.RemittanceInformationUnstructured
		} else if t.RemittanceInformationUnstructuredArray != nil {
			return strings.Join(t.RemittanceInformationUnstructuredArray, " ")
		}

	case Name:
		// Use either creditor or debtor as the payee
		if t.CreditorName != "" {
			return t.CreditorName
		} else if t.DebtorName != "" {
			return t.DebtorName
		}
		return ""

	case Additional:
		// Use AdditionalInformation as payee
		return t.AdditionalInformation
	}
	return ""
}

// stripNonAlphanumeric removes all non-alphanumeric characters from s
func stripNonAlphanumeric(s string) string {
	reg := regexp.MustCompile(`[^\p{L}]+`)
	return strings.TrimSpace(reg.ReplaceAllString(s, " "))
}

// Strip removes each string in strips from s
func strip(s string, strips []string) string {
	for _, strip := range strips {
		s = strings.ReplaceAll(s, strip, "")
	}
	return strings.TrimSpace(s)
}
