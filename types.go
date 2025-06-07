package ynabber

import (
	"strconv"
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

// MilliunitsFromAmount returns a currency amount in milliunits
func MilliunitsFromAmount(amount float64) Milliunits {
	return Milliunits(amount * 1000)
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
