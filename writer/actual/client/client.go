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

// maxMediaTypeLen bounds the server-supplied media type echoed into errors.
const maxMediaTypeLen = 64

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
	Data *importTransactionsResponseData `json:"data"`
}

type importTransactionsResponseData struct {
	// Pointers distinguish a missing or null field from a valid empty array.
	// Only errors is required: silently treating a missing errors array as
	// success would hide real import failures. added and updated are reported
	// as zero when absent, because they only feed import metrics.
	Added   *[]string          `json:"added"`
	Updated *[]string          `json:"updated"`
	Errors  *[]json.RawMessage `json:"errors"`
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
	payload, err := marshalImportTransactionsRequest(transactions, opts)
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

	log.Trace(
		c.logger,
		"http request",
		"method", req.Method,
		"account_id", accountID,
		"transactions", len(transactions),
		"request_bytes", len(payload),
	)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("sending request: %w", err)
	}
	defer res.Body.Close()

	resPayload, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBodyBytes))
	if err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("reading response body: %w", err)
	}

	log.Trace(
		c.logger,
		"http response",
		"account_id", accountID,
		"status", res.StatusCode,
		"response_bytes", len(resPayload),
	)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ImportTransactionsResult{}, fmt.Errorf("actual api response %d: %s", res.StatusCode, responseError(resPayload, res.Header.Get("Content-Type")))
	}

	var response importTransactionsResponse
	if err := json.Unmarshal(resPayload, &response); err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("parsing response body: %w", err)
	}
	if response.Data == nil {
		return ImportTransactionsResult{}, fmt.Errorf("parsing response body: missing data object")
	}
	// A missing errors array would hide real import failures, so it stays
	// fatal. added and updated only feed import metrics, so a response that
	// omits them degrades the summary rather than the import: failing an
	// otherwise successful batch over a counter would be the worse outcome.
	if response.Data.Errors == nil {
		return ImportTransactionsResult{}, fmt.Errorf("parsing response body: missing data.errors array")
	}
	if response.Data.Added == nil || response.Data.Updated == nil {
		c.logger.Warn(
			"actual response omitted import counts; summary totals will undercount",
			"account_id", accountID,
			"has_added", response.Data.Added != nil,
			"has_updated", response.Data.Updated != nil,
		)
	}

	result := ImportTransactionsResult{
		Added:   countIDs(response.Data.Added),
		Updated: countIDs(response.Data.Updated),
	}
	if len(*response.Data.Errors) > 0 {
		parts := make([]string, 0, len(*response.Data.Errors))
		for _, importErr := range *response.Data.Errors {
			parts = append(parts, importErrorMessage(importErr))
		}
		// Actual normally reports no changes alongside import errors, but its
		// contract does not guarantee atomicity. Preserve any reported counts so
		// callers can describe a partial result accurately.
		return result, fmt.Errorf("actual import errors: %s", strings.Join(parts, "; "))
	}

	return result, nil
}

// ImportTransactionsRequestSize returns the exact number of bytes that
// ImportTransactions will use for its JSON request body.
func ImportTransactionsRequestSize(transactions []Transaction, opts ImportTransactionsOptions) (int, error) {
	payload, err := marshalImportTransactionsRequest(transactions, opts)
	if err != nil {
		return 0, err
	}
	return len(payload), nil
}

func marshalImportTransactionsRequest(transactions []Transaction, opts ImportTransactionsOptions) ([]byte, error) {
	return json.Marshal(importTransactionsRequest{
		Transactions:    transactions,
		DefaultCleared:  opts.DefaultCleared,
		ReimportDeleted: opts.ReimportDeleted,
		DryRun:          opts.DryRun,
	})
}

// countIDs reports how many IDs an optional response array carried, treating a
// missing or null array as none reported.
func countIDs(ids *[]string) int {
	if ids == nil {
		return 0
	}
	return len(*ids)
}

func importErrorMessage(raw json.RawMessage) string {
	var importErr struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &importErr); err == nil && importErr.Message != "" {
		return importErr.Message
	}
	return fmt.Sprintf("unrecognized import error (%d byte element)", len(raw))
}

func responseError(payload []byte, contentType string) string {
	var response struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(payload, &response); err == nil && response.Error != "" {
		return response.Error
	}
	return fmt.Sprintf("unexpected response (%s)", describeBody(payload, contentType))
}

// describeBody summarizes a payload that carried no recognizable error
// message. It reports the shape of the body rather than its content, so an
// operator can tell a proxy's HTML error page from a malformed JSON response
// without the error text echoing anything the response happened to contain.
func describeBody(payload []byte, contentType string) string {
	if len(payload) == 0 {
		return "empty body"
	}

	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.TrimSpace(mediaType)
	switch {
	case mediaType == "":
		mediaType = "unknown content type"
	case len(mediaType) > maxMediaTypeLen:
		// The value is server-controlled, so cap it before it reaches a log.
		mediaType = mediaType[:maxMediaTypeLen] + "..."
	}
	return fmt.Sprintf("%d byte %s body", len(payload), mediaType)
}
