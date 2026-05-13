package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/martinohansen/ynabber/internal/log"
)

const maxResponseBodyBytes = 10 * 1024 * 1024

type Transaction struct {
	Account       string `json:"account"`
	Date          string `json:"date"`
	Amount        int64  `json:"amount"`
	PayeeName     string `json:"payee_name,omitempty"`
	Notes         string `json:"notes,omitempty"`
	ImportedPayee string `json:"imported_payee,omitempty"`
	ImportedID    string `json:"imported_id,omitempty"`
	Cleared       *bool  `json:"cleared,omitempty"`
}

type importTransactionsRequest struct {
	Transactions    []Transaction `json:"transactions"`
	DefaultCleared  bool          `json:"defaultCleared"`
	ReimportDeleted bool          `json:"reimportDeleted"`
	DryRun          bool          `json:"dryRun"`
}

type importTransactionsResponse struct {
	Data struct {
		Added   []string          `json:"added"`
		Updated []string          `json:"updated"`
		Errors  []json.RawMessage `json:"errors"`
	} `json:"data"`
}

type ImportTransactionsOptions struct {
	DefaultCleared  bool
	ReimportDeleted bool
	DryRun          bool
}

type ImportTransactionsResult struct {
	Added   int
	Updated int
}

type Client struct {
	baseURL            string
	apiKey             string
	encryptionPassword string
	httpClient         *http.Client
	logger             *slog.Logger
}

// NewClient returns a new Actual Budget API client. If httpClient is nil, a
// default client with a 30 s timeout is used. If logger is nil, the default
// slog logger is used.
func NewClient(baseURL, apiKey, encryptionPassword string, httpClient *http.Client, logger *slog.Logger) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL:            strings.TrimSuffix(baseURL, "/"),
		apiKey:             apiKey,
		encryptionPassword: encryptionPassword,
		httpClient:         httpClient,
		logger:             logger,
	}
}

// ImportTransactions sends transactions to Actual Budget using the import
// endpoint, which reconciles duplicates using imported_id.
func (c *Client) ImportTransactions(ctx context.Context, budgetID, accountID string, transactions []Transaction, opts ImportTransactionsOptions) (ImportTransactionsResult, error) {
	reqBody := importTransactionsRequest{
		Transactions:    transactions,
		DefaultCleared:  opts.DefaultCleared,
		ReimportDeleted: opts.ReimportDeleted,
		DryRun:          opts.DryRun,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/budgets/%s/accounts/%s/transactions/import", c.baseURL, url.PathEscape(budgetID), url.PathEscape(accountID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	if c.encryptionPassword != "" {
		req.Header.Set("budget-encryption-password", c.encryptionPassword)
	}

	log.Trace(c.logger, "http request", "method", req.Method, "url", req.URL.String(), "body", payload)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("sending request: %w", err)
	}
	defer res.Body.Close()

	resPayload, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBodyBytes))
	if err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("reading response body: %w", err)
	}

	log.Trace(c.logger, "http response", "status", res.StatusCode, "body", resPayload)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ImportTransactionsResult{}, fmt.Errorf("actual api response %d: %s", res.StatusCode, responseError(resPayload))
	}

	var response importTransactionsResponse
	if err := json.Unmarshal(resPayload, &response); err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("parsing response body: %w", err)
	}
	if len(response.Data.Errors) > 0 {
		parts := make([]string, 0, len(response.Data.Errors))
		for _, importErr := range response.Data.Errors {
			parts = append(parts, importErrorMessage(importErr))
		}
		return ImportTransactionsResult{}, fmt.Errorf("actual import errors: %s", strings.Join(parts, "; "))
	}

	result := ImportTransactionsResult{
		Added:   len(response.Data.Added),
		Updated: len(response.Data.Updated),
	}

	c.logger.Info(
		"imported transactions",
		"added", result.Added,
		"updated", result.Updated,
	)

	return result, nil
}

func importErrorMessage(raw json.RawMessage) string {
	var importErr struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &importErr); err == nil && importErr.Message != "" {
		return importErr.Message
	}
	return string(raw)
}

func responseError(payload []byte) string {
	var response struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(payload, &response); err == nil && response.Error != "" {
		return response.Error
	}
	return string(payload)
}
