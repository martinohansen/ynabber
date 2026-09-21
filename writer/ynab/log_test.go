package ynab

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/internal/log"
)

func TestBulkLogsFinancialPayloadOnlyAtTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	source := ynabber.Transaction{
		Account: ynabber.Account{IBAN: "private-iban"},
		ID:      "private-transaction",
		Date:    time.Now().Add(-time.Hour),
		Amount:  12340,
		Payee:   "private-payee",
		Memo:    "private-memo",
	}

	for _, test := range []struct {
		name    string
		level   slog.Level
		wantRaw bool
	}{
		{name: "debug", level: slog.LevelDebug},
		{name: "trace", level: log.LevelTrace, wantRaw: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: test.level}))
			writer := newLogTestWriter(Config{
				BudgetID: "budget-id",
				Token:    "private-token",
				AccountMap: AccountMap{
					"private-iban": "account-id",
				},
			}, logger, server.Client(), server.URL)

			if err := writer.Bulk(context.Background(), []ynabber.Transaction{source}); err != nil {
				t.Fatal(err)
			}

			got := output.String()
			if strings.Contains(got, "private-token") {
				t.Fatalf("log contains credential: %s", got)
			}
			for _, operational := range []string{"budget-id", "account-id"} {
				if !strings.Contains(got, operational) {
					t.Fatalf("log omits operational identifier %q: %s", operational, got)
				}
			}
			for _, private := range []string{"private-iban", "private-transaction", "private-payee", "private-memo", "12340"} {
				if strings.Contains(got, private) != test.wantRaw {
					t.Fatalf("raw value %q presence = %t, want %t: %s", private, strings.Contains(got, private), test.wantRaw, got)
				}
			}
		})
	}
}

func TestMappingErrorIsVisibleWithMaskedBankIdentifier(t *testing.T) {
	var output bytes.Buffer
	writer := newLogTestWriter(Config{
		BudgetID: "budget-id",
		Token:    "token",
		AccountMap: AccountMap{
			"configured": "account-id",
		},
	}, slog.New(slog.NewTextHandler(&output, nil)), nil, "")

	err := writer.Bulk(context.Background(), []ynabber.Transaction{{
		Account: ynabber.Account{ID: "provider-account-id", IBAN: "NO9812345678901"},
		ID:      "transaction-id",
		Date:    time.Now().Add(-time.Hour),
	}})
	if err != nil {
		t.Fatal(err)
	}

	got := output.String()
	for _, want := range []string{"provider-account-id", "NO98...8901", "no matching YNAB account"} {
		if !strings.Contains(got, want) {
			t.Fatalf("mapping error log omits %q: %s", want, got)
		}
	}
	if strings.Contains(got, "NO9812345678901") {
		t.Fatalf("mapping error log contains full IBAN: %s", got)
	}
}

func TestTruncationWarningsIncludeImportID(t *testing.T) {
	var output bytes.Buffer
	writer := newLogTestWriter(Config{
		BudgetID: "budget-id",
		Token:    "token",
		AccountMap: AccountMap{
			"bank-id": "account-id",
		},
	}, slog.New(slog.NewTextHandler(&output, nil)), nil, "")
	source := ynabber.Transaction{
		Account: ynabber.Account{ID: "bank-id"},
		ID:      "transaction-id",
		Date:    time.Now().Add(-time.Hour),
		Payee:   strings.Repeat("p", maxPayeeSize+1),
		Memo:    strings.Repeat("m", maxMemoSize+1),
	}

	if _, err := writer.toYNAB(source); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, want := range []string{"memo too long", "payee too long", "import_id=" + makeID(source)} {
		if !strings.Contains(got, want) {
			t.Fatalf("truncation log omits %q: %s", want, got)
		}
	}
}

func newLogTestWriter(config Config, logger *slog.Logger, client httpClient, baseURL string) Writer {
	return Writer{
		Config: config,
		logger: logger.With(
			"writer", "ynab",
			"budget_id", config.BudgetID,
		),
		client:  client,
		baseURL: baseURL,
		now:     time.Now,
	}
}
