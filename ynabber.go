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

func (p Payee) Parsed() (string, error) {
	reg, err := regexp.Compile("[^a-zA-ZøæåØÆÅ]+")
	if err != nil {
		return "", err
	}
	x := reg.ReplaceAllString(string(p), " ")
	x = strings.ReplaceAll(x, "Den", "")
	x = strings.ReplaceAll(x, "Visa", "")
	x = strings.ReplaceAll(x, "køb", "")
	x = strings.ReplaceAll(x, "DKK", "")
	x = strings.ReplaceAll(x, "Check", "")
	x = strings.ReplaceAll(x, "Check", "")
	x = strings.ReplaceAll(x, "Dankort", "")
	x = strings.ReplaceAll(x, "Nota", "")
	x = strings.ReplaceAll(x, "nota", "")
	return strings.TrimSpace(x), nil
}

func (m Milliunits) String() string {
	return strconv.FormatInt(int64(m), 10)
}

// MilliunitsFromString takes the amount string and the decimal seperator and
// returns the amount in milliunits.
func MilliunitsFromString(amount string, separator string) (Milliunits, error) {
	x, err := strconv.ParseInt(strings.Replace(amount, separator, "", -1), 10, 64)
	x = x * 10
	if err != nil {
		return 0, err
	}
	return Milliunits(x), nil
}

func DataDir () string {
	dataDir := "."
	dataDirLookup, found := os.LookupEnv("YNABBER_DATADIR")
	if found {
		dataDir = path.Clean(dataDirLookup)
	}
	return dataDir
}

// ConfigLookup returns the value of the environment variable with the given
// key. If the variable is not found the fallback string is returned. If the
// fallback string is empty and the key dosent exist in the environment, the
// program exits.
func ConfigLookup(key string, fallback string) (string,) {
	value, found := os.LookupEnv(key)
	if !found {
		if fallback ==  "" {
			 log.Fatalf("environment variable %s not found", key)
		} else {
			return fallback
		}
	}
	return value
}
