package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/carlmjohnson/versioninfo"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/internal/log"
	"github.com/martinohansen/ynabber/reader/enablebanking"
	"github.com/martinohansen/ynabber/reader/generator"
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/reader/wealthreader"
	"github.com/martinohansen/ynabber/writer/actual"
	"github.com/martinohansen/ynabber/writer/json"
	"github.com/martinohansen/ynabber/writer/ynab"
)

func setupLogging(logLevel, logFormat string) error {
	programLevel, err := log.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("parsing log level: %w", err)
	}

	// Add source information for debug or lower
	addSource := programLevel <= slog.LevelDebug

	logger, err := log.NewLoggerWithTrace(programLevel, addSource, logFormat)
	if err != nil {
		return fmt.Errorf("creating logger: %w", err)
	}
	slog.SetDefault(logger)
	return nil
}

func main() {
	// Read config from env
	var cfg ynabber.Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		fmt.Printf("error processing config: %v\n", err)
		os.Exit(1)
	}

	err = setupLogging(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		fmt.Printf("error setting up logging: %v\n", err)
		os.Exit(1)
	}

	logger := slog.Default()
	logger.Info("starting...", "version", versioninfo.Short())

	var readers []ynabber.Reader
	var writers []ynabber.Writer
	for _, reader := range cfg.Readers {
		switch reader {
		case "nordigen":
			nordigenReader, err := nordigen.NewReader(cfg.DataDir)
			if err != nil {
				log.Fatal(logger, "creating nordigen reader", "error", err)
			}
			readers = append(readers, nordigenReader)
		case "enablebanking":
			enableBankingReader, err := enablebanking.NewReader(logger, cfg.DataDir)
			if err != nil {
				log.Fatal(logger, "creating enablebanking reader", "error", err)
			}
			readers = append(readers, enableBankingReader)
		case "wealthreader":
			wealthreaderReader, err := wealthreader.NewReader(logger, cfg.DataDir)
			if err != nil {
				log.Fatal(logger, "creating wealthreader reader", "error", err)
			}
			readers = append(readers, wealthreaderReader)
		case "generator":
			generatorReader, err := generator.NewReader()
			if err != nil {
				log.Fatal(logger, "creating generator reader", "error", err)
			}
			readers = append(readers, generatorReader)
		default:
			log.Fatal(logger, "unknown reader", "name", reader)
		}
	}
	for _, writer := range cfg.Writers {
		switch writer {
		case "actual":
			actualWriter, err := actual.NewWriter()
			if err != nil {
				log.Fatal(logger, "creating actual writer", "error", err)
			}
			writers = append(writers, actualWriter)
		case "ynab":
			ynabWriter, err := ynab.NewWriterFromEnv()
			if err != nil {
				log.Fatal(logger, "creating ynab writer", "error", err)
			}
			writers = append(writers, ynabWriter)
		case "json":
			writers = append(writers, json.Writer{})
		default:
			log.Fatal(logger, "unknown writer", "name", writer)
		}
	}

	// Run Ynabber
	y, err := ynabber.New(readers, writers, logger)
	if err != nil {
		log.Fatal(logger, "creating pipeline", "error", err)
	}
	if err := y.Run(context.Background()); err != nil {
		log.Fatal(logger, "pipeline failed", "error", err)
	}
}
