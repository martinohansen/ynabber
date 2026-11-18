package enablebanking

import (
	"strings"
	"testing"
)

// dnbTestSynIDRegex is the ENABLEBANKING_DNB_SYN_ID_REGEX value used in
// production: the card patterns, the 8-digit and Propayref patterns, and the
// Giro and Overføring Innland patterns.
const dnbTestSynIDRegex = `(?i)^Visa(\s+\d{6})?,?\s*;(?i)\b(?:Aut\.\s*\d+|Ark\.Ref\s*\*?\s*\d+|Versjon\s+\d+|Kl\.\s*\d{1,2}[.:]\d{2}|Dato\s*\d{1,2}[.:]\d{2}(?:\s+kl\.\s*\d{1,2}[.:]\d{2})?);\s\d{8}\b;(?:^|,\s*)Propayref:\S+;(?i)\bGiro\b(?:\s+\d+)?;(?i)\bOverføring\s+Innland\b(?:\s+\d+)?`

func dnbTestReader(t *testing.T, aspsp string) Reader {
	t.Helper()
	var patterns SynIDRegex
	if err := patterns.Decode(dnbTestSynIDRegex); err != nil {
		t.Fatal(err)
	}
	return Reader{Config: Config{ASPSP: aspsp, DNBSynIDRegex: patterns}}
}

func dnbTestAccount(t *testing.T) AccountInfo {
	t.Helper()
	return AccountInfo{UID: "dnb-test-acc", AccountID: AccountID{IBAN: randomTestIBAN(t)}}
}

func dnbTestTx(remittance ...string) EBTransaction {
	tx := EBTransaction{
		BookingDate:           "2026-11-17",
		CreditDebitIndicator:  "DBIT",
		Status:                statusBooked,
		RemittanceInformation: remittance,
	}
	tx.TransactionAmount.Currency = "NOK"
	tx.TransactionAmount.Amount = "250.00"
	return tx
}

func TestIsDNB(t *testing.T) {
	for aspsp, want := range map[string]bool{
		"DNB":      true,
		"dnb":      true,
		" DNB ":    true,
		"Bulder":   false,
		"DNB Bank": false,
		"":         false,
	} {
		if got := isDNB(aspsp); got != want {
			t.Errorf("isDNB(%q) = %v, want %v", aspsp, got, want)
		}
	}
}

// TestDNBMapperRemittanceVariantsShareID covers the two 8001 patterns: the
// bank sends the reference in the remittance text on one import and drops it
// on a later one, and may also send reference_number. The IDs must match.
func TestDNBMapperRemittanceVariantsShareID(t *testing.T) {
	cases := []struct {
		name    string
		withRef []string
		without []string
	}{
		{
			name:    "giro single element",
			withRef: []string{"Giro 4821, Testfirma As, Avtalegiro, 99000000000000000001"},
			without: []string{"Giro, Testfirma As, Avtalegiro, 99000000000000000001"},
		},
		{
			name:    "giro split elements",
			withRef: []string{"Giro 4821", "Testfirma As", "Avtalegiro", "99000000000000000001"},
			without: []string{"Giro", "Testfirma As", "Avtalegiro", "99000000000000000001"},
		},
		{
			name:    "overforing innland single element",
			withRef: []string{"Overføring Innland 123456789, Testselskap As"},
			without: []string{"Overføring Innland, Testselskap As"},
		},
		{
			name:    "eight digit reference",
			withRef: []string{"Testhandel 12345678, Kjop"},
			without: []string{"Testhandel, Kjop"},
		},
		{
			name:    "propayref appended",
			withRef: []string{"Testselskap As, Kjop"},
			without: []string{"Testselskap As, Kjop, Propayref:ABC123XYZ"},
		},
		{
			name:    "overforing innland split elements",
			withRef: []string{"Overføring Innland 123456789", "Testselskap As"},
			without: []string{"Overføring Innland", "Testselskap As"},
		},
	}

	reader := dnbTestReader(t, "DNB")
	account := dnbTestAccount(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first := dnbTestTx(tc.withRef...)
			second := dnbTestTx(tc.without...)
			second.ReferenceNumber = "REF-0001"

			tx1, err := reader.Mapper(account, first)
			if err != nil {
				t.Fatal(err)
			}
			tx2, err := reader.Mapper(account, second)
			if err != nil {
				t.Fatal(err)
			}
			if tx1 == nil || tx2 == nil {
				t.Fatal("expected mapped transactions")
			}
			if tx1.ID != tx2.ID {
				t.Errorf("IDs differ: %q != %q", tx1.ID, tx2.ID)
			}
			if !strings.HasPrefix(string(tx1.ID), "synth:") {
				t.Errorf("ID = %q, want synthetic ID", tx1.ID)
			}
		})
	}
}

