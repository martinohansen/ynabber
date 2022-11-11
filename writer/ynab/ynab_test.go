package ynab

import (
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

func TestImportIDMaker(t *testing.T) {
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
			name: "v1",
			args: args{
				defaultConfig,
				ynabber.Transaction{
					Amount: ynabber.Milliunits(-100000),
					Date:   time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC),
					Memo:   "foo",
				},
			},
			want: "YBBR:-100000:2000-01-01:2c26",
		},

		{
			name: "v2",
			args: args{
				ynabber.Config{
					YNAB: ynabber.YNAB{
						ImportID: ynabber.ImportID{
							V2: ynabber.Date(time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC)),
						},
					},
				},
				ynabber.Transaction{Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC)},
			},
			want: "YBBR:5ca3430298b7fb93d2f4fe1e302",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := importIDMaker(tt.args.cfg, tt.args.t)
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
