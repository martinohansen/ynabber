package web_test

import (
. "github.com/martinohansen/ynabber/internal/web"
"context"
"fmt"
"io"
"net"
"net/http"
"strings"
"testing"
"time"
)

// ---------------------------------------------------------------------------
// Minimal test doubles — implement only the sub-interfaces needed per test.
// ---------------------------------------------------------------------------

type stubComponent struct{ name string }

func (s stubComponent) Name() string { return s.name }

type stubStatusProvider struct {
stubComponent
status ComponentStatus
}

func (s stubStatusProvider) Status() ComponentStatus { return s.status }

type stubDetailHandler struct {
stubComponent
body string
}

func (s stubDetailHandler) ServeDetail(w http.ResponseWriter, r *http.Request) {
fmt.Fprint(w, s.body)
}

type stubInputHandler struct {
stubComponent
inputRequired bool
body          string
}

func (s stubInputHandler) ServeInput(w http.ResponseWriter, r *http.Request) {
fmt.Fprint(w, s.body)
}

func (s stubInputHandler) InputRequired() bool { return s.inputRequired }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// startServer creates, starts, and returns a Server on an OS-assigned port.
// The returned cancel func shuts the server down.
func startServer(t *testing.T) (*Server, int, context.CancelFunc) {
t.Helper()
ctx, cancel := context.WithCancel(context.Background())
srv := NewServer(0, "", nil) // port 0 = OS assigns
if err := srv.Start(ctx); err != nil {
cancel()
t.Fatalf("Start() error: %v", err)
}
return srv, srv.Port(), cancel
}

func get(t *testing.T, url string) *http.Response {
t.Helper()
resp, err := http.Get(url)
if err != nil {
t.Fatalf("GET %s: %v", url, err)
}
return resp
}

func body(t *testing.T, resp *http.Response) string {
t.Helper()
defer resp.Body.Close()
b, err := io.ReadAll(resp.Body)
if err != nil {
t.Fatalf("reading body: %v", err)
}
return string(b)
}

// ---------------------------------------------------------------------------
// TestServerHealth — GET /health always returns 200
// ---------------------------------------------------------------------------

func TestServerHealth(t *testing.T) {
_, port, cancel := startServer(t)
defer cancel()

resp := get(t, fmt.Sprintf("http://127.0.0.1:%d/health", port))
if resp.StatusCode != http.StatusOK {
t.Errorf("GET /health = %d, want 200", resp.StatusCode)
}
}

// ---------------------------------------------------------------------------
// TestServerRegister — components registered before and after Start appear
// ---------------------------------------------------------------------------

func TestServerRegister(t *testing.T) {
tests := []struct {
name          string
registerBefore []Component
registerAfter  []Component
wantInIndex   []string
}{
{
name:        "no components — index renders without error",
wantInIndex: nil,
},
{
name: "component registered before Start appears in index",
registerBefore: []Component{
stubStatusProvider{stubComponent{"my-reader"}, ComponentStatus{Healthy: true, Summary: "all good"}},
},
wantInIndex: []string{"my-reader", "all good"},
},
{
name: "component registered after Start appears in index",
registerAfter: []Component{
stubStatusProvider{stubComponent{"late-writer"}, ComponentStatus{Healthy: false, Summary: "degraded"}},
},
wantInIndex: []string{"late-writer", "degraded"},
},
{
name: "multiple components all appear",
registerBefore: []Component{
stubStatusProvider{stubComponent{"reader-a"}, ComponentStatus{Healthy: true, Summary: "ok"}},
stubStatusProvider{stubComponent{"reader-b"}, ComponentStatus{Healthy: true, Summary: "ok too"}},
},
wantInIndex: []string{"reader-a", "reader-b"},
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

srv := NewServer(0, "", nil)
for _, c := range tt.registerBefore {
srv.Register(c)
}
if err := srv.Start(ctx); err != nil {
t.Fatalf("Start() error: %v", err)
}
for _, c := range tt.registerAfter {
srv.Register(c)
time.Sleep(10 * time.Millisecond) // allow re-registration
}

resp := get(t, fmt.Sprintf("http://127.0.0.1:%d/", srv.Port()))
b := body(t, resp)

if resp.StatusCode != http.StatusOK {
t.Errorf("GET / = %d, want 200", resp.StatusCode)
}
for _, want := range tt.wantInIndex {
if !strings.Contains(b, want) {
t.Errorf("index body missing %q", want)
}
}
})
}
}

