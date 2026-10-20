package enablebanking

import (
	"fmt"
	"regexp"
	"strings"
)

// PayeeRegex is a list of regular expressions whose matches are removed from
// payee names. It implements envconfig.Decoder, parsing semicolon-separated
// patterns so regexes may contain commas. Legacy comma-separated lists remain
// supported when the value contains no semicolon.
type PayeeRegex []*regexp.Regexp

// Decode parses value as a semicolon-separated list, or as a legacy
// comma-separated list when no semicolon is present.
func (pr *PayeeRegex) Decode(value string) error {
	if value == "" {
		return nil
	}
	separator := ";"
	if !strings.Contains(value, separator) {
		separator = ","
	}
	for pattern := range strings.SplitSeq(value, separator) {
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
	return strings.Join(parts, ";")
}

var multiSpace = regexp.MustCompile(`\s{2,}`)

// stripRegex removes every match of each pattern from s and trims whitespace.
// Consecutive whitespace left behind by a removal is collapsed into a single
// space so that "foo   bar" becomes "foo bar".
func stripRegex(s string, regexes PayeeRegex) string {
	for _, re := range regexes {
		s = re.ReplaceAllString(s, "")
	}
	s = multiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
