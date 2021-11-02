package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/writer/ynab"
)

func main() {
	sleeps := ynabber.ConfigLookup("YNABBER_INTERVAL", "5m")
	sleep, err := time.ParseDuration(sleeps)
	if err != nil {
		log.Fatalf("Failed to parse YNABBER_INTERVAL: %s", err)
	}

	for {
		err = run()
		if err != nil {
			log.Fatalf("Run failed with: %s", err)
		} else {
			log.Printf("Run succeeded")
		}
		log.Printf("Waiting %s before running again...", sleeps)
        time.Sleep(sleep)
	}
}


func run() error {
	var transactions []ynabber.Transaction

	var readerList []string
	r := ynabber.ConfigLookup("YNABBER_READERS", "[\"nordigen\"]")
	err := json.Unmarshal([]byte(r), &readerList)
	if err != nil {
		return fmt.Errorf("couldn't to parse readers: %s", err)
	}

	var writerList []string
	w := ynabber.ConfigLookup("YNABBER_WRITERS", "[\"ynab\"]")
	err = json.Unmarshal([]byte(w), &writerList)
	if err != nil {
		return fmt.Errorf("couldn't to parse writers: %s", err)
	}

	for _, reader := range readerList {
		log.Printf("Reading from %s", reader)
		switch reader {
		case "nordigen":
			t, err := nordigen.BulkReader()
			if err != nil {
				return err
			}
			transactions = append(transactions, t...)
		}
	}

	for _, writer := range writerList {
		log.Printf("Writing to %s", writer)
		switch writer {
		case "ynab":
			err := ynab.BulkWriter(transactions)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
