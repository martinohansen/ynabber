package enablebanking

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// generateTestKeyPair generates a test RSA key pair and returns PEM-encoded key bytes
func generateTestKeyPair(t *testing.T) []byte {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	// Marshal private key to PKCS8
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}

	// Encode to PEM
	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	pemBytes := pem.EncodeToMemory(block)
	return pemBytes
}

// TestParsePrivateKey tests parsing a PEM-encoded private key
func TestParsePrivateKey(t *testing.T) {
	auth := Auth{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	// Generate test key
	keyData := generateTestKeyPair(t)

	// Parse the key
	privateKey, err := auth.parsePrivateKey(keyData)
	if err != nil {
		t.Fatalf("parsePrivateKey failed: %v", err)
	}

	if privateKey == nil {
		t.Fatal("parsePrivateKey returned nil private key")
	}
}

// TestParsePrivateKeyInvalidPEM tests parsing invalid PEM data
func TestParsePrivateKeyInvalidPEM(t *testing.T) {
	auth := Auth{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	// Try to parse invalid PEM
	_, err := auth.parsePrivateKey([]byte("not a valid pem"))
	if err == nil {
		t.Fatal("parsePrivateKey should have failed with invalid PEM")
	}
}

// TestGenerateJWT tests JWT token generation
func TestGenerateJWT(t *testing.T) {
	// Create temporary PEM file
	keyData := generateTestKeyPair(t)
	tmpFile, err := os.CreateTemp("", "test-key-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write(keyData)
	if err != nil {
		t.Fatalf("failed to write test key: %v", err)
	}
	tmpFile.Close()

	// Create Auth with config
	auth := Auth{
		Config: Config{
			AppID:   "test-app-id",
			PEMFile: tmpFile.Name(),
		},
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	token, err := auth.generateJWT()
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	if token == "" {
		t.Fatal("generateJWT returned empty token")
	}
}

// TestSaveAndLoadSession tests saving and loading a session
func TestSaveAndLoadSession(t *testing.T) {
	// Create temporary PEM file
	keyData := generateTestKeyPair(t)
	keyFile, err := os.CreateTemp("", "test-key-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp key file: %v", err)
	}
	defer os.Remove(keyFile.Name())
	if _, err := keyFile.Write(keyData); err != nil {
		keyFile.Close()
		t.Fatalf("failed to write test key: %v", err)
	}
	keyFile.Close()

	// Create temporary session file
	tmpFile, err := os.CreateTemp("", "test-session-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create Auth with temp session file
	auth := Auth{
		Config: Config{
			AppID:       "test-app-id",
			PEMFile:     keyFile.Name(),
			SessionFile: tmpFile.Name(),
		},
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	// Create a test session
	iban := randomTestIBAN(t)
	testSession := Session{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Accounts: []AccountInfo{
			{
				UID:         "test-uid-1",
				AccountID:   AccountID{IBAN: iban},
				DisplayName: "Test Account 1",
				Currency:    "NOK",
			},
		},
	}

	// Save session
	err = auth.saveSession(testSession)
	if err != nil {
		t.Fatalf("saveSession failed: %v", err)
	}

	// Load session back
	loadedSession, err := auth.Session(context.Background())
	if err != nil {
		t.Fatalf("Session() failed: %v", err)
	}

	// Verify
	if len(loadedSession.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(loadedSession.Accounts))
	}

	if loadedSession.Accounts[0].UID != "test-uid-1" {
		t.Fatalf("expected UID 'test-uid-1', got '%s'", loadedSession.Accounts[0].UID)
	}
}

// TestClaims tests JWT claims structure
func TestClaims(t *testing.T) {
	now := time.Now()
	claims := Claims{
		AppID: "test-app",
		Sub:   "test-app",
		Aud:   "api.enablebanking.com",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			Issuer:    "test-app",
		},
	}

	if claims.AppID != "test-app" {
		t.Fatalf("expected AppID 'test-app', got '%s'", claims.AppID)
	}

	if claims.Aud != "api.enablebanking.com" {
		t.Fatalf("expected Aud 'api.enablebanking.com', got '%s'", claims.Aud)
	}
}

// TestAccountInfo tests AccountInfo structure
func TestAccountInfo(t *testing.T) {
	iban := randomTestIBAN(t)
	account := AccountInfo{
		UID:         "test-uid",
		AccountID:   AccountID{IBAN: iban},
		DisplayName: "Test Account",
		Currency:    "NOK",
		Status:      "active",
	}

	if account.UID != "test-uid" {
		t.Fatalf("expected UID 'test-uid', got '%s'", account.UID)
	}

	if account.AccountID.IBAN != iban {
		t.Fatalf("expected IBAN '%s', got '%s'", iban, account.AccountID.IBAN)
	}
}

// TestAuthNewAuth tests creating a new Auth instance
func TestAuthNewAuth(t *testing.T) {
	cfg := Config{
		AppID:   "test-app",
		Country: "NO",
		ASPSP:   "DNB",
	}

	auth := NewAuth(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if auth.Config.AppID != "test-app" {
		t.Fatalf("expected AppID 'test-app', got '%s'", auth.Config.AppID)
	}

	// httpClient must be initialised — a nil client would panic on the first
	// HTTP call.
	if auth.httpClient == nil {
		t.Fatal("NewAuth() returned Auth with nil httpClient")
	}

	// Confirm the timeout is the expected production value (10 s).
	if auth.httpClient.Timeout != 10*time.Second {
		t.Errorf("httpClient.Timeout = %v, want 10s", auth.httpClient.Timeout)
	}
}

// TestExtractCodeFromRedirectURL tests the state-validation and code-extraction
// logic that guards the OAuth authorization flow against CSRF.
func TestExtractCodeFromRedirectURL(t *testing.T) {
	const validState = "expected-state-uuid"
	const validCode = "auth-code-abc123"

	tests := []struct {
		name          string
		rawURL        string
		expectedState string
		wantCode      string
		wantErr       string
	}{
		{
			name:          "valid URL with matching state",
			rawURL:        "https://example.com/redirect?code=" + validCode + "&state=" + validState,
			expectedState: validState,
			wantCode:      validCode,
		},
		{
			name:          "state mismatch — CSRF attempt",
			rawURL:        "https://example.com/redirect?code=" + validCode + "&state=attacker-state",
			expectedState: validState,
			wantErr:       "state mismatch",
		},
		{
			name:          "missing state parameter",
			rawURL:        "https://example.com/redirect?code=" + validCode,
			expectedState: validState,
			wantErr:       "state mismatch",
		},
		{
			name:          "missing code parameter",
			rawURL:        "https://example.com/redirect?state=" + validState,
			expectedState: validState,
			wantErr:       "no code parameter",
		},
		{
			name:          "both parameters missing",
			rawURL:        "https://example.com/redirect",
			expectedState: validState,
			wantErr:       "state mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractCodeFromRedirectURL(tt.rawURL, tt.expectedState)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantCode {
				t.Errorf("code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

// TestGetMaxConsentDuration is a table-driven test for the (not-yet-implemented)
// getMaxConsentDuration method.  Each sub-test spins up an httptest.NewServer
// that stands in for the EnableBanking /aspsps endpoint and verifies that the
// method returns the correct time.Duration — or falls back to the hardcoded
// accessRequestDays when the API fails, the ASPSP is missing, or the value is 0.
//
// NOTE: This test intentionally references Auth.baseURL and Auth.getMaxConsentDuration,
// which do not exist yet.  The file will NOT COMPILE until the implementation
// is added — that is the expected Red state.
func TestGetMaxConsentDuration(t *testing.T) {
	tests := []struct {
		name         string
		mockBody     string
		mockStatus   int
		wantDuration time.Duration
	}{
		{
			name:         "ASPSP found — uses maximum",
			mockBody:     `{"aspsps":[{"name":"DNB","country":"NO","maximum_consent_validity":15552000}]}`,
			mockStatus:   http.StatusOK,
			wantDuration: 15552000 * time.Second, // 180 days
		},
		{
			name:         "ASPSP not in list — fallback",
			mockBody:     `{"aspsps":[{"name":"OtherBank","country":"NO","maximum_consent_validity":7776000}]}`,
			mockStatus:   http.StatusOK,
			wantDuration: accessRequestDays * 24 * time.Hour,
		},
		{
			name:         "API error — fallback",
			mockBody:     `internal server error`,
			mockStatus:   http.StatusInternalServerError,
			wantDuration: accessRequestDays * 24 * time.Hour,
		},
		{
			name:         "Zero value — fallback",
			mockBody:     `{"aspsps":[{"name":"DNB","country":"NO","maximum_consent_validity":0}]}`,
			mockStatus:   http.StatusOK,
			wantDuration: accessRequestDays * 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}
				if !strings.HasPrefix(r.URL.Path, "/aspsps") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if got := r.URL.Query().Get("country"); got != "NO" {
					t.Errorf("country query param = %q, want %q", got, "NO")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.mockStatus)
				fmt.Fprint(w, tt.mockBody)
			}))
			defer server.Close()

			// Auth.baseURL does not exist yet — the field will be added as
			// part of the implementation.  Referencing it here is deliberate:
			// the test must not compile before the fix is in place.
			a := Auth{
				Config: Config{
					ASPSP:   "DNB",
					Country: "NO",
				},
				baseURL:    server.URL,
				httpClient: server.Client(),
				logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			}

			// getMaxConsentDuration does not exist yet — referencing it here
			// keeps the test in the Red state until it is implemented.
			got := a.getMaxConsentDuration(context.Background(), "test-token")
			if got != tt.wantDuration {
				t.Errorf("getMaxConsentDuration() = %v, want %v", got, tt.wantDuration)
			}
		})
	}
}

// TestCreateSessionWithCodeParsesValidUntilFromAccess verifies that
// createSessionWithCode correctly extracts valid_until from the nested
// access object in the EnableBanking POST /sessions response.
//
// The EnableBanking API wraps valid_until inside an "access" object:
//
//	{
//	  "createdAt": "...",
//	  "accounts": [...],
//	  "access": {
//	    "valid_until": "2026-08-29T17:30:27Z",
//	    ...
//	  }
//	}
//
// The current implementation unmarshals directly into Session{}, whose
// ValidUntil field carries the json tag "valid_until" at the top level, so
// session.ValidUntil is always empty after parsing.
//
// This test MUST FAIL before the fix (session.ValidUntil will be "").
// It MUST PASS once createSessionWithCode uses an intermediate struct that
// reads access.valid_until and copies it into session.ValidUntil.
//
// NOTE: The test also sets baseURL: server.URL because createSessionWithCode
// currently hardcodes enableBankingAPIBase instead of honouring a.baseURL.
// Fixing the URL is a prerequisite for the test to reach the assertion.
func TestCreateSessionWithCodeParsesValidUntilFromAccess(t *testing.T) {
	const (
		wantValidUntil = "2026-08-29T17:30:27Z"
		wantCreatedAt  = "2026-03-02T10:00:00Z"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"createdAt": "2026-03-02T10:00:00Z",
			"accounts": [],
			"access": {
				"valid_until": "2026-08-29T17:30:27Z",
				"balances": true,
				"transactions": true
			}
		}`)
	}))
	defer server.Close()

	// Build a temp PEM file, mirroring TestGenerateJWT.
	keyData := generateTestKeyPair(t)
	tmpKey, err := os.CreateTemp("", "test-key-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp key file: %v", err)
	}
	defer os.Remove(tmpKey.Name())
	if _, err := tmpKey.Write(keyData); err != nil {
		tmpKey.Close()
		t.Fatalf("failed to write test key: %v", err)
	}
	tmpKey.Close()

	a := Auth{
		Config: Config{
			AppID:   "test-app-id",
			PEMFile: tmpKey.Name(),
		},
		baseURL:    server.URL,
		httpClient: server.Client(),
		logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	session, err := a.createSessionWithCode(context.Background(), "test-jwt", "test-code")
	if err != nil {
		t.Fatalf("createSessionWithCode returned unexpected error: %v", err)
	}

	// Core assertion: valid_until must be populated from access.valid_until.
	// Before the fix this will be "" — the test fails here.
	if session.ValidUntil != wantValidUntil {
		t.Errorf("session.ValidUntil = %q, want %q\n"+
			"  This means valid_until is still read from the top-level JSON field\n"+
			"  instead of the nested access.valid_until field returned by the API.",
			session.ValidUntil, wantValidUntil)
	}
}

// TestCreateSessionWithCodeMissingAccess verifies that createSessionWithCode
// handles a POST /sessions response that contains no "access" field without
// panicking or returning an error. session.ValidUntil must be "" in this case.
//
// Like TestCreateSessionWithCodeParsesValidUntilFromAccess, this test currently
// fails because createSessionWithCode hardcodes enableBankingAPIBase instead of
// honouring a.baseURL — the mock server is never reached.  Once the URL routing
// is fixed (a prerequisite shared with the other test), this test must PASS and
// must continue to PASS after the ValidUntil parsing fix is applied.  It guards
// against regressions where an absent "access" field causes a panic or a
// spurious non-empty ValidUntil.
func TestCreateSessionWithCodeMissingAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Deliberately omit the "access" object to test graceful handling.
		fmt.Fprint(w, `{
			"createdAt": "2026-03-02T10:00:00Z",
			"accounts": []
		}`)
	}))
	defer server.Close()

	keyData := generateTestKeyPair(t)
	tmpKey, err := os.CreateTemp("", "test-key-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp key file: %v", err)
	}
	defer os.Remove(tmpKey.Name())
	if _, err := tmpKey.Write(keyData); err != nil {
		tmpKey.Close()
		t.Fatalf("failed to write test key: %v", err)
	}
	tmpKey.Close()

	a := Auth{
		Config: Config{
			AppID:   "test-app-id",
			PEMFile: tmpKey.Name(),
		},
		baseURL:    server.URL,
		httpClient: server.Client(),
		logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	session, err := a.createSessionWithCode(context.Background(), "test-jwt", "test-code")
	if err != nil {
		t.Fatalf("createSessionWithCode returned unexpected error: %v", err)
	}

	// When "access" is absent the field must be empty — never a stale or
	// default value.
	if session.ValidUntil != "" {
		t.Errorf("session.ValidUntil = %q, want empty string when access object is absent",
			session.ValidUntil)
	}
}

// TestInitiateAuthorizationUsesMaxConsentValidity verifies the end-to-end fix:
// initiateAuthorization must call getMaxConsentDuration and use the bank's
// maximum_consent_validity (180 days) when building the valid_until field,
// rather than the hardcoded accessRequestDays (10 days) fallback.
//
// The test registers two routes on a single httptest.NewServer:
//
// GET  /aspsps → returns DNB with maximum_consent_validity = 15 552 000 s (180 d)
// POST /auth   → captures the request body and returns a minimal success response
//
// After the call it asserts that valid_until is approximately 180 days from now
// (window: [now+179d, now+181d]).
//
// NOTE: Like TestGetMaxConsentDuration, this test references Auth.baseURL which
// does not exist yet and will NOT COMPILE until the implementation is added.
func TestInitiateAuthorizationUsesMaxConsentValidity(t *testing.T) {
	var (
		mu           sync.Mutex
		capturedBody []byte
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/aspsps"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"aspsps":[{"name":"DNB","country":"NO","maximum_consent_validity":15552000}]}`)

		case r.Method == http.MethodPost && r.URL.Path == "/auth":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("reading /auth request body: %v", err)
			}
			mu.Lock()
			capturedBody = body
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"url":"https://bank.example/auth","id":"session-1"}`)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Auth.baseURL does not exist yet — see note in TestGetMaxConsentDuration.
	a := Auth{
		Config: Config{
			ASPSP:   "DNB",
			Country: "NO",
		},
		baseURL:    server.URL,
		httpClient: server.Client(),
		logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	now := time.Now().UTC()

	_, _, err := a.initiateAuthorization(context.Background(), "test-jwt-token")
	if err != nil {
		t.Fatalf("initiateAuthorization returned unexpected error: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	if len(body) == 0 {
		t.Fatal("no request body was captured from POST /auth — did initiateAuthorization send a request?")
	}

	var authReq AuthorizationRequest
	if err := json.Unmarshal(body, &authReq); err != nil {
		t.Fatalf("parsing captured POST /auth body: %v", err)
	}

	if authReq.Access.ValidUntil == "" {
		t.Fatal("valid_until is empty in the captured authorization request")
	}

	validUntil, err := time.Parse(time.RFC3339, authReq.Access.ValidUntil)
	if err != nil {
		t.Fatalf("parsing valid_until %q: %v", authReq.Access.ValidUntil, err)
	}

	low := now.AddDate(0, 0, 179)
	high := now.AddDate(0, 0, 181)

	if validUntil.Before(low) || validUntil.After(high) {
		t.Errorf(
			"valid_until = %s; want approximately 180 days from now.\n"+
				"  acceptable window: [%s, %s]\n"+
				"  This likely means the hardcoded accessRequestDays (%d days) is still\n"+
				"  being used instead of the bank's maximum_consent_validity (180 days).",
			validUntil.Format(time.RFC3339),
			low.Format(time.RFC3339),
			high.Format(time.RFC3339),
			accessRequestDays,
		)
	}
}

// TestAccountInfoStableID verifies that StableID() returns the most stable
// identifier for an account, in priority order: IBAN → BBAN/CPAN → UID.
//
// NOTE: This test references AccountID, AccountIDOther, and StableID() which
// do not exist yet. It will NOT COMPILE until the implementation is added —
// that is the expected Red state.
func TestAccountInfoStableID(t *testing.T) {
	tests := []struct {
		name    string
		account AccountInfo
		wantID  string
	}{
		{
			name: "IBAN present — returns IBAN",
			account: AccountInfo{
				UID: "uid-1",
				AccountID: AccountID{
					IBAN: "NO9812345678901",
					Other: AccountIDOther{
						Identification: "12345678901",
						SchemeName:     "BBAN",
					},
				},
			},
			wantID: "NO9812345678901",
		},
		{
			name: "No IBAN, BBAN present — returns BBAN",
			account: AccountInfo{
				UID: "uid-2",
				AccountID: AccountID{
					Other: AccountIDOther{
						Identification: "12345678901",
						SchemeName:     "BBAN",
					},
				},
			},
			wantID: "12345678901",
		},
		{
			name: "No IBAN, CPAN present — returns masked CPAN",
			account: AccountInfo{
				UID: "uid-3",
				AccountID: AccountID{
					Other: AccountIDOther{
						Identification: "540111******9999",
						SchemeName:     "CPAN",
					},
				},
			},
			wantID: "540111******9999",
		},
		{
			name: "No IBAN, unknown scheme — returns identification",
			account: AccountInfo{
				UID: "uid-4",
				AccountID: AccountID{
					Other: AccountIDOther{
						Identification: "XYZABC",
						SchemeName:     "PROPRIETARY",
					},
				},
			},
			wantID: "XYZABC",
		},
		{
			name:    "No identifiers at all — falls back to UID",
			account: AccountInfo{UID: "uid-5"},
			wantID:  "uid-5",
		},
		{
			name: "IBAN exceeds ISO max length — falls back to Other.Identification",
			account: AccountInfo{
				UID: "uid-6",
				AccountID: AccountID{
					// 35 chars — one over the ISO 13616 maximum of 34
					IBAN: "NO981234567890123456789012345678901",
					Other: AccountIDOther{
						Identification: "12345678901",
						SchemeName:     "BBAN",
					},
				},
			},
			wantID: "12345678901",
		},
		{
			name: "Identification exceeds max length — falls back to UID",
			account: AccountInfo{
				UID: "uid-7",
				AccountID: AccountID{
					Other: AccountIDOther{
						// 65 chars — one over the practical maximum of 64
						Identification: "1234567890123456789012345678901234567890123456789012345678901234X",
						SchemeName:     "BBAN",
					},
				},
			},
			wantID: "uid-7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.account.StableID()
			if got != tt.wantID {
				t.Errorf("StableID() = %q, want %q", got, tt.wantID)
			}
		})
	}
}

// TestAccountInfoParsesAccountIDFromSession verifies that AccountInfo correctly
// parses the nested account_id structure returned by the /sessions endpoint.
//
// The API returns stable identifiers nested inside account_id:
//
//	{ "uid": "...", "account_id": { "iban": "...", "other": { "identification": "...", "scheme_name": "BBAN" } } }
//
// The current flat json tags (json:"iban" at top level) silently discard this
// data. This test must FAIL before the fix and PASS after.
//
// NOTE: This test references AccountID.IBAN and AccountID.Other.Identification
// which do not exist yet. It will NOT COMPILE until the struct is added.
func TestAccountInfoParsesAccountIDFromSession(t *testing.T) {
	tests := []struct {
		name               string
		jsonInput          string
		wantIBAN           string
		wantIdentification string
		wantSchemeName     string
	}{
		{
			name:               "standard bank account — IBAN and BBAN",
			jsonInput:          `{"uid":"u1","account_id":{"iban":"NO9812345678901","other":{"identification":"12345678901","scheme_name":"BBAN"}}}`,
			wantIBAN:           "NO9812345678901",
			wantIdentification: "12345678901",
			wantSchemeName:     "BBAN",
		},
		{
			name:               "credit card — null IBAN, masked CPAN",
			jsonInput:          `{"uid":"u2","account_id":{"iban":null,"other":{"identification":"540111******9999","scheme_name":"CPAN"}}}`,
			wantIBAN:           "",
			wantIdentification: "540111******9999",
			wantSchemeName:     "CPAN",
		},
		{
			name:               "missing account_id — all fields empty",
			jsonInput:          `{"uid":"u3"}`,
			wantIBAN:           "",
			wantIdentification: "",
			wantSchemeName:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var account AccountInfo
			if err := json.Unmarshal([]byte(tt.jsonInput), &account); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			if account.AccountID.IBAN != tt.wantIBAN {
				t.Errorf("AccountID.IBAN = %q, want %q\n"+
					"  AccountInfo likely still uses flat json:\"iban\" instead of nested account_id.",
					account.AccountID.IBAN, tt.wantIBAN)
			}
			if account.AccountID.Other.Identification != tt.wantIdentification {
				t.Errorf("AccountID.Other.Identification = %q, want %q",
					account.AccountID.Other.Identification, tt.wantIdentification)
			}
			if account.AccountID.Other.SchemeName != tt.wantSchemeName {
				t.Errorf("AccountID.Other.SchemeName = %q, want %q",
					account.AccountID.Other.SchemeName, tt.wantSchemeName)
			}
		})
	}
}
