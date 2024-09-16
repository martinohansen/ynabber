package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/carlmjohnson/versioninfo"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/writer/json"
	"github.com/martinohansen/ynabber/writer/ynab"
)

func setupLogging(debug bool) {
	programLevel := slog.LevelInfo
	if debug {
		programLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(
		os.Stderr, &slog.HandlerOptions{
			Level: programLevel,
		}))
	slog.SetDefault(logger)
}

func main() {
	// Read config from env
	var cfg ynabber.Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	setupLogging(cfg.Debug)
	slog.Info("starting...", "version", versioninfo.Short())

	ynabber := ynabber.Ynabber{}
	for _, reader := range cfg.Readers {
		switch reader {
		case "nordigen":
			ynabber.Readers = append(ynabber.Readers, nordigen.NewReader(&cfg))
		default:
			log.Fatalf("Unknown reader: %s", reader)
		}
	}
	for _, writer := range cfg.Writers {
		switch writer {
		case "ynab":
			ynabber.Writers = append(ynabber.Writers, ynab.NewWriter(&cfg))
		case "json":
			ynabber.Writers = append(ynabber.Writers, json.Writer{})
		default:
			log.Fatalf("Unknown writer: %s", writer)
		}
	}

	for {
		start := time.Now()
		err = run(ynabber)
		if err != nil {
			panic(err)
		} else {
			slog.Info("run succeeded", "in", time.Since(start))
			if cfg.Interval > 0 {
				slog.Info("waiting for next run", "in", cfg.Interval)
				time.Sleep(cfg.Interval)
			} else {
				os.Exit(0)
			}
		}
	}
}

func run(y ynabber.Ynabber) error {
	var transactions []ynabber.Transaction

	// Read transactions from all readers
	for _, reader := range y.Readers {
		t, err := reader.Bulk()
		if err != nil {
			return fmt.Errorf("reading: %w", err)
		}
		transactions = append(transactions, t...)
	}

	// Write transactions to all writers
	for _, writer := range y.Writers {
		err := writer.Bulk(transactions)
		if err != nil {
			return fmt.Errorf("writing: %w", err)
		}
	}
	return nil
}
