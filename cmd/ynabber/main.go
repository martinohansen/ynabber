package main

import (
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

	y := ynabber.NewYnabber(&cfg)
	for _, reader := range cfg.Readers {
		switch reader {
		case "nordigen":
			nordigenReader, err := nordigen.NewReader(cfg.DataDir)
			if err != nil {
				log.Fatal(logger, "creating nordigen reader", "error", err)
			}
			y.Readers = append(y.Readers, nordigenReader)
		case "enablebanking":
			enableBankingReader, err := enablebanking.NewReader(logger, cfg.DataDir)
			if err != nil {
				log.Fatal(logger, "creating enablebanking reader", "error", err)
			}
			y.Readers = append(y.Readers, enableBankingReader)
		case "generator":
			generatorReader, err := generator.NewReader()
			if err != nil {
				log.Fatal(logger, "creating generator reader", "error", err)
			}
			y.Readers = append(y.Readers, generatorReader)
		default:
			log.Fatal(logger, "unknown reader", "name", reader)
		}
	}
	for _, writer := range cfg.Writers {
		switch writer {
		case "ynab":
			ynabWriter, err := ynab.NewWriter()
			if err != nil {
				log.Fatal(logger, "creating ynab writer", "error", err)
			}
			y.Writers = append(y.Writers, ynabWriter)
		case "json":
			y.Writers = append(y.Writers, json.Writer{})
		default:
			log.Fatal(logger, "unknown writer", "name", writer)
		}
	}

	// Run Ynabber
	if err := y.Run(); err != nil {
		log.Fatal(logger, err.Error())
	}
}
