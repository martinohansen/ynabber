package enablebanking

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	enableBankingAPIBase = "https://api.enablebanking.com"
	jwtExpiryDuration    = 1 * time.Hour
	// accessRequestDays is how many days of access we ask for when creating a
	// new session. The bank may grant a different window; if it returns a
	// valid_until field we use that for expiry checks instead.
	accessRequestDays = 10
)

// ErrSessionExpired is returned when the API rejects the session (HTTP 401)
// or when the session's valid_until timestamp has passed.
var ErrSessionExpired = errors.New("session expired")

// Claims represents the JWT claims for EnableBanking API
type Claims struct {
	AppID string `json:"app_id"`
	Sub   string `json:"sub"`
	Aud   string `json:"aud"`
	jwt.RegisteredClaims
}

// AuthorizationRequest represents the request body for initiating authorization
type AuthorizationRequest struct {
	Access struct {
		ValidUntil string `json:"valid_until"`
	} `json:"access"`
	ASPSP struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"aspsp"`
	State       string `json:"state"`
	RedirectURL string `json:"redirect_url"`
	PSUType     string `json:"psu_type"`
}

// AuthorizationResponse represents the response from the authorization endpoint
type AuthorizationResponse struct {
	URL string `json:"url"`
	ID  string `json:"id"`
}

// SessionRequest represents the request to create a session
type SessionRequest struct {
	Code string `json:"code"`
}

// Session represents an authenticated session with account information
type Session struct {
	CreatedAt  string        `json:"createdAt"`
	ValidUntil string        `json:"valid_until,omitempty"` // set when the API returns it
	Accounts   []AccountInfo `json:"accounts"`
	AuthToken  string        `json:"-"` // Not persisted
}

// IsExpired reports whether the session has definitely expired.
// When the API returns a valid_until timestamp we use it as the authoritative
// expiry. When it is absent we return false and let the API tell us via HTTP
// 401 — there is no reliable local heuristic for when a session expires.
func (s Session) IsExpired() bool {
	if s.ValidUntil == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, s.ValidUntil)
	if err != nil {
		return false // unparseable → assume valid
	}
	return time.Now().UTC().After(t)
}

// AccountInfo represents account information from session
type AccountInfo struct {
	UID         string `json:"uid"`
	IBAN        string `json:"iban"`
	BBAN        string `json:"bban"`
	MaskedPAN   string `json:"maskedPan"`
	Currency    string `json:"currency"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	OwnerName   string `json:"ownerName"`
	AccountType string `json:"accountType"`
	Status      string `json:"status"`
}

// Auth handles authentication and session management for EnableBanking
type Auth struct {
	Config     Config
	httpClient *http.Client
	logger     *slog.Logger
}

