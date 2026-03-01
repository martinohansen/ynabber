package enablebanking

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"os"
	"strings"
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
				IBAN:        iban,
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
		IBAN:        iban,
		DisplayName: "Test Account",
		Currency:    "NOK",
		Status:      "active",
	}

	if account.UID != "test-uid" {
		t.Fatalf("expected UID 'test-uid', got '%s'", account.UID)
	}

	if account.IBAN != iban {
		t.Fatalf("expected IBAN '%s', got '%s'", iban, account.IBAN)
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