// ---------------------------------------------------------------------------
// TestServerDetailRoute
// ---------------------------------------------------------------------------

func TestServerDetailRoute(t *testing.T) {
tests := []struct {
name       string
component  Component
path       string
wantStatus int
wantBody   string
}{
{
name:       "registered DetailHandler is served",
component:  stubDetailHandler{stubComponent{"my-reader"}, "detail page content"},
path:       "/detail/my-reader",
wantStatus: http.StatusOK,
wantBody:   "detail page content",
},
{
name:       "unknown component name returns 404",
component:  stubDetailHandler{stubComponent{"my-reader"}, "detail page content"},
path:       "/detail/no-such-reader",
wantStatus: http.StatusNotFound,
},
{
name:       "component without DetailHandler returns 404",
component:  stubStatusProvider{stubComponent{"status-only"}, ComponentStatus{}},
path:       "/detail/status-only",
wantStatus: http.StatusNotFound,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

srv := NewServer(0, "", nil)
srv.Register(tt.component)
if err := srv.Start(ctx); err != nil {
t.Fatalf("Start(): %v", err)
}

resp := get(t, fmt.Sprintf("http://127.0.0.1:%d%s", srv.Port(), tt.path))
if resp.StatusCode != tt.wantStatus {
t.Errorf("GET %s = %d, want %d", tt.path, resp.StatusCode, tt.wantStatus)
}
if tt.wantBody != "" {
b := body(t, resp)
if !strings.Contains(b, tt.wantBody) {
t.Errorf("body missing %q", tt.wantBody)
}
}
})
}
}

// ---------------------------------------------------------------------------
// TestServerInputRoute
// ---------------------------------------------------------------------------

func TestServerInputRoute(t *testing.T) {
tests := []struct {
name          string
component     Component
path          string
method        string
wantStatus    int
wantInBody    string
wantCTAIndex  bool // when InputRequired true, index should show call-to-action
}{
{
name:       "GET /input/{name} served by InputHandler",
component:  stubInputHandler{stubComponent{"enablebanking"}, false, "paste url here"},
path:       "/input/enablebanking",
method:     http.MethodGet,
wantStatus: http.StatusOK,
wantInBody: "paste url here",
},
{
name:       "POST /input/{name} served by InputHandler",
component:  stubInputHandler{stubComponent{"enablebanking"}, false, "received"},
path:       "/input/enablebanking",
method:     http.MethodPost,
wantStatus: http.StatusOK,
},
{
name:       "unknown component returns 404",
component:  stubInputHandler{stubComponent{"enablebanking"}, false, ""},
path:       "/input/nordigen",
method:     http.MethodGet,
wantStatus: http.StatusNotFound,
},
{
name:          "InputRequired=true shows call-to-action on index",
component:     stubInputHandler{stubComponent{"enablebanking"}, true, ""},
path:          "/",
method:        http.MethodGet,
wantStatus:    http.StatusOK,
wantCTAIndex:  true,
},
{
name:         "InputRequired=false shows no call-to-action on index",
component:    stubInputHandler{stubComponent{"enablebanking"}, false, ""},
path:         "/",
method:       http.MethodGet,
wantStatus:   http.StatusOK,
wantCTAIndex: false,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

srv := NewServer(0, "", nil)
srv.Register(tt.component)
if err := srv.Start(ctx); err != nil {
t.Fatalf("Start(): %v", err)
}

url := fmt.Sprintf("http://127.0.0.1:%d%s", srv.Port(), tt.path)
var resp *http.Response
if tt.method == http.MethodPost {
var err error
resp, err = http.Post(url, "application/x-www-form-urlencoded", nil)
if err != nil {
t.Fatalf("POST %s: %v", tt.path, err)
}
} else {
resp = get(t, url)
}
defer resp.Body.Close()

if resp.StatusCode != tt.wantStatus {
t.Errorf("%s %s = %d, want %d", tt.method, tt.path, resp.StatusCode, tt.wantStatus)
}

b := body(t, resp)
if tt.wantInBody != "" && !strings.Contains(b, tt.wantInBody) {
t.Errorf("body missing %q", tt.wantInBody)
}
if tt.wantCTAIndex && !strings.Contains(b, "action-required") {
t.Errorf("index missing call-to-action marker when InputRequired=true")
}
if !tt.wantCTAIndex && tt.path == "/" && strings.Contains(b, "action-required") {
t.Errorf("index has unexpected call-to-action when InputRequired=false")
}
})
}
}

