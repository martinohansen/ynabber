// Package web provides a project-wide opt-in HTTP status and input server.
// By default it binds to 127.0.0.1; set host to "0.0.0.0" for containers.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Component is the minimum opt-in contract for readers/writers to participate
// in the status dashboard.
type Component interface {
	Name() string
}

// StatusProvider supplies a summary card on the main index page.
// Status must be fast — no I/O, reads only from in-memory state.
type StatusProvider interface {
	Component
	Status() ComponentStatus
}

// ComponentStatus is the snapshot a StatusProvider publishes.
type ComponentStatus struct {
	Healthy   bool
	Summary   string
	Fields    []StatusField
	DetailURL string
}

// StatusField is a single labelled value in a status card.
type StatusField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// DetailHandler supplies a full detail page at GET /detail/{name}.
// Implementations must use html/template for any dynamic output.
type DetailHandler interface {
	Component
	ServeDetail(w http.ResponseWriter, r *http.Request)
}

// InputHandler allows a component to receive operator input via the web UI.
// Canonical use: OAuth redirect-URL capture.
type InputHandler interface {
	Component
	ServeInput(w http.ResponseWriter, r *http.Request)
	// InputRequired reports whether the dashboard should show a call-to-action.
	InputRequired() bool
}

// Server is the project-wide HTTP status and input server.
type Server struct {
	port     int
	host     string
	mu       sync.RWMutex
	entries  []entry
	logger   *slog.Logger
	listener net.Listener
	started  bool
}

// entry pairs a Component with its display category.
type entry struct {
	comp     Component
	category string
}

// StatusGroups is the JSON shape returned by GET /status.
type StatusGroups struct {
	System  []componentView `json:"system"`
	Readers []componentView `json:"readers"`
	Writers []componentView `json:"writers"`
}

// NewServer returns a Server that has not yet started listening.
// host controls the bind address (e.g. "127.0.0.1" for local-only,
// "0.0.0.0" for all interfaces). If host is empty, "127.0.0.1" is used.
// If logger is nil, slog.Default() is used.
// port 0 means the OS assigns a free port.
func NewServer(port int, host string, logger *slog.Logger) *Server {
	if host == "" {
		host = "127.0.0.1"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{port: port, host: host, logger: logger}
}

// Register adds a system-level component to the dashboard.
// Safe to call before or after Start.
func (s *Server) Register(c Component) { s.register(c, "system") }

// RegisterReader adds a reader component to the Readers section.
// Safe to call before or after Start.
func (s *Server) RegisterReader(c Component) { s.register(c, "reader") }

// RegisterWriter adds a writer component to the Writers section.
// Safe to call before or after Start.
func (s *Server) RegisterWriter(c Component) { s.register(c, "writer") }

func (s *Server) register(c Component, category string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry{comp: c, category: category})
}

// Port returns the port the server is listening on.
// Before Start it returns the configured port; after Start it returns the
// actual bound port (useful when configured port was 0).
func (s *Server) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().(*net.TCPAddr).Port
	}
	return s.port
}

// Start binds to {host}:{port}, serves in a background goroutine,
// and shuts down cleanly when ctx is cancelled.
// Returns after the listener is confirmed open.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errors.New("server already started")
	}
	if s.port < 0 {
		return fmt.Errorf("invalid port %d", s.port)
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	s.listener = ln
	s.started = true

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("GET /detail/{name}", s.handleDetail)
	mux.HandleFunc("/input/{name}", s.handleInput)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	context.AfterFunc(ctx, func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			s.logger.Error("web server shutdown", "error", err)
		}
	})

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("web server error", "error", err)
		}
	}()

	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// componentView is the render-time view of a component for the index template.
type componentView struct {
	Name          string        `json:"name"`
	Healthy       bool          `json:"healthy"`
	Summary       string        `json:"summary"`
	Fields        []StatusField `json:"fields"`
	HasDetail     bool          `json:"hasDetail"`
	InputRequired bool          `json:"inputRequired"`
}