// NewAuth creates a new Auth handler
func NewAuth(cfg Config, logger *slog.Logger) Auth {
	return Auth{
		Config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Session attempts to get session from disk, if it fails it will initiate
// a new authorization flow and create a session
func (a Auth) Session(ctx context.Context) (Session, error) {
	// Try to load existing session from disk
	sessionFile, err := os.ReadFile(a.Config.SessionFile)
	if errors.Is(err, os.ErrNotExist) {
		a.logger.Info("session file not found, initiating new authorization")
		return a.createNewSession(ctx)
	} else if err != nil {
		return Session{}, fmt.Errorf("reading session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(sessionFile, &session); err != nil {
		a.logger.Error("parsing session file", "error", err)
		return a.createNewSession(ctx)
	}

	if session.IsExpired() {
		a.logger.Info("session expired (valid_until passed), re-authorization required",
			"file", a.Config.SessionFile,
			"valid_until", session.ValidUntil,
		)
		return Session{}, fmt.Errorf("%w: valid_until=%s", ErrSessionExpired, session.ValidUntil)
	}

	// Placeholder file: accounts are listed but no authorization has been
	// completed yet (createdAt is empty). Treat identically to a missing file.
	if session.CreatedAt == "" {
		a.logger.Info("session file has no createdAt (placeholder), initiating new authorization",
			"file", a.Config.SessionFile,
		)
		return a.createNewSession(ctx)
	}

	jwtToken, err := a.generateJWT()
	if err != nil {
		return Session{}, fmt.Errorf("generating JWT: %w", err)
	}
	session.AuthToken = jwtToken

	a.logger.Info("loaded session from disk", "file", a.Config.SessionFile)
	return session, nil
}

// saveSession persists the session to disk
func (a Auth) saveSession(session Session) error {
	sessionData, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	err = os.WriteFile(a.Config.SessionFile, sessionData, 0600)
	if err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}

	created, parseErr := time.Parse(time.RFC3339, session.CreatedAt)
	if parseErr == nil && session.ValidUntil != "" {
		a.logger.Info("session saved to disk",
			"file", a.Config.SessionFile,
			"created_at", created.Format(time.RFC3339),
			"valid_until", session.ValidUntil,
		)
	} else {
		a.logger.Info("session saved to disk", "file", a.Config.SessionFile)
	}

	return nil
}

// createNewSession initiates authorization and creates a new session
func (a Auth) createNewSession(ctx context.Context) (Session, error) {
	// Load private key and generate JWT
	jwtToken, err := a.generateJWT()
	if err != nil {
		return Session{}, fmt.Errorf("generating JWT: %w", err)
	}

	// Initiate authorization and get URL
	authURL, state, err := a.initiateAuthorization(ctx, jwtToken)
	if err != nil {
		return Session{}, fmt.Errorf("initiating authorization: %w", err)
	}

	// Display authorization instructions
	a.displayAuthorizationInstructions(authURL)

	// Prompt user to paste the full redirect URL; code and state are extracted
	// and validated inside the function.
	code, err := a.promptForRedirectURL(state)
	if err != nil {
		return Session{}, fmt.Errorf("reading authorization code: %w", err)
	}

	// Create session using the authorization code
	session, err := a.createSessionWithCode(ctx, jwtToken, code)
	if err != nil {
		return Session{}, fmt.Errorf("creating session with code: %w", err)
	}

	// Save session for reuse
	if err := a.saveSession(session); err != nil {
		a.logger.Warn("failed to save session", "error", err)
		// Don't fail if we can't save, just continue with in-memory session
	}

	return session, nil
}

// generateJWT generates a JWT token signed with the private key
func (a Auth) generateJWT() (string, error) {
	// Read private key
	keyData, err := os.ReadFile(a.Config.PEMFile)
	if err != nil {
		return "", fmt.Errorf("reading PEM file: %w", err)
	}

	// Parse private key
	privateKey, err := a.parsePrivateKey(keyData)
	if err != nil {
		return "", fmt.Errorf("parsing private key: %w", err)
	}

	// Create JWT
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, Claims{
		AppID: a.Config.AppID,
		Sub:   a.Config.AppID,
		Aud:   "api.enablebanking.com",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtExpiryDuration)),
			Issuer:    a.Config.AppID,
		},
	})

	// Add kid (Key ID) header required by EnableBanking API
	token.Header["kid"] = a.Config.AppID

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return tokenString, nil
}

// parsePrivateKey parses a PEM-encoded private key
func (a Auth) parsePrivateKey(keyData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an RSA private key")
	}

	return rsaKey, nil
}

