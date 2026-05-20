package ynabber

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type Account struct {
	ID   ID
	Name string
	IBAN string
}

type ID string

// Milliunits represents represents 1/1000 of a currency unit
type Milliunits int64

// Negate changes the sign of m to the opposite
func (m Milliunits) Negate() Milliunits {
	return m * -1
}

func (m Milliunits) String() string {
	return strconv.FormatInt(int64(m), 10)
}

// MilliunitsFromAmount returns a currency amount in milliunits. It uses
// math.Round to avoid float64 truncation errors (e.g. 65.02*1000 truncates
// to 65019 instead of 65020).
func MilliunitsFromAmount(amount float64) Milliunits {
	return Milliunits(math.Round(amount * 1000))
}

// MilliunitsFromString parses a decimal amount string into milliunits using
// integer arithmetic only, avoiding float64 precision loss. For example
// "65.02" produces 65020 milliunits.
//
// Inputs with more than 3 fractional digits are rejected to prevent silent
// truncation of sub-milliunit precision.
func MilliunitsFromString(s string) (Milliunits, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty amount string")
	}

	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	} else if s[0] == '+' {
		s = s[1:]
	}

	var intPart, fracPart int64
	if dot := strings.Index(s, "."); dot >= 0 {
		ip, err := strconv.ParseInt(s[:dot], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing integer part: %w", err)
		}
		intPart = ip

		frac := s[dot+1:]
		if len(frac) > 3 {
			return 0, fmt.Errorf("amount %q has more than 3 fractional digits", s)
		}
		fp, err := strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing fractional part: %w", err)
		}
		fracPart = fp
		for i := len(frac); i < 3; i++ {
			fracPart *= 10
		}
	} else {
		ip, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing integer part: %w", err)
		}
		intPart = ip
	}

	result := intPart*1000 + fracPart
	if neg {
		result = -result
	}
	return Milliunits(result), nil
}

// Transaction represents a financial transaction
type Transaction struct {
	Account Account `json:"account"`
	ID      ID      `json:"id"`
	// Date is the date of the transaction in UTC time
	Date   time.Time  `json:"date"`
	Payee  string     `json:"payee"`
	Memo   string     `json:"memo"`
	Amount Milliunits `json:"amount"`
}
