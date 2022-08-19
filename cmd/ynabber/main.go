package main

import (
	"fmt"
	"log"
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
		log.Printf("Waiting %s before running again...", cfg.Interval)
		time.Sleep(cfg.Interval)
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