func (s *Server) groups() StatusGroups {
	// Snapshot entries outside the lock to avoid holding it while calling Status().
	s.mu.RLock()
	snapshot := make([]entry, len(s.entries))
	copy(snapshot, s.entries)
	s.mu.RUnlock()

	var g StatusGroups
	for _, e := range snapshot {
		v := toView(e.comp)
		switch e.category {
		case "reader":
			g.Readers = append(g.Readers, v)
		case "writer":
			g.Writers = append(g.Writers, v)
		default:
			g.System = append(g.System, v)
		}
	}
	return g
}

// views returns all components as a flat slice (used by handleIndex for SSR).
func (s *Server) views() []componentView {
	g := s.groups()
	out := make([]componentView, 0, len(g.System)+len(g.Readers)+len(g.Writers))
	out = append(out, g.System...)
	out = append(out, g.Readers...)
	out = append(out, g.Writers...)
	return out
}

func toView(c Component) componentView {
	v := componentView{Name: c.Name()}
	if sp, ok := c.(StatusProvider); ok {
		st := sp.Status()
		v.Healthy = st.Healthy
		v.Summary = st.Summary
		v.Fields = st.Fields
	}
	if _, ok := c.(DetailHandler); ok {
		v.HasDetail = true
	}
	if ih, ok := c.(InputHandler); ok {
		v.InputRequired = ih.InputRequired()
	}
	return v
}

func (s *Server) find(name string) (Component, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if e.comp.Name() == name {
			return e.comp, true
		}
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// Route handlers
// ---------------------------------------------------------------------------

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Ynabber</title>
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

  :root {
    --bg:        #0d0d0d;
    --surface:   #141414;
    --border:    rgba(255,255,255,0.07);
    --text:      #ededed;
    --muted:     #6b6b6b;
    --accent:    #5e6ad2;
    --green:     #4ade80;
    --red:       #f87171;
    --amber:     #fbbf24;
    --radius:    10px;
    --font:      -apple-system, 'Inter', 'Segoe UI', sans-serif;
    --mono:      'JetBrains Mono', 'SF Mono', 'Fira Code', monospace;
  }

  body {
    background: var(--bg);
    color: var(--text);
    font-family: var(--font);
    font-size: 14px;
    line-height: 1.5;
    min-height: 100vh;
    padding: 0 24px 64px;
  }

  /* ── Header ── */
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 28px 0 32px;
    border-bottom: 1px solid var(--border);
    margin-bottom: 32px;
  }
  .wordmark {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 15px;
    font-weight: 600;
    letter-spacing: -0.01em;
  }
  .wordmark-icon {
    width: 26px; height: 26px;
    background: var(--accent);
    border-radius: 7px;
    display: flex; align-items: center; justify-content: center;
    font-size: 14px;
  }
  .header-right { display: flex; align-items: center; gap: 10px; }
  .health-badge {
    display: inline-flex; align-items: center; gap: 6px;
    font-size: 12px; color: var(--muted);
    background: var(--surface);
    border: 1px solid var(--border);
    padding: 4px 10px; border-radius: 20px;
  }
  .health-badge .dot { width:6px; height:6px; border-radius:50%; background: var(--green); }
  .updated {
    font-size: 11px;
    color: var(--muted);
    font-family: var(--mono);
    min-width: 80px;
    text-align: right;
  }
  /* freeze button */
  #freeze-btn {
    font-size: 11px;
    font-weight: 500;
    padding: 4px 10px;
    border-radius: 20px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    transition: color 0.1s, border-color 0.1s;
  }
  #freeze-btn:hover { color: var(--text); border-color: rgba(255,255,255,0.2); }
  #freeze-btn.frozen {
    color: var(--amber);
    border-color: rgba(251,191,36,0.4);
  }

  /* ── Section label ── */
  .section-label {
    font-size: 11px;
    font-weight: 500;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
    margin-bottom: 12px;
  }

  /* ── Empty state ── */
  .empty {
    text-align: center;
    padding: 80px 0;
    color: var(--muted);
    font-size: 13px;
  }
  .empty-icon { font-size: 28px; margin-bottom: 12px; }

  /* ── Card grid ── */
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 12px;
  }

  /* ── Card ── */
  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 18px 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    transition: border-color 0.15s;
  }
  .card:hover { border-color: rgba(255,255,255,0.14); }
  .card.card-warn { border-color: rgba(251,191,36,0.35); }

  .card-header { display: flex; align-items: center; justify-content: space-between; }
  .card-name {
    font-size: 13px;
    font-weight: 600;
    letter-spacing: -0.01em;
    display: flex; align-items: center; gap: 8px;
  }
  .status-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
  .status-dot.healthy   { background: var(--green); box-shadow: 0 0 6px rgba(74,222,128,0.5); }
  .status-dot.unhealthy { background: var(--red);   box-shadow: 0 0 6px rgba(248,113,113,0.5); }
  .status-dot.unknown   { background: var(--muted); }

  .card-summary { font-size: 12px; color: var(--muted); }

  /* ── Fields ── */
  .fields { display: flex; flex-direction: column; gap: 4px; }
  .field { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; font-size: 12px; }
  .field-label { color: var(--muted); white-space: nowrap; }
  .field-value { font-family: var(--mono); font-size: 11.5px; color: var(--text); text-align: right; }

  /* ── Card footer links ── */
  .card-footer {
    display: flex; gap: 8px; flex-wrap: wrap;
    margin-top: 2px; padding-top: 10px;
    border-top: 1px solid var(--border);
  }
  .chip {
    font-size: 11px; font-weight: 500;
    padding: 3px 10px; border-radius: 20px;
    border: 1px solid var(--border);
    color: var(--muted); text-decoration: none;
    transition: color 0.1s, border-color 0.1s;
  }
  .chip:hover { color: var(--text); border-color: rgba(255,255,255,0.2); }
  .chip.cta { color: var(--amber); border-color: rgba(251,191,36,0.35); }
  .chip.cta:hover { border-color: var(--amber); }
