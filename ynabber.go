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

func (input *AccountMap) Decode(value string) error {
	err := json.Unmarshal([]byte(value), &input)
	if err != nil {
		return err
	}
	return nil
}

// Config is loaded from the environment during execution with cmd/ynabber
type Config struct {
	// DataDir is the path for storing files e.g. Nordigen authorization
	DataDir string `envconfig:"YNABBER_DATADIR" default:"."`

	// Debug prints more log statements
	Debug bool `envconfig:"YNABBER_DEBUG" default:"false"`

	// Interval is how often to execute the read/write loop
	Interval time.Duration `envconfig:"YNABBER_INTERVAL" default:"5m"`

	// Readers is a list of sources to read transactions from
	Readers []string `envconfig:"YNABBER_READERS" default:"nordigen"`

	// Writers is a list of destinations to write transactions from
	Writers []string `envconfig:"YNABBER_WRITERS" default:"ynab"`

	// Nordigen related settings
	Nordigen struct {
		// AccountMap of Nordigen account IDs to YNAB account IDs
		AccountMap AccountMap `envconfig:"NORDIGEN_ACCOUNTMAP"`

		// BankID is used to create requisition
		BankID string `envconfig:"NORDIGEN_BANKID"`

		// SecretID is used to create requisition
		SecretID string `envconfig:"NORDIGEN_SECRET_ID"`

		// SecretKey is used to create requisition
		SecretKey string `envconfig:"NORDIGEN_SECRET_KEY"`
	}

	// YNAB related settings
	YNAB struct {
		// PayeeStrip is a list of words to remove from the Payee before sending
		// to YNAB
		PayeeStrip []string `envconfig:"YNABBER_PAYEE_STRIP"`
	}
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
