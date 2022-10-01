package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"

	_ "github.com/honeycombio/honeycomb-opentelemetry-go"
	"github.com/honeycombio/opentelemetry-go-contrib/launcher"

	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/writer/ynab"
)

const name = "ynabber"

func main() {
	ctx := context.Background()

	otelShutdown, err := launcher.ConfigureOpenTelemetry()
	if err != nil {
		log.Fatalf("Failed to setting up OTel SDK: %e", err)
	}
	defer otelShutdown()

	sleeps := ynabber.ConfigLookup("YNABBER_INTERVAL", "5m")
	sleep, err := time.ParseDuration(sleeps)
	if err != nil {
		log.Fatalf("Failed to parse YNABBER_INTERVAL: %s", err)
	}

	for {
		tracer := otel.Tracer(name)
		mainCtx, rootSpan := tracer.Start(ctx, "main")
		err = run(mainCtx)
		if err != nil {
			panic(err)
		} else {
			log.Printf("Run succeeded")
		}
		rootSpan.End()
		log.Printf("Waiting %s before running again...", sleeps)
		time.Sleep(sleep)
	}
}

func run(ctx context.Context) error {
	tracer := otel.Tracer(name)
	rootCtx, rootSpan := tracer.Start(ctx, "Run")

	var transactions []ynabber.Transaction

	var readerList []string
	r := ynabber.ConfigLookup("YNABBER_READERS", "[\"nordigen\"]")
	err := json.Unmarshal([]byte(r), &readerList)
	if err != nil {
		return fmt.Errorf("couldn't parse readers: %w", err)
	}

	var writerList []string
	w := ynabber.ConfigLookup("YNABBER_WRITERS", "[\"ynab\"]")
	err = json.Unmarshal([]byte(w), &writerList)
	if err != nil {
		return fmt.Errorf("couldn't parse writers: %w", err)
	}

	for _, reader := range readerList {
		readerCtx, readerSpan := tracer.Start(rootCtx, "Reader")
		log.Printf("Reading from %s", reader)
		switch reader {
		case "nordigen":
			t, err := nordigen.BulkReader(readerCtx)
			if err != nil {
				return fmt.Errorf("couldn't read from nordigen: %w", err)
			}
			transactions = append(transactions, t...)
		}
		readerSpan.End()
	}

	for _, writer := range writerList {
		writerCtx, writerSpan := tracer.Start(rootCtx, "Writer")
		log.Printf("Writing to %s", writer)
		switch writer {
		case "ynab":
			err := ynab.BulkWriter(writerCtx, transactions)
			if err != nil {
				return fmt.Errorf("couldn't write to ynab: %w", err)
			}
		}
		writerSpan.End()
	}
	rootSpan.End()
	return nil
}
