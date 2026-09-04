package wealthreader

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/martinohansen/ynabber"
)

func TestMapperUsesValueDateWhenOperationDateMissing(t *testing.T) {
	reader := Reader{}
	account := Account{UUID: "acc", Name: "Test", Code: "ES00"}
	tx := Transaction{
		UUID:        "tx-1",
		ValueDate:   "2024-03-15",
		Amount:      json.Number("10.00"),
		Description: "Fallback date",
	}

	got, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	want := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	if !got.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", got.Date, want)
	}
}

func TestMapperRejectsMissingID(t *testing.T) {
	reader := Reader{}
	_, err := reader.Mapper(Account{UUID: "acc"}, Transaction{
		Amount:        json.Number("1"),
		OperationDate: "2024-01-01",
	})
	if err == nil {
		t.Fatal("expected error for missing uuid")
	}
}

func TestMapperPayeeStrip(t *testing.T) {
	reader := Reader{Config: Config{PayeeStrip: []string{"Dk-Nota "}}}
	got, err := reader.Mapper(Account{UUID: "acc", Name: "A", Code: "IBAN"}, Transaction{
		UUID:          "tx-2",
		OperationDate: "2024-01-01",
		Amount:        json.Number("-3.50"),
		Description:   "Dk-Nota Remouladen",
	})
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if got.Payee != "Remouladen" {
		t.Errorf("Payee = %q, want Remouladen", got.Payee)
	}
	if got.Amount != ynabber.Milliunits(-3500) {
		t.Errorf("Amount = %d, want -3500", got.Amount)
	}
}

func TestMapperPayeeFromTransferDetails(t *testing.T) {
	reader := Reader{}
	got, err := reader.Mapper(Account{UUID: "acc", Name: "A", Code: "IBAN"}, Transaction{
		UUID:          "tx-3",
		OperationDate: "2024-01-01",
		Amount:        json.Number("1.00"),
		TransferDetails: TransferDetails{
			SenderReceiver: "Acme Corp",
			Concept:        "Invoice 12",
		},
	})
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if got.Payee != "Acme Corp" {
		t.Errorf("Payee = %q, want Acme Corp", got.Payee)
	}
	if got.Memo != "Invoice 12" {
		t.Errorf("Memo = %q, want Invoice 12", got.Memo)
	}
}

func TestMapperTruncatesPayee(t *testing.T) {
	reader := Reader{}
	long := strings.Repeat("x", 250)
	got, err := reader.Mapper(Account{UUID: "acc", Name: "A", Code: "IBAN"}, Transaction{
		UUID:          "tx-4",
		OperationDate: "2024-01-01",
		Amount:        json.Number("1"),
		Description:   long,
	})
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if n := len([]rune(got.Payee)); n != 200 {
		t.Errorf("payee runes = %d, want 200", n)
	}
}
