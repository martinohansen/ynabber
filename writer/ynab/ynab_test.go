package ynab

import (
	"reflect"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

func TestMakeID(t *testing.T) {
	// The import IDs cant be more then 32 chars
	var maxLength = 32

	// Read config from env
	var defaultConfig ynabber.Config
	err := envconfig.Process("", &defaultConfig)
	if err != nil {
		t.Fatal(err.Error())
	}

	type args struct {
		cfg ynabber.Config
		t   ynabber.Transaction
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "v2",
			args: args{
				ynabber.Config{},
				ynabber.Transaction{Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC)},
			},
			want: "YBBR:5ca3430298b7fb93d2f4fe1e302",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeID(tt.args.t)
			// Test max length of all test cases
			if len(got) > maxLength {
				t.Errorf("importIDMaker() = %v chars long, max length is %v", len(got), maxLength)
			}
			// Verify hashed output
			if got != tt.want {
				t.Errorf("importIDMaker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccountParser(t *testing.T) {
	type args struct {
		account    string
		accountMap map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "match",
			args:    args{account: "N1", accountMap: map[string]string{"N1": "Y1"}},
			want:    "Y1",
			wantErr: false,
		},
		{name: "noMatch",
			args:    args{account: "im-not-here", accountMap: map[string]string{"foo": "bar"}},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := accountParser(tt.args.account, tt.args.accountMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestYnabberToYNAB(t *testing.T) {
	type args struct {
		cfg ynabber.Config
		t   ynabber.Transaction
	}
	tests := []struct {
		name    string
		args    args
		want    Transaction
		wantErr bool
	}{
		{
			name: "Default",
			args: args{
				cfg: ynabber.Config{
					YNAB: ynabber.YNAB{
						AccountMap: map[string]string{"foobar": "abc"},
					},
				},
				t: ynabber.Transaction{
					Account: ynabber.Account{IBAN: "foobar"},
					Amount:  10000,
				},
			},
			want: Transaction{
				AccountID: "abc",
				Date:      "0001-01-01",
				Amount:    "10000",
				ImportID:  "YBBR:e066d58050f67a602720e5f123f",
				Approved:  false,
			},
			wantErr: false,
		},
		{
			name: "SwapFlow",
			args: args{
				cfg: ynabber.Config{
					YNAB: ynabber.YNAB{
						SwapFlow:   []string{"foobar"},
						AccountMap: map[string]string{"foobar": "abc"},
					},
				},
				t: ynabber.Transaction{
					Account: ynabber.Account{IBAN: "foobar"},
					Amount:  10000,
				},
			},
			want: Transaction{
				AccountID: "abc",
				Date:      "0001-01-01",
				Amount:    "-10000",
				ImportID:  "YBBR:2e18b15a1a51f0c2278147a4ca5",
				Approved:  false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := NewWriter(&tt.args.cfg)
			got, err := writer.toYNAB(tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidTransaction(t *testing.T) {
	yesterday := time.Now().AddDate(-1, 0, 0)
	writer := Writer{Config: &ynabber.Config{}, logger: nil}

	tests := []struct {
		name     string
		date     time.Time
		fromDate time.Time
		delay    time.Duration
		want     bool
	}{
		{
			name:     "Yesterday",
			date:     time.Now().AddDate(0, 0, -1),
			fromDate: yesterday,
			delay:    0, // Default value
			want:     true,
		},
		{
			name:     "Day before yesterday (25h delay)",
			date:     time.Now().AddDate(0, 0, -2),
			fromDate: yesterday,
			delay:    25 * time.Hour,
			want:     true,
		},
		{
			name:     "Tomorrow",
			date:     time.Now().AddDate(0, 0, 1),
			fromDate: yesterday,
			want:     false,
		},
		{
			name:     "5 years ago",
			date:     time.Now().AddDate(-5, 0, 0),
			fromDate: yesterday,
			want:     false,
		},
		{
			name:     "Before FromDate",
			date:     yesterday.AddDate(0, 0, -1),
			fromDate: yesterday,
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer.Config.YNAB.FromDate = ynabber.Date(tt.fromDate)
			writer.Config.YNAB.Delay = tt.delay

			if got := writer.checkTransactionDateValidity(tt.date); got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}
