package wealthreader

import (
	"fmt"
	"regexp"
	"strings"
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

var multiSpace = regexp.MustCompile(`\s{2,}`)

// stripRegex removes every match of each pattern from s and trims whitespace.
func stripRegex(s string, regexes PayeeRegex) string {
	for _, re := range regexes {
		s = re.ReplaceAllString(s, "")
	}
	s = multiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// strip removes each string in strips from s.
func strip(s string, strips []string) string {
	for _, part := range strips {
		s = strings.ReplaceAll(s, part, "")
	}
	return strings.TrimSpace(s)
}
