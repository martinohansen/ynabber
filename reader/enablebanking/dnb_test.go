package enablebanking

import (
	"strings"
	"testing"
)

func TestDNBMapperUsesBookedTransactionIDAcrossRemittanceChanges(t *testing.T) {
	reader := Reader{Config: Config{ASPSP: "dnb"}}
	account := AccountInfo{
		UID:       "dnb-account",
		AccountID: AccountID{IBAN: randomTestIBAN(t)},
	}
	transactionID := "MDExNl8wMDI5NzkzOTE5MDk0XzAwMDAwMDE"

	first := dnbMapperTestTransaction(transactionID, "Varekjøp, Rusta Asker, Dato 10.10 kl. 13.59")
	second := dnbMapperTestTransaction(transactionID, "Varekjøp, Kl. 13.59 Versjon 3 Aut. 405762, Rusta Asker")

	firstMapped, err := reader.Mapper(account, first)
	if err != nil {
		t.Fatalf("Mapper(first) error = %v", err)
	}
	secondMapped, err := reader.Mapper(account, second)
	if err != nil {
		t.Fatalf("Mapper(second) error = %v", err)
	}
	if firstMapped == nil || secondMapped == nil {
		t.Fatal("Mapper() returned nil transaction for booked DNB transaction")
	}
	if got := string(firstMapped.ID); got != transactionID {
		t.Errorf("first ID = %q, want DNB transaction_id %q", got, transactionID)
	}
	if got := string(secondMapped.ID); got != transactionID {
		t.Errorf("second ID = %q, want DNB transaction_id %q", got, transactionID)
	}
}

func TestDNBMapperPrefersReferenceNumberOverTransactionID(t *testing.T) {
	reader := Reader{Config: Config{ASPSP: "DNB"}}
	account := AccountInfo{UID: "dnb-account", AccountID: AccountID{IBAN: randomTestIBAN(t)}}
	tx := dnbMapperTestTransaction("MDExNl8wMDI5NzkzOTE5MDk0XzAwMDAwMDE", "Visa, merchant")
	tx.ReferenceNumber = "  KID-REF-123  "

	mapped, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if mapped == nil {
		t.Fatal("Mapper() returned nil transaction")
	}
	if got := string(mapped.ID); got != "KID-REF-123" {
		t.Errorf("ID = %q, want reference_number", got)
	}
}

func TestDNBMapperPrefersEntryReferenceOverOtherIDs(t *testing.T) {
	reader := Reader{Config: Config{ASPSP: "DNB"}}
	account := AccountInfo{UID: "dnb-account", AccountID: AccountID{IBAN: randomTestIBAN(t)}}
	tx := dnbMapperTestTransaction("MDExNl8wMDI5NzkzOTE5MDk0XzAwMDAwMDE", "Visa, merchant")
	tx.EntryReference = "  ENTRY-REF-456  "
	tx.ReferenceNumber = "  KID-REF-123  "

	mapped, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if mapped == nil {
		t.Fatal("Mapper() returned nil transaction")
	}
	if got := string(mapped.ID); got != "ENTRY-REF-456" {
		t.Errorf("ID = %q, want entry_reference", got)
	}
}

func TestDNBMapperLeavesSyntheticFallbackRemittanceUntouched(t *testing.T) {
	reader := Reader{Config: Config{ASPSP: "DNB"}}
	account := AccountInfo{UID: "dnb-account", AccountID: AccountID{IBAN: randomTestIBAN(t)}}
	first := dnbMapperTestTransaction("invalid-id", "Visa  100021, Eat-Now.No")
	second := dnbMapperTestTransaction("invalid-id", "Visa, Eat-Now.No")

	firstMapped, err := reader.Mapper(account, first)
	if err != nil {
		t.Fatalf("Mapper(first) error = %v", err)
	}
	secondMapped, err := reader.Mapper(account, second)
	if err != nil {
		t.Fatalf("Mapper(second) error = %v", err)
	}
	if firstMapped == nil || secondMapped == nil {
		t.Fatal("Mapper() returned nil transaction")
	}
	if firstMapped.ID == secondMapped.ID {
		t.Errorf("synthetic fallback IDs should reflect raw remittance changes: %q", firstMapped.ID)
	}
}

func TestDNBMapperUsesConfiguredPayeeStrip(t *testing.T) {
	reader := Reader{Config: Config{ASPSP: "DNB", PayeeStrip: []string{"100021"}}}
	account := AccountInfo{UID: "dnb-account", AccountID: AccountID{IBAN: randomTestIBAN(t)}}
	tx := dnbMapperTestTransaction("MDExNl8wMDI5NzkzOTE5MDk0XzAwMDAwMDE", "Visa  100021, Eat-Now.No")

	mapped, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if mapped == nil {
		t.Fatal("Mapper() returned nil transaction")
	}
	if strings.Contains(mapped.Payee, "100021") {
		t.Errorf("Payee = %q, want configured PayeeStrip applied", mapped.Payee)
	}
}

func TestDNBMapperDoesNotUseTransactionIDForOtherASPSPs(t *testing.T) {
	reader := Reader{Config: Config{ASPSP: "Sparebanken Vest"}}
	account := AccountInfo{UID: "other-account", AccountID: AccountID{IBAN: randomTestIBAN(t)}}
	tx := dnbMapperTestTransaction("MDExNl8wMDI5NzkzOTE5MDk0XzAwMDAwMDE", "Visa, merchant")

	mapped, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if mapped == nil {
		t.Fatal("Mapper() returned nil transaction")
	}
	if got := string(mapped.ID); !strings.HasPrefix(got, "synth:") {
		t.Errorf("non-DNB ID = %q, want existing synthetic behavior", got)
	}
}

func TestDNBMapperUsesSyntheticFallbackWhenTransactionIDIsMissing(t *testing.T) {
	reader := Reader{Config: Config{ASPSP: "DNB"}}
	account := AccountInfo{UID: "dnb-account", AccountID: AccountID{IBAN: randomTestIBAN(t)}}
	tx := dnbMapperTestTransaction("", "Visa  100021, Eat-Now.No")

	mapped, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if mapped == nil {
		t.Fatal("Mapper() returned nil transaction")
	}
	if got := string(mapped.ID); !strings.HasPrefix(got, "synth:") {
		t.Errorf("fallback ID = %q, want synth: prefix", got)
	}
}

func TestDNBMapperStillSkipsPendingTransactions(t *testing.T) {
	reader := Reader{Config: Config{ASPSP: "DNB"}}
	account := AccountInfo{UID: "dnb-account", AccountID: AccountID{IBAN: randomTestIBAN(t)}}
	tx := dnbMapperTestTransaction("MDExNl8wMDI5NzkzOTE5MDk0XzAwMDAwMDE", "Visa, merchant")
	tx.Status = "PDNG"

	mapped, err := reader.Mapper(account, tx)
	if err != nil {
		t.Fatalf("Mapper() error = %v", err)
	}
	if mapped != nil {
		t.Fatalf("Mapper() = %#v, want nil for pending transaction", mapped)
	}
}

func dnbMapperTestTransaction(transactionID, remittance string) EBTransaction {
	return EBTransaction{
		TransactionID: transactionID,
		TransactionAmount: struct {
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		}{Currency: "NOK", Amount: "263.20"},
		CreditDebitIndicator:  "DBIT",
		Status:                statusBooked,
		BookingDate:           "2026-10-12",
		RemittanceInformation: []string{remittance},
	}
}
