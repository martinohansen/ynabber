package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

func TestProcessorConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, names, prefix, want string
		unsetNames, unsetPrefix   bool
		count                     int
	}{
		{name: "default disabled", unsetNames: true, unsetPrefix: true},
		{name: "explicitly disabled"},
		{name: "default prefix", names: "prefix", unsetPrefix: true, want: "[test] Shop", count: 1},
		{name: "configured prefix", names: "prefix", prefix: "demo: ", want: "demo: Shop", count: 1},
		{name: "empty prefix", names: "prefix", prefix: "", want: "Shop", count: 1},
		{name: "ordered list", names: "prefix, prefix", prefix: "X", want: "XXShop", count: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("YNABBER_PROCESSORS", test.names)
			t.Setenv("YNABBER_PREFIX_PAYEE_PREFIX", test.prefix)
			if test.unsetNames {
				if err := os.Unsetenv("YNABBER_PROCESSORS"); err != nil {
					t.Fatal(err)
				}
			}
			if test.unsetPrefix {
				if err := os.Unsetenv("YNABBER_PREFIX_PAYEE_PREFIX"); err != nil {
					t.Fatal(err)
				}
			}
			var cfg ynabber.Config
			if err := envconfig.Process("", &cfg); err != nil {
				t.Fatal(err)
			}
			processors, err := setupProcessors(cfg.Processors)
			if err != nil {
				t.Fatal(err)
			}
			if len(processors) != test.count {
				t.Fatalf("processor count = %d", len(processors))
			}
			batch := []ynabber.Transaction{{Payee: "Shop"}}
			for _, processor := range processors {
				batch, err = processor.Process(context.Background(), batch)
				if err != nil {
					t.Fatal(err)
				}
			}
			if test.count > 0 && batch[0].Payee != test.want {
				t.Fatalf("payee = %q, want %q", batch[0].Payee, test.want)
			}
		})
	}
}

func TestUnknownProcessor(t *testing.T) {
	if _, err := setupProcessors([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v", err)
	}
}
