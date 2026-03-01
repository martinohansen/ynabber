package ynabber

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/martinohansen/ynabber/internal/web"
)

type Ynabber struct {
	Readers []Reader
	Writers []Writer

	config    *Config
	logger    slog.Logger
	webServer *web.Server
}

// WebServer returns the built-in status server, or nil if disabled.
// Readers and writers can call Register on it to appear on the dashboard.
func (y *Ynabber) WebServer() *web.Server { return y.webServer }
func NewYnabber(config *Config) *Ynabber {
	y := &Ynabber{
		config: config,
		logger: *slog.Default(),
	}
	if config.Port > 0 {
		y.webServer = web.NewServer(config.Port, config.Host, slog.Default())
		y.webServer.Register(&sysInfo{cfg: config})
	}
	return y
}

// sysInfo exposes global ynabber configuration on the dashboard.
type sysInfo struct{ cfg *Config }

func (s *sysInfo) Name() string { return "ynabber" }
func (s *sysInfo) Status() web.ComponentStatus {
	return web.ComponentStatus{
		Healthy: true,
		Fields: []web.StatusField{
			{Label: "readers", Value: fmt.Sprintf("%v", s.cfg.Readers)},
			{Label: "writers", Value: fmt.Sprintf("%v", s.cfg.Writers)},
			{Label: "data dir", Value: s.cfg.DataDir},
			{Label: "listen", Value: fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)},
		},
	}
}

type Reader interface {
	Runner(ctx context.Context, out chan<- []Transaction) error
	String() string
}

type Writer interface {
	Runner(ctx context.Context, in <-chan []Transaction) error
	String() string
}

// Run starts Ynabber by reading transactions from all readers into a channel to
// fan out to all writers. Returns immediately on first error from any reader or
// writer.
func (y *Ynabber) Run() error {
	g, ctx := errgroup.WithContext(context.Background())

	if y.webServer != nil {
		if err := y.webServer.Start(ctx); err != nil {
			return err
		}
		y.logger.Info("web server started", "port", y.webServer.Port())
	}

	// Move transactions from reader to writer in batches on this channel.
	// Multiple readers and writer can be used
	batches := make(chan []Transaction)

	// Create a channel for each writer and fan out transactions to each one
	channels := make([]chan []Transaction, len(y.Writers))
	for c := range channels {
		channels[c] = make(chan []Transaction)
	}

	// Track when all readers are done
	var readerWg sync.WaitGroup
	readerWg.Add(len(y.Readers))

	// Close batches channel when all readers are done
	go func() {
		readerWg.Wait()
		close(batches)
	}()

	// Fan out transactions to all writer channels
	g.Go(func() error {
		defer func() {
			for _, c := range channels {
				close(c)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case batch, ok := <-batches:
				if !ok {
					return nil
				}
				for _, c := range channels {
					select {
					case c <- batch:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}
	})

	// Start all writers
	for c, writer := range y.Writers {
		g.Go(func() error {
			return writer.Runner(ctx, channels[c])
		})
	}

	// Start all readers
	for _, reader := range y.Readers {
		g.Go(func() error {
			defer readerWg.Done()
			return reader.Runner(ctx, batches)
		})
	}

	// Wait for all goroutines to complete or first error
	if err := g.Wait(); err != nil && err != context.Canceled {
		return err
	}

	y.logger.Info("all readers and writers completed successfully")
	return nil
}