// initiateAuthorization initiates the authorization flow and returns the auth URL and ID
func (a Auth) initiateAuthorization(ctx context.Context, jwtToken string) (string, string, error) {
	// Generate a UUID for the state parameter
	stateUUID := uuid.New().String()

	// Create authorization request
	authReq := AuthorizationRequest{
		PSUType: "personal",
		State:   stateUUID,
		ASPSP: struct {
			Name    string `json:"name"`
			Country string `json:"country"`
		}{
			Name:    a.Config.ASPSP,
			Country: a.Config.Country,
		},
		RedirectURL: a.Config.RedirectURL,
	}

	// Set valid_until to N days from now
	validUntil := time.Now().UTC().AddDate(0, 0, accessRequestDays)
	authReq.Access.ValidUntil = validUntil.Format(time.RFC3339)

	// Marshal request body
	body, err := json.Marshal(authReq)
	if err != nil {
		return "", "", fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	url := enableBankingAPIBase + "/auth"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Check for specific errors
		if resp.StatusCode == http.StatusBadRequest && bytes.Contains(respBody, []byte("REDIRECT_URI_NOT_ALLOWED")) {
			return "", "", fmt.Errorf(
				"Redirect URI not allowed. The URL '%s' must be registered in your EnableBanking application settings",
				a.Config.RedirectURL,
			)
		}
		return "", "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var authResp AuthorizationResponse
	if err := json.Unmarshal(respBody, &authResp); err != nil {
		return "", "", fmt.Errorf("parsing response: %w", err)
	}

	return authResp.URL, stateUUID, nil
}

// displayAuthorizationInstructions displays the authorization URL and instructions to the user
func (a Auth) displayAuthorizationInstructions(authURL string) {
	separator := strings.Repeat("=", 60)
	a.logger.Info("authorization required", "aspsp", a.Config.ASPSP, "country", a.Config.Country)
	a.logger.Info(separator)
	a.logger.Info("please visit the following URL to authorize", "url", authURL)
	a.logger.Info("after authorizing you will be redirected to a URL that looks like")
	a.logger.Info("  " + a.Config.RedirectURL + "?code=<code>&state=<state>")
	a.logger.Info("copy that FULL redirect URL and paste it below")
	a.logger.Info(separator)
	a.logger.Info("running in Docker? attach with: docker attach <container-name>")
	a.logger.Info("  then paste the URL (Ctrl+V) and press Enter")
	a.logger.Info("  once done, detach without stopping with: Ctrl+P then Ctrl+Q")
	a.logger.Info(separator)
}

// promptForRedirectURL asks the operator to paste the full redirect URL after
// completing the authorization flow, then extracts and validates the code and
// state query parameters. Validating the state guards against a code from a
// different (or attacker-substituted) authorization session being accepted.
//
// When stdin returns EOF (e.g. a container started without an attached
// terminal), the function waits and retries — allowing the operator to attach
// to the running container and provide input rather than crashing immediately.
func (a Auth) promptForRedirectURL(expectedState string) (string, error) {
	fmt.Fprintln(os.Stderr, "Paste the full redirect URL here, then press Enter:")
	fmt.Fprint(os.Stderr, "> ")
	for {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// No terminal attached yet — wait and retry so the operator
				// can attach to the container (e.g. docker attach) and paste.
				time.Sleep(2 * time.Second)
				continue
			}
			return "", fmt.Errorf("reading from stdin: %w", err)
		}

		rawURL := strings.TrimSpace(line)
		if rawURL == "" {
			continue
		}

		return extractCodeFromRedirectURL(rawURL, expectedState)
	}
}

// extractCodeFromRedirectURL parses a redirect URL, validates the state
// parameter against expectedState, and returns the authorization code.
// Separating this from stdin I/O makes it straightforwardly testable.
func extractCodeFromRedirectURL(rawURL, expectedState string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing redirect URL: %w", err)
	}

	state := parsed.Query().Get("state")
	if state != expectedState {
		return "", fmt.Errorf("state mismatch: possible CSRF — expected %s, got %s", expectedState, state)
	}

	code := parsed.Query().Get("code")
	if code == "" {
		return "", errors.New("no code parameter found in redirect URL")
	}

	return code, nil
}

// createSessionWithCode exchanges the authorization code for a session
func (a Auth) createSessionWithCode(ctx context.Context, jwtToken, code string) (Session, error) {
	url := enableBankingAPIBase + "/sessions"

	// Create request body
	reqBody := SessionRequest{
		Code: code,
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return Session{}, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return Session{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return Session{}, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return Session{}, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return Session{}, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return Session{}, fmt.Errorf("parsing response: %w", err)
	}

	// Ensure CreatedAt is set; the API may not return it.
	if session.CreatedAt == "" {
		session.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	session.AuthToken = jwtToken

	if session.ValidUntil != "" {
		a.logger.Info("session created",
			"accounts", len(session.Accounts),
			"created_at", session.CreatedAt,
			"valid_until", session.ValidUntil,
		)
	} else {
		a.logger.Info("session created",
			"accounts", len(session.Accounts),
			"created_at", session.CreatedAt,
		)
	}

	return session, nil
}
