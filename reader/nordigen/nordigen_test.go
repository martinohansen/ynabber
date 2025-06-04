package nordigen

import (
	"log/slog"
	"testing"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

// getAccountTransactions returns a nordigen.AccountTransactions with a single
// transaction for testing purposes.
func getAccountTransactions(t nordigen.Transaction) nordigen.AccountTransactions {
	return nordigen.AccountTransactions{
		Transactions: struct {
			Booked  []nordigen.Transaction `json:"booked,omitempty"`
			Pending []nordigen.Transaction `json:"pending,omitempty"`
		}{
			Booked: []nordigen.Transaction{t},
		},
	}
}

func TestToYnabber(t *testing.T) {
	logger := slog.Default()
	var defaultConfig Config
	_ = envconfig.Process("", &defaultConfig)
	alternateConfig1 := defaultConfig
	alternateConfig1.PayeeSource = PayeeGroups{{Name}}

	type args struct {
		account ynabber.Account
		t       nordigen.AccountTransactions
	}
	tests := []struct {
		name    string
		bankID  string
		reader  Reader
		args    args
		want    []ynabber.Transaction
		wantErr bool
	}{
		{
			// Test SPAREBANK_SR_BANK_SPRONO22
			bankID: "SPAREBANK_SR_BANK_SPRONO22",
			reader: Reader{Config: defaultConfig, logger: logger},
			args: args{
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: getAccountTransactions(nordigen.Transaction{
					TransactionId: "enc!!ustedqqdRxVCJQO-LJY_6Z7YX10m9eOm9EsPIggKCwmfZ5zhsW_aTPp_JRNdvIBxpVXjoREGjie99BjskkgI6sOtyxvfxJy66entsPGFZf1wE4B6EgITjdX3K33fL7kG325l7CJRV_pj1EnnuvnD4a7zEsrXG1IFLW-EHrfdBs7ndQgVdIVzH5pfsZewhyvOBWss4iusmmm7V9MYeumYKEkzwcwl77YsGsRgN9_hHm0=",
					BookingDate:   "2025-05-15",
					TransactionAmount: struct {
						Amount   string "json:\"amount,omitempty\""
						Currency string "json:\"currency,omitempty\""
					}{Amount: "-469.64", Currency: "NOK"},
					CreditorName: "Tibber Norge AS",
					CreditorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: ""},
					DebtorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: ""},
					RemittanceInformationUnstructuredArray: []string{"5345", "SCOR"},
					ProprietaryBankTransactionCode:         "foobar",
					InternalTransactionId:                  "c74f52528b29483f519d9794b1df8c3b",
				}),
			},
			want: []ynabber.Transaction{{
				Account: ynabber.Account{Name: "foo", IBAN: "bar"},
				ID:      ynabber.ID("foobar"),
				Date:    time.Date(2025, time.May, 15, 0, 0, 0, 0, time.UTC),
				Payee:   "SCOR",
				Memo:    "5345 SCOR",
				Amount:  ynabber.Milliunits(-469640)},
			},
			wantErr: false,
		},
		{
			// Tests a common Nordigen transaction from NORDEA_NDEADKKK with the
			// default config to highlight any breaking changes.
			bankID: "NORDEA_NDEADKKK",
			reader: Reader{Config: defaultConfig, logger: logger},
			args: args{
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: getAccountTransactions(nordigen.Transaction{
					TransactionId:  "H00000000000000000000",
					EntryReference: "",
					BookingDate:    "2023-02-24",
					ValueDate:      "2023-02-24",
					TransactionAmount: struct {
						Amount   string "json:\"amount,omitempty\""
						Currency string "json:\"currency,omitempty\""
					}{Amount: "10", Currency: "DKK"},
					CreditorName: "",
					CreditorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: "0"},
					UltimateCreditor: "",
					DebtorName:       "",
					DebtorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: ""},
					UltimateDebtor:                         "",
					RemittanceInformationUnstructured:      "Visa køb DKK 424,00 HELLOFRESH Copenha Den 23.02",
					RemittanceInformationUnstructuredArray: []string{""},
					BankTransactionCode:                    "",
					AdditionalInformation:                  "VISA KØB"},
				),
			},
			want: []ynabber.Transaction{{
				Account: ynabber.Account{Name: "foo", IBAN: "bar"},
				ID:      ynabber.ID("H00000000000000000000"),
				Date:    time.Date(2023, time.February, 24, 0, 0, 0, 0, time.UTC),
				Payee:   "Visa køb DKK HELLOFRESH Copenha Den",
				Memo:    "Visa køb DKK 424,00 HELLOFRESH Copenha Den 23.02",
				Amount:  ynabber.Milliunits(10000)},
			},
			wantErr: false,
		},
		{
			// Nordea should remove P transactions
			name:   "Remove P transactions",
			bankID: "NORDEA_NDEADKKK",
			reader: Reader{Config: defaultConfig, logger: logger},
			args: args{
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: getAccountTransactions(nordigen.Transaction{
					TransactionId: "P4392858879202309260000000524",
					BookingDate:   "2023-09-26",
					ValueDate:     "2023-09-26",
					TransactionAmount: struct {
						Amount   string `json:"amount,omitempty"`
						Currency string `json:"currency,omitempty"`
					}{
						Amount:   "-4200.00",
						Currency: "DKK",
					},
					RemittanceInformationUnstructured: "Ovf. til, konto nr. 1111-222-333",
					AdditionalInformation:             "OVERFØRT TIL",
				}),
			},
			want:    []ynabber.Transaction{},
			wantErr: false,
		},
		{
			// Test transaction from SEB_KORT_AB_NO_SKHSFI21
			name:   "SEB_KORT_AB_NO_SKHSFI21",
			bankID: "SEB_KORT_AB_NO_SKHSFI21",
			reader: Reader{Config: defaultConfig, logger: logger},
			args: args{
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: getAccountTransactions(nordigen.Transaction{
					TransactionId:  "foobar",
					EntryReference: "",
					BookingDate:    "2023-02-24",
					ValueDate:      "2023-02-24",
					TransactionAmount: struct {
						Amount   string "json:\"amount,omitempty\""
						Currency string "json:\"currency,omitempty\""
					}{Amount: "10", Currency: "NOK"},
					CreditorName: "",
					CreditorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: "0"},
					UltimateCreditor: "",
					DebtorName:       "",
					DebtorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: ""},
					UltimateDebtor:                         "",
					RemittanceInformationUnstructured:      "",
					RemittanceInformationUnstructuredArray: []string{""},
					BankTransactionCode:                    "PURCHASE",
					AdditionalInformation:                  "PASCAL AS"},
				),
			},
			want: []ynabber.Transaction{{
				Account: ynabber.Account{Name: "foo", IBAN: "bar"},
				ID:      ynabber.ID("foobar"),
				Date:    time.Date(2023, time.February, 24, 0, 0, 0, 0, time.UTC),
				Payee:   "PASCAL AS",
				Memo:    "PASCAL AS",
				Amount:  ynabber.Milliunits(10000)},
			},
			wantErr: false,
		},
		{
			// Test transaction from S_PANKKI_SBANFIHH - a typical debit card transaction
			name:   "S_PANKKI_SBANFIHH Debit card",
			bankID: "S_PANKKI_SBANFIHH",
			reader: Reader{Config: alternateConfig1, logger: logger},
			args: args{
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: getAccountTransactions(nordigen.Transaction{
					TransactionId: "foobar",
					BookingDate:   "2025-05-28",
					ValueDate:     "2025-05-28",
					TransactionAmount: struct {
						Amount   string "json:\"amount,omitempty\""
						Currency string "json:\"currency,omitempty\""
					}{Amount: "-80", Currency: "EUR"},
					CreditorName: "Retail shop",
					CreditorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: ""},
					UltimateCreditor: "",
					DebtorName:       "",
					DebtorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: ""},
					UltimateDebtor:                         "",
					RemittanceInformationUnstructured:      "",
					RemittanceInformationUnstructuredArray: []string{""},
					BankTransactionCode:                    "CCRD-POSD",
					AdditionalInformation:                  ""},
				),
			},
			want: []ynabber.Transaction{{
				Account: ynabber.Account{Name: "foo", IBAN: "bar"},
				ID:      ynabber.ID("foobar"),
				Date:    time.Date(2025, time.May, 28, 0, 0, 0, 0, time.UTC),
				Payee:   "Retail shop",
				Memo:    "Retail shop",
				Amount:  -ynabber.Milliunits(80000)},
			},
			wantErr: false,
		},
		{
			// Test transaction from S_PANKKI_SBANFIHH - an incoming transaction with message
			name:   "S_PANKKI_SBANFIHH Received with message",
			bankID: "S_PANKKI_SBANFIHH",
			reader: Reader{Config: alternateConfig1, logger: logger},
			args: args{
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: getAccountTransactions(nordigen.Transaction{
					TransactionId: "foobar",
					BookingDate:   "2025-05-28",
					ValueDate:     "2025-05-28",
					TransactionAmount: struct {
						Amount   string "json:\"amount,omitempty\""
						Currency string "json:\"currency,omitempty\""
					}{Amount: "80", Currency: "EUR"},
					CreditorName: "",
					CreditorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: ""},
					UltimateCreditor: "",
					DebtorName:       "JOHN DOE",
					DebtorAccount: struct {
						Iban string "json:\"iban,omitempty\""
					}{Iban: ""},
					UltimateDebtor:                         "",
					RemittanceInformationUnstructured:      "Hello there",
					RemittanceInformationUnstructuredArray: []string{""},
					BankTransactionCode:                    "RCDT-ESCT",
					AdditionalInformation:                  ""},
				),
			},
			want: []ynabber.Transaction{{
				Account: ynabber.Account{Name: "foo", IBAN: "bar"},
				ID:      ynabber.ID("foobar"),
				Date:    time.Date(2025, time.May, 28, 0, 0, 0, 0, time.UTC),
				Payee:   "JOHN DOE",
				Memo:    "Hello there",
				Amount:  ynabber.Milliunits(80000)},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		name := tt.bankID
		if tt.name != "" {
			name = name + "_" + tt.name
		}
		t.Run(name, func(t *testing.T) {

			// Set the BankID to the test case but keep the rest of the config
			// as is
			tt.reader.Config.BankID = tt.bankID

			got, err := tt.reader.toYnabbers(tt.args.account, tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %+v, wantErr %+v", err, tt.wantErr)
				return
			}
			if cmp.Equal(got, tt.want) == false {
				t.Errorf("got = \n%+v, want \n%+v", got, tt.want)
			}
		})
	}
}
