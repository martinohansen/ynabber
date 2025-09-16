package actual

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/martinohansen/ynabber/internal/log"
)

type Options struct {
	RunTransfers    bool
	LearnCategories bool
}

type Transaction struct {
	Account       string `json:"account"`
	Date          string `json:"date"`
	Amount        int64  `json:"amount"`
	PayeeName     string `json:"payee_name,omitempty"`
	Payee         string `json:"payee,omitempty"`
	Notes         string `json:"notes,omitempty"`
	ImportedPayee string `json:"imported_payee,omitempty"`
	Category      string `json:"category,omitempty"`
	ImportedID    string `json:"imported_id,omitempty"`
	Cleared       *bool  `json:"cleared,omitempty"`
	TransferID    string `json:"transfer_id,omitempty"`
}

type addTransactionsRequest struct {
	Transactions    []Transaction `json:"transactions"`
	RunTransfers    bool          `json:"runTransfers"`
	LearnCategories bool          `json:"learnCategories"`
}

type Client struct {
	baseURL            string
	apiKey             string
	encryptionPassword string
	httpClient         *http.Client
	logger             *slog.Logger
}

func NewClient(baseURL, apiKey, encryptionPassword string, httpClient *http.Client, logger *slog.Logger) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
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

func (c *Client) BatchTransactions(ctx context.Context, budgetID, accountID string, transactions []Transaction, opts Options) error {
	reqBody := addTransactionsRequest{
		Transactions:    transactions,
		RunTransfers:    opts.RunTransfers,
		LearnCategories: opts.LearnCategories,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/budgets/%s/accounts/%s/transactions/batch", c.baseURL, budgetID, accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
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
		return fmt.Errorf("sending request: %w", err)
	}
	defer res.Body.Close()

	resPayload, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	log.Trace(c.logger, "http response", "status", res.StatusCode, "body", resPayload)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("actual api response %d: %s", res.StatusCode, string(resPayload))
	}

	return nil
}
