package enablebanking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
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

// AccountDetails represents the account details response from the EnableBanking API
type AccountDetails struct {
	AccountID struct {
		IBAN string `json:"iban"`
	} `json:"account_id"`
	AllAccountIDs []struct {
		Identification string `json:"identification"`
		SchemeName     string `json:"scheme_name"`
	} `json:"all_account_ids"`
	AccountServicer struct {
		BicFi string `json:"bic_fi"`
		Name  string `json:"name"`
	} `json:"account_servicer"`
	Name            string `json:"name"`
	Details         string `json:"details"`
	Usage           string `json:"usage"`
	CashAccountType string `json:"cash_account_type"`
	Product         string `json:"product"`
	Currency        string `json:"currency"`
	PSUStatus       string `json:"psu_status"`
	CreditLimit     struct {
		Currency string `json:"currency"`
		Amount   string `json:"amount"`
	} `json:"credit_limit"`
	UID                  string   `json:"uid"`
	IdentificationHash   string   `json:"identification_hash"`
	IdentificationHashes []string `json:"identification_hashes"`
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

// GetAccountDetails fetches detailed information for a specific account
func (c *Client) GetAccountDetails(ctx context.Context, jwtToken, accountUID string) (*AccountDetails, error) {
	url := fmt.Sprintf("%s/accounts/%s/details", c.BaseURL, accountUID)

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

	var details AccountDetails
	if err := json.Unmarshal(respBody, &details); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &details, nil
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

	if cfg.Debug {
		logger.Warn("ENABLEBANKING_DEBUG is enabled — raw API responses will be written to disk; NEVER use in production")
	}

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

	// Debug: dump session JSON to file
	if r.Config.Debug {
		if err := dumpJSON("session.json", session); err != nil {
			r.logger.Warn("failed to dump session JSON", "error", err)
		}
	}

	r.logger.Info("loaded session", "accounts", len(session.Accounts))

	// Fetch and log a summary of all account details for easy identification
	accountDetailsMap := make(map[string]*AccountDetails)
	for _, account := range session.Accounts {
		details, err := r.Client.GetAccountDetails(ctx, session.AuthToken, account.UID)
		if err != nil {
			r.logger.Warn("failed to fetch account details", "uid", account.UID, "error", err)
			continue
		}

		accountDetailsMap[account.UID] = details

		// Debug: dump account details JSON to file
		if r.Config.Debug {
			if err := dumpJSON(fmt.Sprintf("account_%s_details.json", account.UID), details); err != nil {
				r.logger.Warn("failed to dump account details JSON", "uid", account.UID, "error", err)
			}
		}

		// Log account details for easy identification
		r.logger.Info("account_details",
			"uid", details.UID,
			"account_id", details.AccountID,
			"details", details.Details,
		)
	}

	// Fetch transactions for each account
	var results []ynabber.Transaction
	fromDate := r.Config.FromDate
	toDate := r.Config.ToDate

	for i, account := range session.Accounts {
		details := accountDetailsMap[account.UID]

		logFields := []interface{}{
			"account", account.UID,
		}
		if details != nil {
			logFields = append(logFields, "iban", details.AccountID.IBAN)
			// Enrich account with IBAN from details API
			account.IBAN = details.AccountID.IBAN
		} else {
			logFields = append(logFields, "iban", account.IBAN)
		}

		accountLogger := r.logger.With(logFields...)

		txResp, err := r.Client.GetAccountTransactions(ctx, session.AuthToken, account.UID, fromDate, toDate)
		if err != nil {
			accountLogger.Error("fetching transactions", "error", err)
			continue
		}

		// Debug: dump transactions JSON to file
		if r.Config.Debug {
			if err := dumpJSON(fmt.Sprintf("account_%s_transactions.json", account.UID), txResp); err != nil {
				accountLogger.Warn("failed to dump transactions JSON", "error", err)
			}
		}

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

// loadEnvConfig loads config from environment variables using kelseyhightower/envconfig
func loadEnvConfig(cfg *Config) error {
	if err := envconfig.Process("", cfg); err != nil {
		return fmt.Errorf("processing config: %w", err)
	}
	return nil
}

// dumpJSON writes data as indented JSON to transactions/<filename>.
// Only called when ENABLEBANKING_DEBUG=true. Never use in production —
// output contains unredacted session tokens and full transaction history.
func dumpJSON(filename string, data interface{}) error {
	const dir = "transactions"
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating debug directory: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	// filepath.Base strips any directory components from filenames that
	// originate from API responses (e.g. account UIDs), preventing a
	// compromised upstream from writing outside the transactions/ directory.
	dest := filepath.Join(dir, filepath.Base(filename))
	if err := os.WriteFile(dest, jsonBytes, 0600); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}
