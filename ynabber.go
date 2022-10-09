package ynabber

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AccountMap map[string]string

// Decode implements `envconfig.Decoder` for AccountMap to decode JSON properly
func (input *AccountMap) Decode(value string) error {
	err := json.Unmarshal([]byte(value), &input)
	if err != nil {
		return err
	}
	return nil
}

type Account struct {
	ID   ID
	Name string
}

type ID uuid.UUID

type Payee string

type Milliunits int64

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

func IDFromString(id string) uuid.UUID {
	x, err := uuid.Parse(id)
	if err != nil {
		return uuid.New()
	}
	return x
}

// Parsed removes all non-alphanumeric characters and elements of strips from p
func (p Payee) Parsed(strips []string) (string, error) {
	reg := regexp.MustCompile(`[^\p{L}]+`)
	x := reg.ReplaceAllString(string(p), " ")

	for _, strip := range strips {
		x = strings.ReplaceAll(x, strip, "")
	}

	return strings.TrimSpace(x), nil
}

func (m Milliunits) String() string {
	return strconv.FormatInt(int64(m), 10)
}

// MilliunitsFromAmount returns a transaction amount in YNABs milliunits format
func MilliunitsFromAmount(amount float64) Milliunits {
	return Milliunits(amount * 1000)
}