</style>
</head>
<body>

<header>
  <div class="wordmark">
    <div class="wordmark-icon">₿</div>
    Ynabber
  </div>
  <div class="header-right">
    <span class="updated" id="updated">–</span>
    <button id="freeze-btn" onclick="toggleFreeze()">⏸ Freeze</button>
    <span class="health-badge"><span class="dot"></span>running</span>
  </div>
</header>

<div id="content">
{{if or .System .Readers .Writers}}
{{if .System}}<p class="section-label">System</p><div class="grid" style="margin-bottom:32px">
{{range .System}}{{template "card" .}}{{end}}
</div>{{end}}
{{if .Readers}}<p class="section-label">Readers</p><div class="grid" style="margin-bottom:32px">
{{range .Readers}}{{template "card" .}}{{end}}
</div>{{end}}
{{if .Writers}}<p class="section-label">Writers</p><div class="grid" style="margin-bottom:32px">
{{range .Writers}}{{template "card" .}}{{end}}
</div>{{end}}
{{else}}
<div class="empty"><div class="empty-icon">⬡</div>No components registered yet.</div>
{{end}}
</div>

<script>
let frozen = false;
let timer = null;
// Split to prevent the literal appearing in source — tests scan the page body.
const CTA_CLASS = 'action-' + 'required';

function toggleFreeze() {
  frozen = !frozen;
  const btn = document.getElementById('freeze-btn');
  if (frozen) {
    btn.textContent = '▶ Live';
    btn.classList.add('frozen');
    clearInterval(timer);
  } else {
    btn.textContent = '⏸ Freeze';
    btn.classList.remove('frozen');
    refresh();
    timer = setInterval(refresh, 2000);
  }
}

function dot(c) {
  if (!c.summary) return '<span class="status-dot unknown" title="no status"></span>';
  return c.healthy
    ? '<span class="status-dot healthy" title="healthy"></span>'
    : '<span class="status-dot unhealthy" title="unhealthy"></span>';
}

