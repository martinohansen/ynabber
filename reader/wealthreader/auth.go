package wealthreader

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	alnumLength          = 41
	alnumAlphabet        = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	maxResponseBodyBytes = 10 * 1024 * 1024
)

// ErrSessionExpired is returned when automatic reauthorization cannot continue.
var ErrSessionExpired = errors.New("session expired")

// Session is what we persist under WEALTHREADER_SESSION_FILE.
// Token + Code are the only credentials later POSTs to /entities/ need.
type Session struct {
	Token     string `json:"token"`
	Code      string `json:"code"`
	CreatedAt string `json:"created_at"`
}

// pkceChallenge is the one-shot OAuth material. Wealth Reader's PKCE is not
// standard RFC 7636: nonce/state/code_verifier are bin2hex(randomAlnum(41)),
// and challenge_code is SHA-256 hex of the already-hex code_verifier.
// See demo/oauth/index.php and docs/en/integracion-via-oauth-backend.md.
type pkceChallenge struct {
	Nonce        string
	State        string
	CodeVerifier string
	Challenge    string
	WRConf       string
}

// Auth handles OAuth and the session file.
type Auth struct {
	Config        Config
	httpClient    *http.Client
	redirectInput io.Reader
	logger        *slog.Logger
}

// NewAuth creates a new Auth handler.
func NewAuth(cfg Config, logger *slog.Logger) Auth {
	return Auth{
		Config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		redirectInput: os.Stdin,
		logger:        logger,
	}
}

// acquireSession returns a reusable token session. authorized is true when the
// session was created in this call (so Bulk should not treat a 401 as "retry
// with a brand new OAuth" more than once).
func (a Auth) acquireSession(ctx context.Context) (Session, bool, error) {
	session, err := a.loadSession()
	if err == nil && session.Token != "" {
		return session, false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Session{}, false, err
	}

	session, err = a.createNewSession(ctx)
	if err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func (a Auth) loadSession() (Session, error) {
	data, err := os.ReadFile(a.Config.SessionFile)
	if err != nil {
		return Session{}, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("parsing session file: %w", err)
	}
	if session.Token == "" {
		return Session{}, os.ErrNotExist
	}
	if session.Code == "" {
		session.Code = a.Config.Code
	}
	return session, nil
}

func (a Auth) saveSession(session Session) error {
	if session.CreatedAt == "" {
		session.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding session: %w", err)
	}
	if err := os.WriteFile(a.Config.SessionFile, data, 0o600); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}
	a.logger.Info("session saved to disk", "path", a.Config.SessionFile)
	return nil
}

func (a Auth) invalidateSession() error {
	if err := os.Remove(a.Config.SessionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing session file: %w", err)
	}
	return nil
}

func (a Auth) reauthorize(ctx context.Context) (Session, error) {
	if err := a.invalidateSession(); err != nil {
		return Session{}, err
	}
	return a.createNewSession(ctx)
}

