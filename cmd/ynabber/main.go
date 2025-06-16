package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/carlmjohnson/versioninfo"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/internal/log"
	"github.com/martinohansen/ynabber/reader/generator"
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/writer/json"
	"github.com/martinohansen/ynabber/writer/ynab"
)

func setupLogging(logLevel, logFormat string) {
	programLevel, err := log.ParseLevel(logLevel)
	if err != nil {
		Exit(fmt.Sprintf("Error parsing log level: %s", err))
	}

	// Add source information for debug or lower
	addSource := programLevel <= slog.LevelDebug

	logger, err := log.NewLoggerWithTrace(programLevel, addSource, logFormat)
	if err != nil {
		Exit(fmt.Sprintf("Error creating logger: %s", err))
	}
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

	setupLogging(cfg.LogLevel, cfg.LogFormat)
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
