package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/carlmjohnson/versioninfo"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/reader/generator"
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/writer/json"
	"github.com/martinohansen/ynabber/writer/ynab"
)

func setupLogging(debug bool) {
	programLevel := slog.LevelInfo
	addSource := false
	if debug {
		programLevel = slog.LevelDebug
		addSource = true
	}
	logger := slog.New(slog.NewTextHandler(
		os.Stderr, &slog.HandlerOptions{
			Level:     programLevel,
			AddSource: addSource,
		}))
	slog.SetDefault(logger)
}

func Exit(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}

func main() {
	// Read config from env
	var cfg ynabber.Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		Exit(err.Error())
	}

	setupLogging(cfg.Debug)
	slog.Info("starting...", "version", versioninfo.Short())

	y := ynabber.NewYnabber(&cfg)
	for _, reader := range cfg.Readers {
		switch reader {
		case "nordigen":
			nordigenReader, err := nordigen.NewReader(cfg.DataDir)
			if err != nil {
				Exit(fmt.Sprintf("Failed to create nordigen reader: %v", err))
			}
			y.Readers = append(y.Readers, nordigenReader)
		case "generator":
			generatorReader, err := generator.NewReader()
			if err != nil {
				Exit(fmt.Sprintf("Failed to create generator reader: %v", err))
			}
			y.Readers = append(y.Readers, generatorReader)
		default:
			Exit(fmt.Sprintf("Unknown reader: %s", reader))
		}
	}
	for _, writer := range cfg.Writers {
		switch writer {
		case "ynab":
			ynabWriter, err := ynab.NewWriter()
			if err != nil {
				Exit(fmt.Sprintf("Failed to create ynab writer: %v", err))
			}
			y.Writers = append(y.Writers, ynabWriter)
		case "json":
			y.Writers = append(y.Writers, json.Writer{})
		default:
			Exit(fmt.Sprintf("Unknown writer: %s", writer))
		}
	}

	// Run Ynabber
	if err := y.Run(); err != nil {
		Exit(fmt.Sprintf("Unexpected error: %v", err))
	}
}