// TestDNBMapperIgnoresBankReferenceIDs checks that DNB never uses
// entry_reference or reference_number as the ID.
func TestDNBMapperIgnoresBankReferenceIDs(t *testing.T) {
	reader := dnbTestReader(t, "DNB")
	account := dnbTestAccount(t)

	plain := dnbTestTx("Giro, Testfirma As")
	withEntryRef := dnbTestTx("Giro, Testfirma As")
	withEntryRef.EntryReference = "BANK-REF-1"
	withRefNumber := dnbTestTx("Giro, Testfirma As")
	withRefNumber.ReferenceNumber = "REF-0002"

	want, err := reader.Mapper(account, plain)
	if err != nil {
		t.Fatal(err)
	}
	for name, tx := range map[string]EBTransaction{"entry_reference": withEntryRef, "reference_number": withRefNumber} {
		got, err := reader.Mapper(account, tx)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != want.ID {
			t.Errorf("%s changed ID: %q != %q", name, got.ID, want.ID)
		}
		if got.ID == "BANK-REF-1" || got.ID == "REF-0002" {
			t.Errorf("%s used as ID: %q", name, got.ID)
		}
	}
}

// TestDNBMapperDoesNotStripOtherNumbers checks that the Giro pattern only
// removes the reference next to "Giro". A changed Avtalegiro number or amount
// must still produce a different ID.
func TestDNBMapperDoesNotStripOtherNumbers(t *testing.T) {
	reader := dnbTestReader(t, "DNB")
	account := dnbTestAccount(t)

	base := dnbTestTx("Giro 4821, Testfirma As, Avtalegiro, 99000000000000000001")
	otherAgreement := dnbTestTx("Giro 4821, Testfirma As, Avtalegiro, 99000000000000000002")
	otherAmount := dnbTestTx("Giro 4821, Testfirma As, Avtalegiro, 99000000000000000001")
	otherAmount.TransactionAmount.Amount = "251.00"

	want, err := reader.Mapper(account, base)
	if err != nil {
		t.Fatal(err)
	}
	for name, tx := range map[string]EBTransaction{"agreement number": otherAgreement, "amount": otherAmount} {
		got, err := reader.Mapper(account, tx)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID == want.ID {
			t.Errorf("changing %s did not change ID", name)
		}
	}
}

// TestDNBMapperPreservesPayeeAndMemo checks that the ID regex only affects the
// hash input.
func TestDNBMapperPreservesPayeeAndMemo(t *testing.T) {
	account := dnbTestAccount(t)
	tx := dnbTestTx("Giro 4821, Testfirma As, Avtalegiro, 99000000000000000001")

	withRegex, err := dnbTestReader(t, "DNB").Mapper(account, tx)
	if err != nil {
		t.Fatal(err)
	}
	withoutRegex, err := (Reader{Config: Config{ASPSP: "DNB"}}).Mapper(account, tx)
	if err != nil {
		t.Fatal(err)
	}
	if withRegex.Payee != withoutRegex.Payee {
		t.Errorf("Payee = %q, want %q", withRegex.Payee, withoutRegex.Payee)
	}
	if withRegex.Memo != withoutRegex.Memo {
		t.Errorf("Memo = %q, want %q", withRegex.Memo, withoutRegex.Memo)
	}
	if withRegex.ID == withoutRegex.ID {
		t.Error("DNB regex did not change the synthetic ID input")
	}
}

// TestNonDNBMapperIgnoresDNBSynIDRegex checks that the DNB regex and the
// DNB-only ID logic do not apply to other ASPSPs.
func TestNonDNBMapperIgnoresDNBSynIDRegex(t *testing.T) {
	reader := dnbTestReader(t, "Bulder")
	account := dnbTestAccount(t)

	first := dnbTestTx("Giro 4821, Testfirma As")
	second := dnbTestTx("Giro, Testfirma As")

	tx1, err := reader.Mapper(account, first)
	if err != nil {
		t.Fatal(err)
	}
	tx2, err := reader.Mapper(account, second)
	if err != nil {
		t.Fatal(err)
	}
	if tx1.ID == tx2.ID {
		t.Errorf("DNB regex applied to non-DNB ASPSP: both IDs %q", tx1.ID)
	}

	withRef := dnbTestTx("Giro 4821, Testfirma As")
	withRef.EntryReference = "BANK-REF-2"
	tx3, err := reader.Mapper(account, withRef)
	if err != nil {
		t.Fatal(err)
	}
	if tx3.ID != "BANK-REF-2" {
		t.Errorf("non-DNB ID = %q, want entry_reference", tx3.ID)
	}
}

func TestDNBMapperSkipsNonBookedTransactions(t *testing.T) {
	reader := dnbTestReader(t, "DNB")
	tx := dnbTestTx("Giro 4821, Testfirma As")
	tx.Status = "PDNG"

	got, err := reader.Mapper(dnbTestAccount(t), tx)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("pending transaction mapped to %+v, want nil", got)
	}
}
