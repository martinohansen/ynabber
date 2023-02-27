package ynabber

import (
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

type Payee string

// Strip removes the elements from s from the payee
func (p Payee) Strip(s []string) Payee {
	x := string(p)
	for _, strip := range s {
		x = strings.ReplaceAll(x, strip, "")
	}
	return Payee(strings.TrimSpace(x))
}

type Milliunits int64

// Negate changes the sign of m to the opposite
func (m Milliunits) Negate() Milliunits {
	return m * -1
}

type Transaction struct {
	Account Account
	ID      ID
	Date    time.Time
	Payee   Payee
	Memo    string
	Amount  Milliunits
}

type Ynabber interface {
	bulkReader() ([]Transaction, error)
	bulkWriter([]Transaction) error
}

func (m Milliunits) String() string {
	return strconv.FormatInt(int64(m), 10)
}

// MilliunitsFromAmount returns a transaction amount in YNABs milliunits format
func MilliunitsFromAmount(amount float64) Milliunits {
	return Milliunits(amount * 1000)
}
