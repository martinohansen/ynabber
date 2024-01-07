package nordigen

import (
	"testing"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

func TestTransactionToYnabber(t *testing.T) {

	var defaultConfig ynabber.Config
	_ = envconfig.Process("", &defaultConfig)

	reader := Reader{
		Config: &defaultConfig,
	}

	type args struct {
		account ynabber.Account
		t       nordigen.Transaction
	}
	tests := []struct {
		name    string
		args    args
		want    ynabber.Transaction
		wantErr bool
	}{
		{name: "milliunits a",
			args: args{account: ynabber.Account{}, t: nordigen.Transaction{
				BookingDate: "0001-01-01",
				TransactionAmount: struct {
					Amount   string "json:\"amount,omitempty\""
					Currency string "json:\"currency,omitempty\""
				}{
					Amount: "328.18",
				},
			}},
			want: ynabber.Transaction{
				Amount: ynabber.Milliunits(328180),
			},
			wantErr: false,
		},
		{name: "milliunits b",
			args: args{account: ynabber.Account{}, t: nordigen.Transaction{
				BookingDate: "0001-01-01",
				TransactionAmount: struct {
					Amount   string "json:\"amount,omitempty\""
					Currency string "json:\"currency,omitempty\""
				}{
					Amount: "32818",
				},
			}},
			want: ynabber.Transaction{
				Amount: ynabber.Milliunits(32818000),
			},
			wantErr: false,
		},
		{
			// Tests a common Nordigen transaction from NORDEA_NDEADKKK with the
			// default config to highlight any breaking changes.
			name: "NORDEA_NDEADKKK",
			args: args{
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: nordigen.Transaction{
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
			},
			want: ynabber.Transaction{
				Account: ynabber.Account{Name: "foo", IBAN: "bar"},
				ID:      ynabber.ID("H00000000000000000000"),
				Date:    time.Date(2023, time.February, 24, 0, 0, 0, 0, time.UTC),
				Payee:   "Visa køb DKK HELLOFRESH Copenha Den",
				Memo:    "Visa køb DKK 424,00 HELLOFRESH Copenha Den 23.02",
				Amount:  ynabber.Milliunits(10000),
			},
			wantErr: false,
		},
		{
			// Test transaction from SEB_KORT_AB_NO_SKHSFI21
			name: "SEB_KORT_AB_NO_SKHSFI21",
			args: args{
				account: ynabber.Account{Name: "foo", IBAN: "bar"},
				t: nordigen.Transaction{
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
			},
			want: ynabber.Transaction{
				Account: ynabber.Account{Name: "foo", IBAN: "bar"},
				ID:      ynabber.ID("foobar"),
				Date:    time.Date(2023, time.February, 24, 0, 0, 0, 0, time.UTC),
				Payee:   "PASCAL AS",
				Memo:    "",
				Amount:  ynabber.Milliunits(10000),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reader.toYnabber(tt.args.account, tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %+v, wantErr %+v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got = \n%+v, want \n%+v", got, tt.want)
			}
		})
	}
}

func TestPayeeStripNonAlphanumeric(t *testing.T) {
	want := "Im just alphanumeric"
	got := payeeStripNonAlphanumeric("Im just alphanumeric")
	if want != got {
		t.Fatalf("alphanumeric: %s != %s", want, got)
	}

	want = "你好世界"
	got = payeeStripNonAlphanumeric("你好世界")
	if want != got {
		t.Fatalf("non-english: %s != %s", want, got)
	}

	want = "Im not j ust alphanumeric"
	got = payeeStripNonAlphanumeric("Im! not j.ust alphanumeric42 69")
	if want != got {
		t.Fatalf("non-alphanumeric: %s != %s", want, got)
	}
}
