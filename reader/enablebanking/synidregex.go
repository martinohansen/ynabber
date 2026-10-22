package enablebanking

import (
	"fmt"
	"regexp"
	"strings"
)

// SynIDRegex is a list of patterns removed only from synthetic-ID input.
type SynIDRegex []*regexp.Regexp

// Decode parses value as a semicolon-separated list of regex patterns.
func (sr *SynIDRegex) Decode(value string) error {
	if value == "" {
		return nil
	}
	for pattern := range strings.SplitSeq(value, ";") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		*sr = append(*sr, re)
	}
	return nil
}

// String returns the source patterns joined by semicolons.
func (sr SynIDRegex) String() string {
	parts := make([]string, len(sr))
	for i, re := range sr {
		parts[i] = re.String()
	}
	return strings.Join(parts, ";")
}