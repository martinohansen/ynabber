package ynab

import (
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

func TestMakeID(t *testing.T) {
	// The import IDs cant be more then 32 chars
	var maxLength = 32

	// Read config from env
	var defaultConfig Config
	err := envconfig.Process("", &defaultConfig)
	if err != nil {
		t.Fatal(err.Error())
	}

	type args struct {
		cfg Config
		t   ynabber.Transaction
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "with IBAN (nordigen/enablebanking)",
			args: args{
				Config{},
				ynabber.Transaction{
					Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
					Account: ynabber.Account{
						IBAN: "NO1234567890",
					},
				},
			},
			want: "YBBR:acf540311af299dbd9f543c0211",
		},
		{
			// IBAN is preferred over Account ID to preserve backward
			// compatibility with existing Nordigen import IDs. The hash must
			// equal the IBAN-only case above.
			name: "with Account ID and IBAN (IBAN preferred — backward compat)",
			args: args{
				Config{},
				ynabber.Transaction{
					Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
					Account: ynabber.Account{
						ID:   "account-uid-123",
						IBAN: "NO1234567890",
					},
				},
			},
			want: "YBBR:acf540311af299dbd9f543c0211",
		},
		{
			// Account ID is used when no IBAN is present (rare account types).
			name: "with Account ID only (no IBAN — fallback)",
			args: args{
				Config{},
				ynabber.Transaction{
					Date: time.Date(2022, 12, 24, 0, 0, 0, 0, time.UTC),
					Account: ynabber.Account{
						ID: "account-uid-123",
					},
				},
			},
			want: "YBBR:da39cbb8db30d3b58ed861a97fd",
		},
		{
			name: "without ID or IBAN",
			args: args{
				Config{},
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
				t.Errorf("makeID() = %v chars long, max length is %v", len(got), maxLength)
			}
			// Verify hashed output
			if got != tt.want {
				t.Errorf("makeID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccountParser(t *testing.T) {
	type args struct {
		account    ynabber.Account
		accountMap map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "match by ID (enablebanking)",
			args: args{
				account:    ynabber.Account{ID: "account-uid-123", IBAN: "NO1234567890"},
				accountMap: map[string]string{"account-uid-123": "Y1"},
			},
			want:    "Y1",
			wantErr: false,
		},
		{
			name: "match by IBAN when ID not in map (nordigen)",
			args: args{
				account:    ynabber.Account{ID: "nordigen-id-456", IBAN: "NO9876543210"},
				accountMap: map[string]string{"NO9876543210": "Y2"},
			},
			want:    "Y2",
			wantErr: false,
		},
		{
			name: "ID takes precedence over IBAN",
			args: args{
				account:    ynabber.Account{ID: "account-uid-789", IBAN: "NO1234567890"},
				accountMap: map[string]string{"account-uid-789": "Y1", "NO1234567890": "Y2"},
			},
			want:    "Y1",
			wantErr: false,
		},
		{
			name: "no match",
			args: args{
				account:    ynabber.Account{ID: "unknown-id", IBAN: "NO9999999999"},
				accountMap: map[string]string{"foo": "bar"},
			},
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
	logger := slog.Default()

	type args struct {
		cfg Config
		t   ynabber.Transaction
	}
	tests := []struct {
		name    string
		args    args
		want    Transaction
		wantErr bool
	}{
		{
			name: "Default with IBAN",
			args: args{
				cfg: Config{
					AccountMap: map[string]string{"foobar": "abc"},
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
			name: "With Account ID (enablebanking)",
			args: args{
				cfg: Config{
					AccountMap: map[string]string{"account-uid-123": "def"},
				},
				t: ynabber.Transaction{
					Account: ynabber.Account{ID: "account-uid-123", IBAN: "NO1234567890"},
					Amount:  5000,
				},
			},
			want: Transaction{
				AccountID: "def",
				Date:      "0001-01-01",
				Amount:    "5000",
				ImportID:  "YBBR:dabd11f760cd4eb509d4a25ea8d",
				Approved:  false,
			},
			wantErr: false,
		},
		{
			name: "SwapFlow with IBAN",
			args: args{
				cfg: Config{
					SwapFlow:   []string{"foobar"},
					AccountMap: map[string]string{"foobar": "abc"},
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
		{
			name: "SwapFlow with Account ID",
			args: args{
				cfg: Config{
					SwapFlow:   []string{"account-uid-456"},
					AccountMap: map[string]string{"account-uid-456": "ghi"},
				},
				t: ynabber.Transaction{
					Account: ynabber.Account{ID: "account-uid-456", IBAN: "NO9876543210"},
					Amount:  7500,
				},
			},
			want: Transaction{
				AccountID: "ghi",
				Date:      "0001-01-01",
				Amount:    "-7500",
				ImportID:  "YBBR:6d02c7734ee0b3230ff324efdbe",
				Approved:  false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := Writer{
				Config: tt.args.cfg,
				logger: logger,
			}
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
	writer := Writer{Config: Config{}, logger: nil}

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
			writer.Config.FromDate = Date(tt.fromDate)
			writer.Config.Delay = tt.delay

			if got := writer.checkTransactionDateValidity(tt.date); got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestToYNABPayeeAndMemo covers payee/memo truncation and whitespace
// normalisation in Writer.toYNAB.
func TestToYNABPayeeAndMemo(t *testing.T) {
	const testIBAN = "TEST-IBAN-001"
	const testAccountID = "ynab-account-id-001"

	writer := Writer{
		Config: Config{
			AccountMap: map[string]string{testIBAN: testAccountID},
		},
		logger: slog.Default(),
	}

	// 201 'a' characters — one beyond the 200-char limit.
	longString := strings.Repeat("a", 201)
	// 205 chars of "word " repeated — verifies truncation works after
	// whitespace normalisation has already collapsed runs of spaces.
	longWords := strings.Repeat("word ", 41) // 41 × 5 = 205 chars

	baseTransaction := ynabber.Transaction{
		Account: ynabber.Account{IBAN: testIBAN},
		Amount:  1000,
		Date:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}

	type args struct {
		payee string
		memo  string
		iban  string // leave empty to use testIBAN
	}
	tests := []struct {
		name          string
		args          args
		wantErr       bool
		checkPayeeLen bool   // assert len(PayeeName) <= maxPayeeSize
		checkMemoLen  bool   // assert len(Memo) <= maxMemoSize
		wantPayee     string // exact match when non-empty
		wantMemo      string // exact match when non-empty
	}{
		{
			// A payee that exceeds 200 chars must be silently truncated.
			// The implementation slices [0:(maxPayeeSize-1)], yielding 199 chars.
			name: "payee truncation: payee longer than 200 chars is truncated",
			args: args{
				payee: longString, // 201 chars
				memo:  "normal memo",
			},
			wantErr:       false,
			checkPayeeLen: true,
		},
		{
			// A memo that exceeds 200 chars must be silently truncated.
			// The implementation slices [0:(maxMemoSize-1)], yielding 199 chars.
			name: "memo truncation: memo longer than 200 chars is truncated",
			args: args{
				payee: "normal payee",
				memo:  longString, // 201 chars
			},
			wantErr:      false,
			checkMemoLen: true,
		},
		{
			// Multi-word payee over the limit — verifies truncation after normalisation.
			name: "payee truncation: long multi-word payee is truncated after normalisation",
			args: args{
				payee: longWords, // 205 chars of "word " repeated
				memo:  "normal memo",
			},
			wantErr:       false,
			checkPayeeLen: true,
		},
		{
			// Consecutive spaces inside a payee must be collapsed to one space.
			name: "whitespace normalisation: consecutive spaces in payee are collapsed",
			args: args{
				payee: "Hello  World", // two spaces
				memo:  "normal memo",
			},
			wantErr:   false,
			wantPayee: "Hello World",
		},
		{
			// Tabs mixed with spaces in a payee must also be collapsed.
			name: "whitespace normalisation: tabs and spaces in payee are collapsed",
			args: args{
				payee: "Foo\t\t Bar   Baz",
				memo:  "normal memo",
			},
			wantErr:   false,
			wantPayee: "Foo Bar Baz",
		},
		{
			// Leading and trailing whitespace must be stripped entirely.
			name: "whitespace normalisation: leading and trailing spaces in payee are trimmed",
			args: args{
				payee: "  trimmed  ",
				memo:  "normal memo",
			},
			wantErr:   false,
			wantPayee: "trimmed",
		},
		{
			// Unknown IBAN has no entry in AccountMap → error expected.
			name: "error: unknown IBAN returns error",
			args: args{
				payee: "any payee",
				memo:  "any memo",
				iban:  "UNKNOWN-IBAN",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := baseTransaction
			tx.Payee = tt.args.payee
			tx.Memo = tt.args.memo
			if tt.args.iban != "" {
				tx.Account.IBAN = tt.args.iban
			}

			got, err := writer.toYNAB(tx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("toYNAB() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if tt.checkPayeeLen && len(got.PayeeName) > maxPayeeSize {
				t.Errorf("PayeeName length = %d, want <= %d", len(got.PayeeName), maxPayeeSize)
			}

			if tt.checkMemoLen && len(got.Memo) > maxMemoSize {
				t.Errorf("Memo length = %d, want <= %d", len(got.Memo), maxMemoSize)
			}

			if tt.wantPayee != "" && got.PayeeName != tt.wantPayee {
				t.Errorf("PayeeName = %q, want %q", got.PayeeName, tt.wantPayee)
			}

			if tt.wantMemo != "" && got.Memo != tt.wantMemo {
				t.Errorf("Memo = %q, want %q", got.Memo, tt.wantMemo)
			}
		})
	}
}
