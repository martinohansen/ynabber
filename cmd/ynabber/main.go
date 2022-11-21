package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/writer/ynab"
)

func main() {
	// Read config from env
	var cfg ynabber.Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Check that some values are valid
	cfg.YNAB.Cleared = strings.ToLower(cfg.YNAB.Cleared)
	if cfg.YNAB.Cleared != "cleared" &&
		cfg.YNAB.Cleared != "uncleared" &&
		cfg.YNAB.Cleared != "reconciled" {
		log.Fatal("YNAB_CLEARED must be one of cleared, uncleared or reconciled")
	}

	// Handle movement of config options and warn users
	if cfg.Nordigen.PayeeStrip == nil {
		if cfg.PayeeStrip != nil {
			log.Printf("Config YNABBER_PAYEE_STRIP is depreciated, please use NORDIGEN_PAYEE_STRIP instead")
			cfg.Nordigen.PayeeStrip = cfg.PayeeStrip
		}
	}
	if cfg.YNAB.AccountMap == nil {
		if cfg.Nordigen.AccountMap != nil {
			log.Printf("Config NORDIGEN_ACCOUNTMAP is depreciated, please use YNAB_ACCOUNTMAP instead")
			cfg.YNAB.AccountMap = cfg.Nordigen.AccountMap
		}
	}

	if cfg.Debug {
		log.Printf("Config: %+v\n", cfg)
	}

	for {
		err = run(cfg)
		if err != nil {
			panic(err)
		} else {
			log.Printf("Run succeeded")
		}
		if cfg.Interval > 0 {
			log.Printf("Waiting %s before running again...", cfg.Interval)
			time.Sleep(cfg.Interval)
		} else {
			os.Exit(0)
		}
	}
}

func run(cfg ynabber.Config) error {
	var transactions []ynabber.Transaction

	for _, reader := range cfg.Readers {
		log.Printf("Reading from %s", reader)
		switch reader {
		case "nordigen":
			t, err := nordigen.BulkReader(cfg)
			if err != nil {
				return fmt.Errorf("couldn't read from nordigen: %w", err)
			}
			transactions = append(transactions, t...)
		}
	}

	for _, writer := range cfg.Writers {
		log.Printf("Writing to %s", writer)
		switch writer {
		case "ynab":
			err := ynab.BulkWriter(cfg, transactions)
			if err != nil {
				return fmt.Errorf("couldn't write to ynab: %w", err)
			}
		}
	}
	return nil
}
