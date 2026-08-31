package wealthreader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
)

// ErrRateLimit is returned when the API responds with HTTP 429.
var ErrRateLimit = errors.New("rate limited")

// ErrUnauthorized is returned when the stored token is no longer usable
// (password change, new 2FA, consent expired). Bulk then starts a new OAuth.
var ErrUnauthorized = errors.New("session rejected by API")

// authErrorCodes are Wealth Reader error codes that mean "ask the user again".
// Do not retry an invalid password in a tight loop (docs: iframe-backend).
var authErrorCodes = map[int]struct{}{
	1010: {}, // invalid credentials
	1020: {}, // 2FA / SCA required
	2010: {}, // login failed
	2020: {}, // token / session invalid
}

type Client struct {
	HTTPClient *http.Client
	logger     *slog.Logger
	config     Config
}

func NewClient(cfg Config, logger *slog.Logger) *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		logger:     logger,
		config:     cfg,
	}
}

// Fetch refreshes accounts + transactions with the stored token.
// POST https://api.wealthreader.com/entities/  api_key + code + token + date_from.
func (c *Client) Fetch(ctx context.Context, session Session) (Response, error) {
	fromDate := time.Time(c.config.FromDate).Format(dateFormat)

	form := url.Values{}
	form.Set("api_key", c.config.ApiKey)
	form.Set("code", session.Code)
	form.Set("token", session.Token)
	form.Set("product_types", c.config.ProductTypes)
	form.Set("date_from", fromDate)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.ApiBase+"/entities/", strings.NewReader(form.Encode()))
	if err != nil {
		return Response{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return Response{}, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return Response{}, fmt.Errorf("%w: %s", ErrRateLimit, string(body))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return Response{}, fmt.Errorf("%w: %s", ErrUnauthorized, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	parsed, err := parseResponse(body)
	if err != nil {
		return Response{}, err
	}
	return parsed, nil
}

func parseResponse(body []byte) (Response, error) {
	var parsed Response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Response{}, fmt.Errorf("parsing response: %w", err)
	}
	if !parsed.Success {
		if parsed.Error != nil {
			if _, auth := authErrorCodes[parsed.Error.Code]; auth {
				return Response{}, fmt.Errorf("%w: %d %s", ErrUnauthorized, parsed.Error.Code, parsed.Error.Message)
			}
			return Response{}, fmt.Errorf("wealthreader error %d: %s", parsed.Error.Code, parsed.Error.Message)
		}
		return Response{}, errors.New("wealthreader returned success=false")
	}
	return parsed, nil
}

type Reader struct {
	Config     Config
	Auth       Auth
	Client     *Client
	logger     *slog.Logger
	bulkFn     func(context.Context) ([]ynabber.Transaction, error)
	afterFn    func(time.Duration) <-chan time.Time
	retryDelay time.Duration
}

func NewReader(logger *slog.Logger, dataDir string) (Reader, error) {
	logger = logger.With("reader", "wealthreader")

	cfg := Config{}
	if err := envconfig.Process("", &cfg); err != nil {
		return Reader{}, fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(dataDir); err != nil {
		return Reader{}, fmt.Errorf("validating config: %w", err)
	}

	logger.Debug("config loaded", "code", cfg.Code)

	return Reader{
		Config: cfg,
		Auth:   NewAuth(cfg, logger),
		Client: NewClient(cfg, logger),
		logger: logger,
	}, nil
}

func (r Reader) String() string {
	return "wealthreader"
}

func (r Reader) Bulk(ctx context.Context) ([]ynabber.Transaction, error) {
	session, authorized, err := r.Auth.acquireSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}

	results, err := r.fetchSessionTransactions(ctx, session)
	if !errors.Is(err, ErrUnauthorized) {
		return results, err
	}
	if authorized {
		return nil, r.rejectedNewSessionError(err)
	}

	r.logger.Info("session rejected by API; initiating new authorization")
	replacement, reauthErr := r.Auth.reauthorize(ctx)
	if reauthErr != nil {
		return nil, fmt.Errorf("reauthorizing rejected session: %w", reauthErr)
	}

	results, err = r.fetchSessionTransactions(ctx, replacement)
	if !errors.Is(err, ErrUnauthorized) {
		return results, err
	}
	return nil, r.rejectedNewSessionError(err)
}

func (r Reader) rejectedNewSessionError(fetchErr error) error {
	sessionErr := fmt.Errorf("%w: newly authorized session rejected by API: %w", ErrSessionExpired, fetchErr)
	if invalidateErr := r.Auth.invalidateSession(); invalidateErr != nil {
		return fmt.Errorf("%w; discarding rejected session: %w", sessionErr, invalidateErr)
	}
	return sessionErr
}

func (r Reader) fetchSessionTransactions(ctx context.Context, session Session) ([]ynabber.Transaction, error) {
	resp, err := r.Client.Fetch(ctx, session)
	if err != nil {
		return nil, err
	}
	if len(resp.Payload.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts found in response")
	}

	r.logger.Info("loaded accounts", "accounts", len(resp.Payload.Accounts), "entity", resp.Statistics.Code)

	var results []ynabber.Transaction
	for i, account := range resp.Payload.Accounts {
		accountLogger := r.logger.With("account", maskIdentifier(account.UUID), "iban_hint", maskIdentifier(account.Code))
		accountLogger.Info("fetched transactions", "booked", len(account.Transactions))

		for _, wrTx := range account.Transactions {
			tx, err := r.Mapper(account, wrTx)
			if err != nil {
				accountLogger.Debug("skipping transaction", "error", err, "id", wrTx.UUID)
				continue
			}
			if tx != nil {
				results = append(results, *tx)
			}
		}

		rate := float64(i+1) / float64(len(resp.Payload.Accounts)) * 100
		accountLogger.Debug("processed", "progress_pct", fmt.Sprintf("%.0f%%", rate))
	}

	r.logger.Info("read transactions", "total", len(results))
	return results, nil
}

func maskIdentifier(id string) string {
	r := []rune(id)
	if len(r) <= 8 {
		return "****"
	}
	return string(r[:4]) + "..." + string(r[len(r)-4:])
}
