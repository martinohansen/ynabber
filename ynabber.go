package ynabber

import (
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

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

func DataDir() string {
	dataDir := "."
	dataDirLookup, found := os.LookupEnv("YNABBER_DATADIR")
	if found {
		dataDir = path.Clean(dataDirLookup)
	}
	return dataDir
}

// ConfigLookup returns the value of the environment variable with the given
// key. If the variable is not found the fallback string is returned. If the
// fallback string is empty and the key doesn't exist in the environment, the
// program exits.
func ConfigLookup(key string, fallback string) string {
	value, found := os.LookupEnv(key)
	if !found {
		if fallback == "" {
			log.Fatalf("environment variable %s not found", key)
		} else {
			return fallback
		}
	}
	return value
}

// ConfigDebug returns true if the YNABBER_DEBUG environment variable is set.
func ConfigDebug() bool {
	_, found := os.LookupEnv("YNABBER_DEBUG")
	if found {
		return true
	} else {
		return false
	}
}
