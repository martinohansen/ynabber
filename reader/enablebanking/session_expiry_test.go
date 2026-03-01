package enablebanking

import (
"context"
"encoding/json"
"errors"
"log/slog"
"os"
"testing"
"time"
)

// ---------------------------------------------------------------------------
// Session.IsExpired() — unit tests
// ---------------------------------------------------------------------------

func TestSessionIsExpired(t *testing.T) {
now := time.Now().UTC()

tests := []struct {
name       string
validUntil string
want       bool
}{
{
name:       "no valid_until — assume valid forever",
validUntil: "",
want:       false,
},
{
name:       "valid_until in the future — not expired",
validUntil: now.Add(24 * time.Hour).Format(time.RFC3339),
want:       false,
},
{
name:       "valid_until one second from now — not expired",
validUntil: now.Add(time.Second).Format(time.RFC3339),
want:       false,
},
{
name:       "valid_until one second ago — expired",
validUntil: now.Add(-time.Second).Format(time.RFC3339),
want:       true,
},
{
name:       "valid_until long in the past — expired",
validUntil: now.AddDate(-1, 0, 0).Format(time.RFC3339),
want:       true,
},
{
name:       "malformed valid_until — assume valid (fail open)",
validUntil: "not-a-date",
want:       false,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
s := Session{
CreatedAt:  now.Format(time.RFC3339),
ValidUntil: tt.validUntil,
}
got := s.IsExpired()
if got != tt.want {
t.Errorf("IsExpired() = %v, want %v (valid_until=%q)", got, tt.want, tt.validUntil)
}
})
}
}

// TestSessionIsExpiredNoValidUntilIgnoresCreatedAt confirms that sessions
// without a valid_until field are never considered locally expired, regardless
// of how old createdAt is. The API is the authority on expiry in this case.
func TestSessionIsExpiredNoValidUntilIgnoresCreatedAt(t *testing.T) {
old := Session{
CreatedAt: time.Now().UTC().AddDate(-1, 0, 0).Format(time.RFC3339),
// ValidUntil intentionally absent
}
if old.IsExpired() {
t.Error("IsExpired() = true for old CreatedAt without ValidUntil; want false (API is source of truth)")
}
}

// ---------------------------------------------------------------------------
// Auth.Session() with expired session on disk
// ---------------------------------------------------------------------------

func TestAuthSessionRejectsExpiredSessionFile(t *testing.T) {
past := time.Now().UTC().Add(-time.Second).Format(time.RFC3339)

staleSession := Session{
CreatedAt:  time.Now().UTC().AddDate(0, 0, -11).Format(time.RFC3339),
ValidUntil: past,
Accounts:   []AccountInfo{{UID: "acc-1", IBAN: randomTestIBAN(t)}},
}
data, err := json.Marshal(staleSession)
if err != nil {
t.Fatalf("marshaling session: %v", err)
}

f, err := os.CreateTemp(t.TempDir(), "session-*.json")
if err != nil {
t.Fatalf("creating temp session file: %v", err)
}
if _, err := f.Write(data); err != nil {
t.Fatalf("writing session file: %v", err)
}
f.Close()

auth := Auth{
Config: Config{
SessionFile: f.Name(),
// No PEMFile — if the expiry check is skipped and code falls
// through to generateJWT it will fail for the wrong reason.
},
logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
}

_, err = auth.Session(context.Background())
if err == nil {
t.Fatal("Auth.Session() returned nil error for expired session; expected ErrSessionExpired")
}
if !errors.Is(err, ErrSessionExpired) {
t.Errorf("Auth.Session() error = %q; want errors.Is(err, ErrSessionExpired)", err)
}
}

// TestAuthSessionPlaceholderFileInitiatesNewAuth verifies that a session file
// with an empty createdAt (the checked-in placeholder files like
// enablebanking_dnb_no_session.json) triggers a new authorization flow rather
// than returning a fatal ErrSessionExpired. The new auth flow will itself fail
// (no PEM key configured in the test), but crucially the error must NOT be
// ErrSessionExpired — that would cause the runner to exit permanently instead
// of prompting for re-authorization.
func TestAuthSessionPlaceholderFileInitiatesNewAuth(t *testing.T) {
tests := []struct {
name    string
session Session
}{
{
name: "placeholder file with empty createdAt and account UIDs",
session: Session{
CreatedAt: "",
Accounts:  []AccountInfo{{UID: "8cb9eed1-724f-44df-b7e4-81338c32ba63"}},
},
},
{
name: "completely empty session object",
session: Session{
CreatedAt: "",
Accounts:  nil,
},
},
{
name: "empty createdAt with multiple accounts",
session: Session{
CreatedAt: "",
Accounts: []AccountInfo{
{UID: "acc-1"},
{UID: "acc-2"},
},
},
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
data, err := json.Marshal(tt.session)
if err != nil {
t.Fatalf("marshaling session: %v", err)
}

f, err := os.CreateTemp(t.TempDir(), "session-*.json")
if err != nil {
t.Fatalf("creating temp session file: %v", err)
}
if _, err := f.Write(data); err != nil {
t.Fatalf("writing session file: %v", err)
}
f.Close()

auth := Auth{
Config: Config{
SessionFile: f.Name(),
// PEMFile intentionally empty — createNewSession will fail,
// but the error must not be ErrSessionExpired.
},
logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
}

_, err = auth.Session(context.Background())

// Must fail (no PEM configured), but must NOT be ErrSessionExpired —
// that error causes the runner to stop permanently.
if err == nil {
t.Fatal("expected an error (no PEM file configured), got nil")
}
if errors.Is(err, ErrSessionExpired) {
t.Errorf("placeholder file with empty createdAt must not return ErrSessionExpired; got: %v", err)
}
})
}
}

// TestAuthSessionAcceptsFreshSessionFile verifies that a valid, non-expired
// session (no valid_until — API did not return one) loads successfully.
func TestAuthSessionAcceptsFreshSessionFile(t *testing.T) {
freshSession := Session{
CreatedAt: time.Now().UTC().Format(time.RFC3339),
Accounts:  []AccountInfo{{UID: "acc-fresh", IBAN: randomTestIBAN(t)}},
// No ValidUntil — API did not return one; assume valid forever.
}
data, err := json.Marshal(freshSession)
if err != nil {
t.Fatalf("marshaling session: %v", err)
}

keyData := generateTestKeyPair(t)
pemFile, err := os.CreateTemp(t.TempDir(), "key-*.pem")
if err != nil {
t.Fatalf("creating temp PEM file: %v", err)
}
if _, err := pemFile.Write(keyData); err != nil {
t.Fatalf("writing PEM file: %v", err)
}
pemFile.Close()

sessionFile, err := os.CreateTemp(t.TempDir(), "session-*.json")
if err != nil {
t.Fatalf("creating temp session file: %v", err)
}
if _, err := sessionFile.Write(data); err != nil {
t.Fatalf("writing session file: %v", err)
}
sessionFile.Close()

auth := Auth{
Config: Config{
SessionFile: sessionFile.Name(),
PEMFile:     pemFile.Name(),
AppID:       "test-app-id",
},
logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
}

_, err = auth.Session(context.Background())

if errors.Is(err, ErrSessionExpired) {
t.Errorf("Auth.Session() returned ErrSessionExpired for a fresh session with no valid_until")
}
}
