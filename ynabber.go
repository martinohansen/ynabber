package ynabber

import (
	"strconv"
	"strings"
	"time"
)

type Ynabber struct {
	Readers []Reader
	Writers []Writer
}

type Reader interface {
	Bulk() ([]Transaction, error)
}

type Writer interface {
	Bulk([]Transaction) error
}

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
	Account Account `json:"account"`
	ID      ID      `json:"id"`
	// Date is the date of the transaction in UTC time
	Date   time.Time  `json:"date"`
	Payee  Payee      `json:"payee"`
	Memo   string     `json:"memo"`
	Amount Milliunits `json:"amount"`
}

func (m Milliunits) String() string {
	return strconv.FormatInt(int64(m), 10)
}

// MilliunitsFromAmount returns a transaction amount in YNABs milliunits format
func MilliunitsFromAmount(amount float64) Milliunits {
	return Milliunits(amount * 1000)
}