// ---------------------------------------------------------------------------
// TestServerContextCancellation — server shuts down cleanly on ctx cancel
// ---------------------------------------------------------------------------

func TestServerContextCancellation(t *testing.T) {
ctx, cancel := context.WithCancel(context.Background())
srv := NewServer(0, "", nil)
if err := srv.Start(ctx); err != nil {
t.Fatalf("Start(): %v", err)
}
port := srv.Port()

// Confirm server is up
resp := get(t, fmt.Sprintf("http://127.0.0.1:%d/health", port))
resp.Body.Close()
if resp.StatusCode != http.StatusOK {
t.Fatalf("server not up before cancel")
}

// Cancel context and wait for shutdown
cancel()
deadline := time.Now().Add(2 * time.Second)
for time.Now().Before(deadline) {
conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
if err != nil {
return // server stopped — test passes
}
conn.Close()
time.Sleep(50 * time.Millisecond)
}
t.Error("server still accepting connections 2s after context cancellation")
}

// ---------------------------------------------------------------------------
// TestServerBindsLocalhost — server must not bind on 0.0.0.0
// ---------------------------------------------------------------------------

func TestServerBindsLocalhost(t *testing.T) {
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

srv := NewServer(0, "", nil)
if err := srv.Start(ctx); err != nil {
t.Fatalf("Start(): %v", err)
}

port := srv.Port()

// Reachable on 127.0.0.1
resp := get(t, fmt.Sprintf("http://127.0.0.1:%d/health", port))
resp.Body.Close()
if resp.StatusCode != http.StatusOK {
t.Fatalf("server not reachable on 127.0.0.1")
}

// Must NOT be reachable on any non-loopback (external) interface.
// Probing 0.0.0.0 as a destination routes to loopback on Linux and cannot
// distinguish a 127.0.0.1-only bind, so we find a real external IP instead.
var externalIP string
ifaces, _ := net.Interfaces()
for _, iface := range ifaces {
if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
continue
}
addrs, _ := iface.Addrs()
for _, addr := range addrs {
var ip net.IP
switch v := addr.(type) {
case *net.IPNet:
ip = v.IP
case *net.IPAddr:
ip = v.IP
}
if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
externalIP = ip.String()
break
}
}
if externalIP != "" {
break
}
}
if externalIP == "" {
t.Skip("no non-loopback interface found, skipping external binding check")
}
conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", externalIP, port), 200*time.Millisecond)
if err == nil {
conn.Close()
t.Errorf("server is reachable on external interface %s — should only bind 127.0.0.1", externalIP)
}
}

// ---------------------------------------------------------------------------
// TestNewServer — edge cases for construction
// ---------------------------------------------------------------------------

func TestNewServer(t *testing.T) {
tests := []struct {
name    string
port    int
wantErr bool
}{
{name: "port 0 (OS assigns)", port: 0, wantErr: false},
{name: "negative port", port: -1, wantErr: true},
{name: "privileged port (requires root)", port: 80, wantErr: true},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

srv := NewServer(tt.port, "", nil)
err := srv.Start(ctx)
if (err != nil) != tt.wantErr {
t.Errorf("Start(port=%d) error = %v, wantErr %v", tt.port, err, tt.wantErr)
}
})
}
}
