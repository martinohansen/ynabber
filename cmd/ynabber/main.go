package main

import (
	"log"
	"log/slog"
	"os"

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

	y := ynabber.NewYnabber(&cfg)
	for _, reader := range cfg.Readers {
		switch reader {
		case "nordigen":
			y.Readers = append(y.Readers, nordigen.NewReader(&cfg))
		default:
			log.Fatalf("Unknown reader: %s", reader)
		}
	}
	for _, writer := range cfg.Writers {
		switch writer {
		case "ynab":
			y.Writers = append(y.Writers, ynab.NewWriter(&cfg))
		case "json":
			y.Writers = append(y.Writers, json.Writer{})
		default:
			log.Fatalf("Unknown writer: %s", writer)
		}
	}

	// Run Ynabber
	y.Run()
}
