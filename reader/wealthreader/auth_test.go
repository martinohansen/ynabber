package wealthreader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestExtractCodeFromRedirectURL(t *testing.T) {
	nonce := "abcnonce"
	state := "abcstate"
	raw := "https://example.com/oauth/success?nonce=abcnonce&code=the-code&state=abcstate"

	code, gotNonce, err := extractCodeFromRedirectURL(raw, nonce, state)
	if err != nil {
		t.Fatalf("extractCodeFromRedirectURL() error = %v", err)
	}
	if code != "the-code" {
		t.Errorf("code = %q, want the-code", code)
	}
	if gotNonce != nonce {
		t.Errorf("nonce = %q, want %s", gotNonce, nonce)
	}
}

func TestExtractCodeFromRedirectURLNonceMismatch(t *testing.T) {
	_, _, err := extractCodeFromRedirectURL(
		"https://example.com/ok?nonce=other&code=x",
		"expected",
		"",
	)
	if err == nil {
		t.Fatal("expected nonce mismatch error")
	}
}

func TestExtractCodeFromRedirectURLMissingCode(t *testing.T) {
	_, _, err := extractCodeFromRedirectURL(
		"https://example.com/ok?nonce=expected",
		"expected",
		"",
	)
	if err == nil {
		t.Fatal("expected missing code error")
	}
}

func TestGeneratePKCEPinsEntity(t *testing.T) {
	challenge, err := generatePKCE("bbva")
	if err != nil {
		t.Fatalf("generatePKCE() error = %v", err)
	}
	if len(challenge.Nonce) != 82 {
		t.Errorf("nonce length = %d, want 82 hex chars", len(challenge.Nonce))
	}
	if len(challenge.CodeVerifier) != 82 {
		t.Errorf("code_verifier length = %d, want 82 hex chars", len(challenge.CodeVerifier))
	}
	sum := sha256.Sum256([]byte(challenge.CodeVerifier))
	if challenge.Challenge != hex.EncodeToString(sum[:]) {
		t.Errorf("challenge_code is not SHA-256 of code_verifier")
	}

	raw, err := hex.DecodeString(challenge.WRConf)
	if err != nil {
		t.Fatalf("wr_conf is not hex: %v", err)
	}
	var conf map[string]any
	if err := json.Unmarshal(raw, &conf); err != nil {
		t.Fatalf("wr_conf json: %v", err)
	}
	entities, ok := conf["entities_to_display"].([]any)
	if !ok || len(entities) != 1 || entities[0] != "bbva" {
		t.Errorf("entities_to_display = %#v, want [bbva]", conf["entities_to_display"])
	}
	if conf["wait_full_response"] != true {
		t.Errorf("wait_full_response = %v, want true", conf["wait_full_response"])
	}
}

func TestAuthorizationURL(t *testing.T) {
	auth := Auth{Config: Config{
		OAuthBase:   "https://oauth.wealthreader.com",
		RedirectURL: "https://example.com/ok",
	}}
	challenge := pkceChallenge{
		Nonce:     "n",
		State:     "s",
		Challenge: "c",
		WRConf:    "7b7d",
	}
	got, err := auth.authorizationURL(challenge)
	if err != nil {
		t.Fatalf("authorizationURL() error = %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if !strings.HasPrefix(got, "https://oauth.wealthreader.com/oauth2/") {
		t.Errorf("url = %s, want oauth2 prefix", got)
	}
	q := parsed.Query()
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %s", q.Get("code_challenge_method"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type = %s", q.Get("response_type"))
	}
	if q.Get("redirect_uri") != "https://example.com/ok" {
		t.Errorf("redirect_uri = %s", q.Get("redirect_uri"))
	}
}
