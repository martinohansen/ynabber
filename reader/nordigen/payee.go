package nordigen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/frieser/nordigen-go-lib/v2"
)

// PayeeRegex is a list of regular expressions whose matches are removed from
// payee names. It implements envconfig.Decoder, parsing a comma-separated list
// of patterns and compiling each one. Patterns themselves cannot contain a
// literal comma.
type PayeeRegex []*regexp.Regexp

// Decode parses value as a comma-separated list of regex patterns.
func (pr *PayeeRegex) Decode(value string) error {
	if value == "" {
		return nil
	}
	for pattern := range strings.SplitSeq(value, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		*pr = append(*pr, re)
	}
	return nil
}

// String returns the source patterns joined by commas.
func (pr PayeeRegex) String() string {
	parts := make([]string, len(pr))
	for i, re := range pr {
		parts[i] = re.String()
	}
	return strings.Join(parts, ",")
}

// stripRegex removes every match of each pattern from s and trims whitespace.
// Consecutive whitespace left behind by a removal is collapsed into a single
// space so that "foo   bar" becomes "foo bar".
func stripRegex(s string, regexes PayeeRegex) string {
	for _, re := range regexes {
		s = re.ReplaceAllString(s, "")
	}
	s = collapseSpaces(s)
	return strings.TrimSpace(s)
}

var multiSpace = regexp.MustCompile(`\s{2,}`)

func collapseSpaces(s string) string {
	return multiSpace.ReplaceAllString(s, " ")
}

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