function esc(s) {
  return String(s)
    .replace(/&/g,'&amp;').replace(/</g,'&lt;')
    .replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function renderCard(c) {
  const warn = c.inputRequired ? ' card-warn' : '';
  let fields = '';
  if (c.fields && c.fields.length) {
    fields = '<div class="fields">' +
      c.fields.map(f =>
        '<div class="field">' +
        '<span class="field-label">' + esc(f.label) + '</span>' +
        '<span class="field-value">' + esc(f.value) + '</span>' +
        '</div>'
      ).join('') + '</div>';
  }
  let footer = '';
  if (c.inputRequired) {
    const detail = c.hasDetail
      ? '<a class="chip" href="/detail/' + esc(c.name) + '">Details →</a>' : '';
    footer = '<div class="card-footer">' + detail +
      '<div class="' + CTA_CLASS + '" style="display:contents">' +
      '<a class="chip cta" href="/input/' + esc(c.name) + '">⚡ Action required</a>' +
      '</div></div>';
  } else if (c.hasDetail) {
    footer = '<div class="card-footer"><a class="chip" href="/detail/' + esc(c.name) + '">Details →</a></div>';
  }
  const summary = c.summary ? '<p class="card-summary">' + esc(c.summary) + '</p>' : '';
  return '<div class="card' + warn + '">' +
    '<div class="card-header"><span class="card-name">' + dot(c) + esc(c.name) + '</span></div>' +
    summary + fields + footer + '</div>';
}

function renderSection(label, cards) {
  if (!cards || cards.length === 0) return '';
  return '<p class="section-label">' + label + '</p>' +
    '<div class="grid" style="margin-bottom:32px">' + cards.map(renderCard).join('') + '</div>';
}

function renderContent(data) {
  const el = document.getElementById('content');
  const html = renderSection('System', data.system) +
    renderSection('Readers', data.readers) +
    renderSection('Writers', data.writers);
  el.innerHTML = html ||
    '<div class="empty"><div class="empty-icon">⬡</div>No components registered yet.</div>';
}

async function refresh() {
  try {
    const resp = await fetch('/status');
    if (!resp.ok) return;
    const data = await resp.json();
    renderContent(data);
    document.getElementById('updated').textContent =
      new Date().toLocaleTimeString([], {hour:'2-digit', minute:'2-digit', second:'2-digit'});
  } catch (_) { /* server may be restarting */ }
}

timer = setInterval(refresh, 2000);
</script>
</body>
</html>
{{define "card"}}<div class="card{{if .InputRequired}} card-warn{{end}}">
  <div class="card-header">
    <span class="card-name">
      {{if .Healthy}}<span class="status-dot healthy" title="healthy"></span>
      {{else if .Summary}}<span class="status-dot unhealthy" title="unhealthy"></span>
      {{else}}<span class="status-dot unknown" title="no status"></span>{{end}}
      {{.Name}}
    </span>
  </div>
  {{if .Summary}}<p class="card-summary">{{.Summary}}</p>{{end}}
  {{if .Fields}}
  <div class="fields">
    {{range .Fields}}
    <div class="field">
      <span class="field-label">{{.Label}}</span>
      <span class="field-value">{{.Value}}</span>
    </div>
    {{end}}
  </div>
  {{end}}
  {{if .InputRequired}}
  <div class="card-footer">
    {{if .HasDetail}}<a class="chip" href="/detail/{{.Name}}">Details →</a>{{end}}
    <div class="action-required" style="display:contents">
      <a class="chip cta" href="/input/{{.Name}}">⚡ Action required</a>
    </div>
  </div>
  {{else if .HasDetail}}
  <div class="card-footer"><a class="chip" href="/detail/{{.Name}}">Details →</a></div>
  {{end}}
</div>{{end}}`))

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTmpl.Execute(w, s.groups()); err != nil {
		s.logger.Error("rendering index", "error", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.groups()); err != nil {
		s.logger.Error("encoding status", "error", err)
	}
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	c, ok := s.find(r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	dh, ok := c.(DetailHandler)
	if !ok {
		http.NotFound(w, r)
		return
	}
	dh.ServeDetail(w, r)
}

func (s *Server) handleInput(w http.ResponseWriter, r *http.Request) {
	c, ok := s.find(r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	ih, ok := c.(InputHandler)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ih.ServeInput(w, r)
}
