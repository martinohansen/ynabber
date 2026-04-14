package enablebanking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/internal/log"
)

// ErrRateLimit is returned when the API responds with HTTP 429 Too Many Requests.
var ErrRateLimit = errors.New("rate limited")

// ErrUnauthorized is returned when the API responds with HTTP 401 Unauthorized,
// indicating the session has been revoked or has expired server-side.
var ErrUnauthorized = errors.New("session rejected by API")

// maxResponseBodyBytes caps how much of an HTTP response body we buffer. 10 MB
// is far above any realistic API payload and guards against a misbehaving or
// compromised upstream exhausting the process's memory.
const maxResponseBodyBytes = 10 * 1024 * 1024

// Client handles HTTP communication with the EnableBanking API
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a new EnableBanking API client
func NewClient(logger *slog.Logger) *Client {
	return &Client{
		BaseURL: enableBankingAPIBase,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// TransactionsResponse represents the transactions response from the API
type TransactionsResponse struct {
	Transactions []EBTransaction `json:"transactions"`
	Pending      []EBTransaction `json:"pending"`
}

// EBTransaction represents a transaction from the EnableBanking API
type EBTransaction struct {
	EntryReference       interface{} `json:"entry_reference"`
	MerchantCategoryCode interface{} `json:"merchant_category_code"`
	TransactionAmount    struct {
		Currency string `json:"currency"`
		Amount   string `json:"amount"`
	} `json:"transaction_amount"`
	Creditor                    interface{} `json:"creditor"`
	CreditorAccount             interface{} `json:"creditor_account"`
	CreditorAgent               interface{} `json:"creditor_agent"`
	Debtor                      interface{} `json:"debtor"`
	DebtorAccount               interface{} `json:"debtor_account"`
	DebtorAgent                 interface{} `json:"debtor_agent"`
	BankTransactionCode         interface{} `json:"bank_transaction_code"`
	CreditDebitIndicator        string      `json:"credit_debit_indicator"`
	Status                      string      `json:"status"`
	BookingDate                 string      `json:"booking_date"`
	ValueDate                   string      `json:"value_date"`
	TransactionDate             interface{} `json:"transaction_date"`
	BalanceAfterTransaction     interface{} `json:"balance_after_transaction"`
	ReferenceNumber             interface{} `json:"reference_number"`
	ReferenceNumberSchema       interface{} `json:"reference_number_schema"`
	RemittanceInformation       []string    `json:"remittance_information"`
	DebtorAccountAdditionalID   interface{} `json:"debtor_account_additional_identification"`
	CreditorAccountAdditionalID interface{} `json:"creditor_account_additional_identification"`
	ExchangeRate                interface{} `json:"exchange_rate"`
	Note                        interface{} `json:"note"`
	TransactionID               string      `json:"transaction_id"`
}

// GetAccountTransactions fetches transactions for a specific account
func (c *Client) GetAccountTransactions(ctx context.Context, jwtToken, accountUID, fromDate, toDate string) (*TransactionsResponse, error) {
	url := fmt.Sprintf("%s/accounts/%s/transactions?date_from=%s&date_to=%s",
		c.BaseURL, accountUID, fromDate, toDate)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("%w: %s", ErrRateLimit, string(respBody))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: %s", ErrUnauthorized, string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var transactions TransactionsResponse
	if err := json.Unmarshal(respBody, &transactions); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &transactions, nil
}

// Reader represents an EnableBanking reader instance
type Reader struct {
	Config Config
	Auth   Auth
	Client *Client
	logger *slog.Logger
	// retryDelay overrides the default backoff between retries.
	// Zero means use retryBaseDelay. Set in tests to keep them fast.
	retryDelay time.Duration
}

// NewReader returns a new EnableBanking reader
func NewReader(logger *slog.Logger, dataDir string) (Reader, error) {
	logger = logger.With("reader", "enablebanking")

	// Load and validate config
	cfg := Config{}
	err := loadEnvConfig(&cfg)
	if err != nil {
		return Reader{}, fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(dataDir); err != nil {
		return Reader{}, fmt.Errorf("validating config: %w", err)
	}

	logger.Debug("config loaded", "aspsp", cfg.ASPSP, "country", cfg.Country)

	auth := NewAuth(cfg, logger)
	client := NewClient(logger)

	return Reader{
		Config: cfg,
		Auth:   auth,
		Client: client,
		logger: logger,
	}, nil
}

// String returns the reader name
func (r Reader) String() string {
	return "enablebanking"
}

// Bulk fetches all accounts and their transactions
func (r Reader) Bulk(ctx context.Context) ([]ynabber.Transaction, error) {
	// Get or create session
	session, err := r.Auth.Session(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}

	if len(session.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts found in session")
	}

	log.Trace(r.logger, "session", "data", session)

	r.logger.Info("loaded session", "accounts", len(session.Accounts))

	// Fetch transactions for each account
	var results []ynabber.Transaction
	fromDate := time.Time(r.Config.FromDate).Format(dateFormat)
	toDateTime, err := r.Config.GetToDate()
	if err != nil {
		return nil, fmt.Errorf("getting to date: %w", err)
	}
	toDate := toDateTime.Format(dateFormat)

	for i, account := range session.Accounts {
		accountLogger := r.logger.With("account", account.UID, "stable_id_hint", maskIdentifier(account.StableID()))
		log.Trace(accountLogger, "stable id", "stable_id", account.StableID())

		// Warn when the session file predates the account_id fix (issue #152).
		// In that case StableID() falls back to the session-scoped UID, which
		// won't match any YNAB_ACCOUNTMAP key and transactions will be dropped.
		if account.AccountID.IBAN == "" && account.AccountID.Other.Identification == "" {
			accountLogger.Warn("account has no stable ID (IBAN/BBAN/CPAN) — " +
				"delete the session file and re-authorize to fix YNAB_ACCOUNTMAP matching")
		}

		txResp, err := r.Client.GetAccountTransactions(ctx, session.AuthToken, account.UID, fromDate, toDate)
		if err != nil {
			accountLogger.Error("fetching transactions", "error", err)
			continue
		}

		log.Trace(accountLogger, "transactions", "data", txResp)

		accountLogger.Info("fetched transactions", "booked", len(txResp.Transactions), "pending", len(txResp.Pending))

		// Process booked transactions
		for _, ebTx := range txResp.Transactions {
			tx, err := r.Mapper(account, ebTx)
			if err != nil {
				accountLogger.Debug("skipping transaction", "error", err, "id", ebTx.TransactionID)
				continue
			}

			if tx != nil {
				results = append(results, *tx)
			}
		}

		// Log progress
		rate := float64(i+1) / float64(len(session.Accounts)) * 100
		accountLogger.Debug("processed", "progress_pct", fmt.Sprintf("%.0f%%", rate))
	}

	r.logger.Info("read transactions", "total", len(results))
	return results, nil
}

// maskIdentifier returns a truncated identifier safe for INFO-level logs,
// showing only the first 4 and last 4 characters (e.g. "NO98...8901").
// This avoids emitting full IBANs or BBANs to log aggregators.
func maskIdentifier(id string) string {
	r := []rune(id)
	if len(r) <= 8 {
		return "****"
	}
	return string(r[:4]) + "..." + string(r[len(r)-4:])
}

// loadEnvConfig loads config from environment variables using kelseyhightower/envconfig
func loadEnvConfig(cfg *Config) error {
	if err := envconfig.Process("", cfg); err != nil {
		return fmt.Errorf("processing config: %w", err)
	}
	return nil
}