func (a Auth) createNewSession(ctx context.Context) (Session, error) {
	challenge, err := generatePKCE(a.Config.Code)
	if err != nil {
		return Session{}, err
	}

	authURL, err := a.authorizationURL(challenge)
	if err != nil {
		return Session{}, err
	}

	fmt.Fprintf(os.Stderr, "Open this URL to connect the bank via Wealth Reader:\n%s\n\n", authURL)

	code, nonce, err := a.promptForRedirectURL(ctx, challenge.Nonce, challenge.State)
	if err != nil {
		return Session{}, err
	}
	if nonce != challenge.Nonce {
		return Session{}, fmt.Errorf("nonce mismatch: possible CSRF")
	}

	resp, err := a.exchangeCode(ctx, challenge, code)
	if err != nil {
		return Session{}, err
	}

	token := resp.Statistics.Token
	if token == "" {
		return Session{}, fmt.Errorf("token exchange returned no statistics.token — register the domain with tokenize=1")
	}
	codeValue := resp.Statistics.Code
	if codeValue == "" {
		codeValue = a.Config.Code
	}

	session := Session{
		Token:     token,
		Code:      codeValue,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := a.saveSession(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (a Auth) authorizationURL(challenge pkceChallenge) (string, error) {
	u, err := url.Parse(a.Config.OAuthBase + "/oauth2/")
	if err != nil {
		return "", fmt.Errorf("parsing oauth base: %w", err)
	}
	q := u.Query()
	q.Set("challenge_code", challenge.Challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("redirect_uri", a.Config.RedirectURL)
	q.Set("response_type", "code")
	q.Set("state", challenge.State)
	q.Set("nonce", challenge.Nonce)
	q.Set("wr_conf", challenge.WRConf)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (a Auth) exchangeCode(ctx context.Context, challenge pkceChallenge, code string) (Response, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", a.Config.RedirectURL)
	form.Set("code", code)
	form.Set("code_verifier", challenge.CodeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Config.OAuthBase+"/token/", strings.NewReader(form.Encode()))
	if err != nil {
		return Response{}, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("sending token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return Response{}, fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	parsed, err := parseResponse(body)
	if err != nil {
		return Response{}, err
	}
	return parsed, nil
}

func generatePKCE(entityCode string) (pkceChallenge, error) {
	nonceAlnum, err := randomAlnum(alnumLength)
	if err != nil {
		return pkceChallenge{}, err
	}
	stateAlnum, err := randomAlnum(alnumLength)
	if err != nil {
		return pkceChallenge{}, err
	}
	verifierAlnum, err := randomAlnum(alnumLength)
	if err != nil {
		return pkceChallenge{}, err
	}

	nonce := hex.EncodeToString([]byte(nonceAlnum))
	state := hex.EncodeToString([]byte(stateAlnum))
	codeVerifier := hex.EncodeToString([]byte(verifierAlnum))
	sum := sha256.Sum256([]byte(codeVerifier))

	entities := []string{}
	if entityCode != "" {
		entities = []string{entityCode}
	}
	wrConfJSON, err := json.Marshal(map[string]any{
		"operation_id":        randomOperationID(),
		"entities_to_display": entities,
		"wait_full_response":  true,
	})
	if err != nil {
		return pkceChallenge{}, err
	}

	return pkceChallenge{
		Nonce:        nonce,
		State:        state,
		CodeVerifier: codeVerifier,
		Challenge:    hex.EncodeToString(sum[:]),
		WRConf:       hex.EncodeToString(wrConfJSON),
	}, nil
}

func randomAlnum(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = alnumAlphabet[int(b)%len(alnumAlphabet)]
	}
	return string(out), nil
}

func randomOperationID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("op_%d", time.Now().UnixNano())
	}
	return "op_" + hex.EncodeToString(buf)
}

type redirectLine struct {
	line string
	err  error
}

func readRedirectLine(ctx context.Context, reader *bufio.Reader) (string, error) {
	result := make(chan redirectLine, 1)
	go func() {
		line, err := reader.ReadString('\n')
		result <- redirectLine{line: line, err: err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case read := <-result:
		return read.line, read.err
	}
}

// promptForRedirectURL asks the operator to paste the full redirect URL.
// Wealth Reader returns nonce + code (docs); state is accepted when present.
func (a Auth) promptForRedirectURL(ctx context.Context, expectedNonce, expectedState string) (code, nonce string, err error) {
	input := a.redirectInput
	if input == nil {
		input = os.Stdin
	}
	reader := bufio.NewReader(input)

	fmt.Fprint(os.Stderr, "Paste the full redirect URL, then press Enter:\n> ")
	for {
		line, err := readRedirectLine(ctx, reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				timer := time.NewTimer(2 * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return "", "", ctx.Err()
				case <-timer.C:
					continue
				}
			}
			return "", "", fmt.Errorf("reading from stdin: %w", err)
		}

		rawURL := strings.TrimSpace(line)
		if rawURL == "" {
			continue
		}
		return extractCodeFromRedirectURL(rawURL, expectedNonce, expectedState)
	}
}

func extractCodeFromRedirectURL(rawURL, expectedNonce, expectedState string) (code, nonce string, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing redirect URL: %w", err)
	}

	nonce = parsed.Query().Get("nonce")
	if nonce == "" {
		return "", "", errors.New("no nonce parameter found in redirect URL")
	}
	if nonce != expectedNonce {
		return "", "", fmt.Errorf("nonce mismatch: possible CSRF")
	}

	if state := parsed.Query().Get("state"); state != "" && expectedState != "" && state != expectedState {
		return "", "", fmt.Errorf("state mismatch: possible CSRF")
	}

	code = parsed.Query().Get("code")
	if code == "" {
		return "", "", errors.New("no code parameter found in redirect URL")
	}
	return code, nonce, nil
}
