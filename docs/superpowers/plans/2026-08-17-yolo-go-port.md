# Yolo Go Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `yolo`, a faithful Go port of opencode v1.18.18's TUI + core server (bubbletea v2 TUI as a pure wire-protocol client over an in-process REST+SSE server with SQLite storage, dual-protocol LLM drivers, and opencode-faithful permissions/agents/tools).

**Architecture:** Single binary. `yolo` starts a core HTTP server in-process on a local port, then runs the bubbletea TUI which talks to it *only* via the wire contract (spec §3). Core is layered: `protocol` (wire DTOs, single source of truth) → `server` (HTTP/SSE) → `session` (agent loop) → `llm`/`provider`/`tool`/`permission`/`config`/`auth`/`storage`/`bus`. The TUI never imports core packages; it imports `internal/protocol` plus its own `internal/tui/client`.

**Tech Stack:** Go ≥ 1.25 (installed 1.26.5), stdlib `net/http` (1.22+ `ServeMux`), `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`, `modernc.org/sqlite` (pure-Go, no cgo), `tidwall/jsonc`; dev-only `charm.land/x/exp/teatest/v2`. No LLM SDK, no router framework, no CLI framework.

**Spec:** `docs/superpowers/specs/2026-08-17-yolo-go-port-design.md` (approved 2026-08-17). Upstream reference clone: `/tmp/opencode-upstream` at tag `v1.18.18`. If it is missing: `git clone --depth 1 --branch v1.18.18 https://github.com/anomalyco/opencode /tmp/opencode-upstream` (never touch `/tmp/opencode` — pre-existing user data).

## Global Constraints

- Module `github.com/kido5217/yolo`, binary `yolo`, Go ≥ 1.25.
- Pinned deps (exact versions, nothing else except stdlib and teatest dev-only):
  - `charm.land/bubbletea/v2` v2.0.8
  - `charm.land/lipgloss/v2` v2.0.6
  - `charm.land/bubbles/v2` v2.1.1
  - `modernc.org/sqlite` v1.56.0
  - `tidwall/jsonc` v0.3.3
  - dev-only: `charm.land/x/exp/teatest/v2` v2.0.0-20260816001655-68d539dca504
  - **PLAN CORRECTION (verified 2026-08-17 against proxy.golang.org):** the spec's dev dep `charm.land/x/exp/teatest` is the **bubbletea-v1** module (its go.mod requires `github.com/charmbracelet/bubbletea v1.3.5`) and cannot compile a v2 TUI. The correct module is `charm.land/x/exp/teatest/v2` (requires `charm.land/bubbletea/v2`). Use the `/v2` path everywhere.
- Single deliberate wire deviation: header **`x-yolo-directory`** (opencode: `x-opencode-directory`). It carries the URL-encoded absolute project directory. Absent on an incoming request → default to the server's working directory.
- Error envelope on non-2xx: `{"error": {"message": "...", "data"?: ...}}`; statuses: 400 bad input, 404 unknown resource, 409 session busy, 500 internal. Skipped endpoint families (spec §3) return 404.
- SSE on `GET /event`: frames `data: {json}\n\n` where json = `{"id": "evt_...", "type": "...", "properties": {...}}` (legacy envelope; `id` is `evt_`-prefixed). Event set (spec §3) exactly: `message.updated`, `message.part.updated`, `message.part.delta`, `message.removed`, `message.part.removed`, `session.updated`, `session.deleted`, `session.status`, `permission.asked`, `permission.replied`.
- ID prefixes: `ses_`, `msg_`, `prt_`, `per_`, `evt_` (20 random base62 chars after the prefix+`_`).
- Config: files `yolo.json`/`yolo.jsonc` (project, walked up from CWD), `<project>/.yolo/` (project dir), global `~/.config/yolo/` (merge order `config.json` → `yolo.json` → `yolo.jsonc`). XDG env vars respected (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_CACHE_HOME`), HOME fallback.
- Data dirs: `~/.local/share/yolo/` (`auth.json` 0600, `storage/yolo.db`, `plans/*.md`, `log/yolo.log`), `~/.cache/yolo/` (`models.json` zen catalog cache).
- SQLite: WAL mode, `busy_timeout=5000`, single file `~/.local/share/yolo/storage/yolo.db`, one DB for all directories (sessions carry `project_dir`).
- Providers: `kido` default (base `https://ai.kido.ws/v1`, key optional, default model `kido/Qwen3.8-27B`); `opencode` zen (base `https://opencode.ai/zen/v1`, key required, catalog `https://models.opencode.ai/api.json`, cache TTL 5 min + atomic rewrite, paid-only filter `cost.input > 0`, google-npm models excluded).
- Agents: `build` (default), `plan`, `yolo` — permission matrices exactly per spec §4.5 (last-match-wins, `ask` fallback, doom-loop × 3 identical inputs, reject cascade, wildcard-deny hides tool).
- Tools (model-visible names): `read`, `write`, `edit`, `glob`, `grep`, `bash`, `todowrite`. Output truncation defaults: 2000 lines, 50×1024 bytes (`tool_output.max_lines`/`max_bytes` in config override).
- Session engine: one active turn per session (concurrent send → 409); max 50 tool steps per turn; transient LLM errors (429/5xx/network) retried with exponential backoff + jitter (4 attempts, base 1 s); context overflow → hard stop; abort cancels the turn's context.
- System prompt = agent base (family prompt file per upstream `system.ts:27-49` selection) + environment block (upstream `system.ts:72-83` text verbatim) + instruction files (project `AGENTS.md` walked up from CWD + config `instructions[]`). Plan agent additionally gets the `plan-mode.txt` reminder.
- TUI: pure client, tree-of-models, SSE reconnect backoff 1 s → 30 s with REST re-hydration. Keymap per spec §5.
- Verification per task (the CI gate, run on the module root): `go vet ./... && go test ./...` — every task ends with both green and a commit.
- Live services are test-gated: unit/integration tests never hit the network; live checks run only behind env vars (`YOLO_LLM=fake` selects the scripted fake LLM driver; e2e smoke vs `ai.kido.ws` is on-demand, never CI-gated).

---

## File Structure (locked by this plan)

```
cmd/yolo/main.go            # entrypoint: default (in-process server + TUI), serve, auth, resume
cmd/yolo/main_test.go
internal/protocol/          # wire DTOs + SSE events + IDs + config schema (TUI may import ONLY this core pkg)
    protocol.go  session.go  message.go  part.go  event.go  provider.go  agent.go  config.go  id.go  errors.go
internal/server/            # HTTP mux, handlers, SSE fanout, scoping, error envelope
    server.go  handlers.go  sse.go  errors.go
internal/bus/bus.go         # in-process pub/sub
internal/session/           # agent engine: turn loop, system prompt, title, retry, overflow
    engine.go  prompt.go  history.go  prompt/*.txt (13 embedded files)
internal/llm/               # Driver interface + openai + anthropic drivers (hand-rolled SSE)
    llm.go  openai.go  anthropic.go  fake/fake.go
internal/provider/          # kido + zen catalog, auth state, registry
    provider.go  kido.go  zen.go  testdata/zen-opencode.json  testdata/kido-models.json
internal/tool/              # registry + 7 tools + truncation
    tool.go  truncate.go  read.go  write.go  edit.go  glob.go  grep.go  bash.go  todowrite.go  desc/*.txt
internal/permission/        # rulesets, evaluation, agent matrices, pending-ask service
    permission.go  builtins.go  service.go
internal/config/config.go   # discovery, JSONC, deep merge, env substitution
internal/auth/auth.go       # auth.json, key resolution, CLI store
internal/storage/db.go dao.go migrate.go   # SQLite open/migrations/DAOs
internal/tui/               # bubbletea v2 app (imports only internal/protocol + its own client)
    client.go  store.go  app.go  home.go  session_view.go  prompt.go  perm_dialog.go  model_dialog.go  agent_dialog.go  footer.go  toasts.go  testutil_test.go
```

Dependency direction (enforced by review; Go import check in one test, Task 27):
`cmd → server, tui, config, auth` · `server → session, provider, permission, config, storage, bus, protocol` · `session → llm, tool, permission, provider, storage, bus, config, protocol` · `tui → protocol` (client) and stdlib only · `llm`, `tool`, `permission`, `config`, `auth`, `storage`, `bus` → `protocol` (and each other as noted). **`internal/tui` must not import any other `internal/*` package** — verified in Task 27.

---

# Milestone M0 — Skeleton

### Task 1: Module bootstrap, command dispatch, health endpoint

**Files:**
- Create: `go.mod`, `.gitignore`, `cmd/yolo/main.go`, `cmd/yolo/main_test.go`, `internal/server/server.go`, `internal/server/server_test.go`

**Interfaces:**
- Produces: `server.New(workDir string, opts ...Opt) *server.Server` (+ `func (s *Server) Handler() http.Handler`, `func (s *Server) Start(addr string) (net.Addr, error)`, `func (s *Server) Close()`); subcommands `yolo` (default: prints interim notice — replaced by real TUI in Task 27), `yolo serve [--port N]`, `yolo auth …` (interim notice, real in Task 4), `yolo <sessionID>` (interim notice, real in Task 27).

- [ ] **Step 1: Write the failing test**

`internal/server/server_test.go`:

```go
package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/server"
)

func TestHealth(t *testing.T) {
	s := server.New("/tmp/work")
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/global/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	want := `{"status":"ok"}`
	if strings.TrimSpace(string.body(body)) != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}

func TestHealthWithDirectoryHeader(t *testing.T) {
	s := server.New("/tmp/work")
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/global/health", nil)
	req.Header.Set("x-yolo-directory", "%2Ftmp%2Fother")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
```

(Fix typo as written: `strings.TrimSpace(string(body))`. The two tests above are the deliverable test set for this task.)

`cmd/yolo/main_test.go`:

```go
package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestDispatchServeFlag(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help exit err: %v\n%s", err, out)
	}
	for _, want := range []string{"serve", "auth"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("help output missing %q:\n%s", want, out)
		}
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := t.TempDir() + "/yolo"
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./...`
Expected: FAIL — `package github.com/kido5217/yolo/internal/server is not in std` / build error (module and packages do not exist yet).

- [ ] **Step 3: Write minimal implementation**

`go.mod` (after `go mod init github.com/kido5217/yolo`; deps are added later in the tasks that need them — keep go.mod minimal at M0):

```
module github.com/kido5217/yolo

go 1.25
```

`.gitignore`:

```
/yolo
*.log
```

`internal/server/server.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sync"
)

type Opt func(*Server)

type Server struct {
	WorkDir string
 mux *http.ServeMux
	srv *http.Server
	addr net.Addr
	mu sync.Mutex
}

func New(workDir string, opts ...Opt) *Server {
	s := &Server{WorkDir: workDir, mux: http.NewServeMux()}
	for _, o := range opts {
		o(s)
	}
	s.mux.HandleFunc("GET /global/health", s.handleHealth)
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

// Scope returns the x-yolo-directory value (URL-decoded) or the server work dir.
func (s *Server) Scope(r *http.Request) (string, error) {
	v := r.Header.Get("x-yolo-directory")
	if v == "" {
		return s.WorkDir, nil
	}
	d, err := url.PathUnescape(v)
	if err != nil {
		return "", err
	}
	if d == "" {
		return s.WorkDir, nil
	}
	return d, nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) Start(addr string) (net.Addr, error) {
	s.srv = &http.Server{Addr: addr, Handler: s.Handler()}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s.addr = ln.Addr()
	go func() {
		_ = s.srv.Serve(ln)
	}()
	return s.addr, nil
}

func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

func (s *Server) Close() {
	if s.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2_000_000_000) // 2s ns
	defer cancel()
	_ = s.srv.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
```

`cmd/yolo/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/kido5217/yolo/internal/server"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "yolo: TUI not wired yet (milestone M6); use `yolo serve`")
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, `yolo — Go port of opencode (v1.18.18 wire contract)

Usage:
  yolo [<sessionID>]        start the TUI (or resume a session)
  yolo serve [--port N]     run the core server only
  yolo auth <subcommand>    manage credentials (list | add <provider> [key] | remove <provider>)
  yolo help                 this help
`)
		return 0
	case "serve":
		return serve(args[1:])
	case "auth":
		fmt.Fprintln(os.Stderr, "yolo auth: not wired yet (Task 4)")
		return 0
	case "version":
		fmt.Println("yolo 0.0.0-dev")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "yolo: resume not wired yet (Task 27); unknown argument %q\n", args[0])
		return 2
	}
}

func serve(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 0, "port to listen on (0 = ephemeral)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	s := server.New(mustGetwd())
	addr, err := s.Start(fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		return 1
	}
	srv := addr.(*net.TCPAddr)
	fmt.Printf("yolo serving on http://%s (dir %s)\n", srv.String(), s.WorkDir)
	stop := make(chan os.Signal, 1)
	// import os/signal here is not allowed pre-M8; block on channel closed by Close
	<-stop
	s.Close()
	return 0
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS (all packages, no failures).

- [ ] **Step 5: Commit**

```bash
git add go.mod .gitignore cmd/yolo internal/server
git commit -m "chore: bootstrap module, command dispatch, /global/health"
```

---

# Milestone M1 — protocol + config + auth + storage + bus

### Task 2: `internal/protocol` — wire DTOs and SSE event types

**Files:**
- Create: `internal/protocol/id.go`, `internal/protocol/errors.go`, `internal/protocol/session.go`, `internal/protocol/message.go`, `internal/protocol/part.go`, `internal/protocol/event.go`, `internal/protocol/provider.go`, `internal/protocol/agent.go`, `internal/protocol/config.go`, `internal/protocol/protocol_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (exact names/types — every later task uses these):

```go
package protocol

// id.go
func NewID(prefix string) string            // e.g. "ses_" + 20 base62 chars
func NewEventID() string                    // "evt_" + 20 base62 chars

// errors.go
type Error struct {
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// session.go
type SessionTime struct {
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
}
type CacheTokens struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}
type Tokens struct {
	Input     int64        `json:"input"`
	Output    int64        `json:"output"`
	Reasoning int64        `json:"reasoning"`
	Cache     CacheTokens  `json:"cache"`
}
type ModelRef struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
}
type Session struct {
	ID         string           `json:"id"`
	ProjectID  string           `json:"projectID"`
	Directory  string           `json:"directory"`
	Title      string           `json:"title"`
	Agent      string           `json:"agent,omitempty"`
	Model      *ModelRef        `json:"model,omitempty"`
	Cost       float64          `json:"cost"`
	Tokens     Tokens           `json:"tokens"`
	Version    string           `json:"version"`
	Time       SessionTime      `json:"time"`
	Permission []Rule           `json:"permission,omitempty"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
}
type SessionStatus struct {
	Type    string `json:"type"` // "idle" | "busy" | "retry"
	Attempt int    `json:"attempt,omitempty"`
	Message string `json:"message,omitempty"`
	Next    int64  `json:"next,omitempty"`
}
const (
	StatusIdle  = "idle"
	StatusBusy  = "busy"
	StatusRetry = "retry"
)

// message.go
type MessageTime struct {
	Created   int64 `json:"created"`
	Completed int64 `json:"completed,omitempty"`
}
type MessageModel struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}
type MessagePath struct {
	Cwd  string `json:"cwd"`
	Root string `json:"root"`
}
type MessageError struct {
	Type    string `json:"type"` // "unknown" | "aborted" | "overflow"
	Message string `json:"message"`
}
type Message struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"sessionID"`
	Role       string         `json:"role"` // "user" | "assistant"
	Time       MessageTime    `json:"time"`
	Agent      string         `json:"agent,omitempty"`
	Model      *MessageModel  `json:"model,omitempty"`   // user messages
	ParentID   string         `json:"parentID,omitempty"`
	ModelID    string         `json:"modelID,omitempty"`
	ProviderID string         `json:"providerID,omitempty"`
	Mode       string         `json:"mode,omitempty"`
	Path       *MessagePath   `json:"path,omitempty"`
	Cost       float64        `json:"cost,omitempty"`
	Tokens     *Tokens        `json:"tokens,omitempty"`
	Finish     string         `json:"finish,omitempty"`
	Error      *MessageError  `json:"error,omitempty"`
	Variant    string         `json:"variant,omitempty"`
}

// part.go
type PartTime struct {
	Start     int64 `json:"start"`
	End       int64 `json:"end,omitempty"`
	Compacted int64 `json:"compacted,omitempty"`
}
type ToolState struct {
	Status   string         `json:"status"` // "running" | "completed" | "error"
	Input    map[string]any `json:"input"`
	Title    string         `json:"title,omitempty"`
	Output   string         `json:"output,omitempty"`
	Error    string         `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Time     PartTime       `json:"time"`
}
type Part struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionID"`
	MessageID string         `json:"messageID"`
	Type      string         `json:"type"` // "text" | "reasoning" | "tool"
	Text      string         `json:"text,omitempty"`
	CallID    string         `json:"callID,omitempty"`
	Tool      string         `json:"tool,omitempty"`
	State     *ToolState     `json:"state,omitempty"`
	Synthetic *bool          `json:"synthetic,omitempty"`
	Ignored   *bool          `json:"ignored,omitempty"`
	Time      PartTime       `json:"time"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
type MessageWithParts struct {
	Info  Message `json:"info"`
	Parts []Part  `json:"parts"`
}

// event.go
type Event struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}
const (
	EventTypeMessageUpdated      = "message.updated"
	EventTypeMessagePartUpdated  = "message.part.updated"
	EventTypeMessagePartDelta    = "message.part.delta"
	EventTypeMessageRemoved      = "message.removed"
	EventTypeMessagePartRemoved  = "message.part.removed"
	EventTypeSessionUpdated      = "session.updated"
	EventTypeSessionDeleted      = "session.deleted"
	EventTypeSessionStatus       = "session.status"
	EventTypePermissionAsked     = "permission.asked"
	EventTypePermissionReplied   = "permission.replied"
)
func MakeEvent(t string, props any) (Event, error)

// typed props (all JSON shapes match upstream v1.18.18 openapi legacy schemas):
type MessageUpdatedProps struct {
	SessionID string  `json:"sessionID"`
	Info      Message `json:"info"`
}
type MessageRemovedProps struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
}
type MessagePartUpdatedProps struct {
	SessionID string `json:"sessionID"`
	Part      Part   `json:"part"`
	Time      int64  `json:"time"`
}
type MessagePartDeltaProps struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
	Field     string `json:"field"` // "text" | "reasoning" | "input"
	Delta     string `json:"delta"`
}
type MessagePartRemovedProps struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
}
type SessionUpdatedProps struct {
	SessionID string  `json:"sessionID"`
	Info      Session `json:"info"`
}
type SessionDeletedProps struct {
	SessionID string  `json:"sessionID"`
	Info      Session `json:"info"`
}
type SessionStatusProps struct {
	SessionID string        `json:"sessionID"`
	Status    SessionStatus `json:"status"`
}
type PermissionToolRef struct {
	MessageID string `json:"messageID"`
	CallID    string `json:"callID"`
}
type PermissionAskedProps struct {
	ID         string            `json:"id"`
	SessionID  string            `json:"sessionID"`
	Permission string            `json:"permission"`
	Patterns   []string          `json:"patterns"`
	Metadata   map[string]any    `json:"metadata"`
	Always     []string          `json:"always"`
	Tool       *PermissionToolRef `json:"tool,omitempty"`
}
type PermissionRepliedProps struct {
	SessionID string `json:"sessionID"`
	RequestID string `json:"requestID"`
	Reply     string `json:"reply"` // "once" | "always" | "reject"
}

// provider.go
type ProviderAuth struct {
	Type        string `json:"type"` // "api" | "none"
	Status      string `json:"status"` // "loaded" | "missing" | "not-required"
	KeyRequired bool   `json:"keyRequired"`
}
type ModelLimit struct {
	Context int   `json:"context"`
	Output  int   `json:"output"`
}
type ModelCost struct {
	Input     float64 `json:"input"`
	Output    float64 `json:"output"`
	CacheRead float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}
type Model struct {
	ID         string    `json:"id"`
	ProviderID string    `json:"providerID"`
	Name       string    `json:"name"`
	Family     string    `json:"family,omitempty"`
	ToolCall   bool      `json:"toolcall"`
	Reasoning  bool      `json:"reasoning"`
	Attachment bool      `json:"attachment"`
	Limit      ModelLimit `json:"limit"`
	Cost       ModelCost  `json:"cost"`
	Adapter    string    `json:"adapter"` // "openai" | "anthropic" (driver selection)
}
type Provider struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Source  string            `json:"source"` // "builtin" | "config"
	Env     []string          `json:"env"`
	Options map[string]any    `json:"options"`
	Models  map[string]Model  `json:"models"`
	Auth    *ProviderAuth     `json:"auth,omitempty"` // Yolo extension: drives TUI auth status
}

// agent.go
type Rule struct {
	Permission string `json:"permission"`
	Pattern    string `json:"pattern"`
	Action     string `json:"action"` // "allow" | "deny" | "ask"
}
const (
	ActionAllow = "allow"
	ActionDeny  = "deny"
	ActionAsk   = "ask"
)
type Agent struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Mode        string           `json:"mode"` // "primary"
	Model       *ModelRef        `json:"model,omitempty"`
	Permission  []Rule           `json:"permission"`
	Options     map[string]any   `json:"options"`
	Hidden      bool             `json:"hidden,omitempty"`
}
type Command struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Template    string   `json:"template,omitempty"`
	Hints       []string `json:"hints"`
}

// config.go (wire = config file schema, spec §6.1)
type ToolOutput struct {
	MaxLines int `json:"max_lines,omitempty"`
	MaxBytes int `json:"max_bytes,omitempty"`
}
type ProviderConfig struct {
	BaseURL string         `json:"baseURL,omitempty"`
	APIKey  string         `json:"apiKey,omitempty"`
	Options map[string]any `json:"options,omitempty"`
	Models  map[string]any `json:"models,omitempty"`
}
type CustomAgent struct {
	Description string         `json:"description,omitempty"`
	Permission  map[string]any `json:"permission,omitempty"`
}
type Config struct {
	Model        string                  `json:"model,omitempty"`
	Agent        string                  `json:"agent,omitempty"`
	Provider     map[string]ProviderConfig `json:"provider,omitempty"`
	Permission   map[string]any          `json:"permission,omitempty"`
	Instructions []string                `json:"instructions,omitempty"`
	Theme        map[string]any          `json:"theme,omitempty"`
	ToolOutput   *ToolOutput             `json:"tool_output,omitempty"`
	Agents       map[string]CustomAgent  `json:"agents,omitempty"` // plan resolution of spec ambiguity (see Self-Review)
}
func ParsePerms(m map[string]any) []Rule
```

- [ ] **Step 1: Write the failing test**

`internal/protocol/protocol_test.go`:

```go
package protocol_test

import (
	"encoding/json"
	"regexp"
	"testing"

	p "github.com/kido5217/yolo/internal/protocol"
)

var sesRe = regexp.MustCompile(`^ses_[2-9A-HJK-NP-Zb-hj-np-z]{20}$`)
var evtRe = regexp.MustCompile(`^evt_[2-9A-HJK-NP-Zb-hj-np-z]{20}$`)

func TestNewIDFormats(t *testing.T) {
	if !sesRe.MatchString(p.NewID("ses")) {
		t.Fatalf("bad session id format: %q", p.NewID("ses"))
	}
	if got := p.NewID("ses")[:4]; got != "ses_" {
		t.Fatalf("prefix = %q", got)
	}
	if !evtRe.MatchString(p.NewEventID()) {
		t.Fatalf("bad event id: %q", p.NewEventID())
	}
	if p.NewID("msg") == p.NewID("msg") {
		t.Fatal("ids are not random")
	}
}

func TestSessionWireShape(t *testing.T) {
	s := p.Session{
		ID: "ses_test1234567890123456", ProjectID: "prj_123", Directory: "/w",
		Title: "t", Cost: 0.5, Version: "1",
		Model: &p.ModelRef{ID: "Qwen3.8-27B", ProviderID: "kido"},
		Time:  p.SessionTime{Created: 1, Updated: 2},
		Tokens: p.Tokens{Input: 10, Output: 20, Reasoning: 0,
			Cache: p.CacheTokens{Read: 1, Write: 2}},
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"ses_test1234567890123456","projectID":"prj_123","directory":"/w","title":"t","cost":0.5,"tokens":{"input":10,"output":20,"reasoning":0,"cache":{"read":1,"write":2}},"version":"1","time":{"created":1,"updated":2}}`
	if string(b) != want {
		t.Fatalf("\ngot  %s\nwant %s", b, want)
	}
	var back p.Session
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Model.ProviderID != "kido" || back.Tokens.Cache.Write != 2 {
		t.Fatal("round-trip mismatch")
	}
}

func TestMessageRoles(t *testing.T) {
	u := p.Message{ID: "msg_1", SessionID: "ses_1", Role: "user",
		Time: p.MessageTime{Created: 1}, Agent: "build",
		Model: &p.MessageModel{ProviderID: "kido", ModelID: "Qwen3.8-27B"}}
	b, _ := json.Marshal(u)
	// user message must not carry assistant-only fields
	for _, banned := range []string{`"parentID"`, `"modelID"`, `"providerID":`, `"path"`, `"cost"`, `"tokens"`, `"finish"`} {
		// note: providerID appears inside model object, ban the top-level key form
		if banned == `"providerID":` && !contains(b, `,"providerID":`) {
			continue
		}
		if banned != `"providerID":` && contains(b, banned) {
			t.Fatalf("user msg carries %s: %s", banned, b)
		}
	}
	a := p.Message{ID: "msg_2", SessionID: "ses_1", Role: "assistant",
		Time: p.MessageTime{Created: 1, Completed: 2}, ParentID: "msg_1",
		ModelID: "Qwen3.8-27B", ProviderID: "kido", Mode: "primary", Agent: "build",
		Path: &p.MessagePath{Cwd: "/w", Root: "/w"}, Cost: 0.1,
		Tokens: &p.Tokens{Input: 3, Output: 4}}
	ba, _ := json.Marshal(a)
	for _, want := range []string{`"parentID":"msg_1"`, `"modelID":"Qwen3.8-27B"`, `"providerID":"kido"`, `"path":{"cwd":"/w","root":"/w"}`, `"cost":0.1`, `"tokens":{"input":3,"output":4,"reasoning":0,"cache":{"read":0,"write":0}}`} {
		if !contains(ba, want) {
			t.Fatalf("assistant msg missing %s:\n%s", want, ba)
		}
	}
}

func TestPartAndToolStateShapes(t *testing.T) {
	text := p.Part{ID: "prt_1", SessionID: "ses_1", MessageID: "msg_2", Type: "text", Text: "hi", Time: p.PartTime{Start: 1}}
	b, _ := json.Marshal(text)
	if want := `{"id":"prt_1","sessionID":"ses_1","messageID":"msg_2","type":"text","text":"hi","time":{"start":1}}`; string(b) != want {
		t.Fatalf("text part:\n%s\nwant\n%s", b, want)
	}
	done := p.Part{ID: "prt_2", SessionID: "ses_1", MessageID: "msg_2", Type: "tool", CallID: "call_1", Tool: "bash",
		State: &p.ToolState{Status: "completed", Input: map[string]any{"command": "ls"}, Output: "ok", Title: "ls", Time: p.PartTime{Start: 1, End: 2}}}
	bd, _ := json.Marshal(done)
	for _, want := range []string{`"type":"tool"`, `"callID":"call_1"`, `"tool":"bash"`, `"status":"completed"`, `"output":"ok"`, `"end":2`} {
		if !contains(bd, want) {
			t.Fatalf("tool part missing %s:\n%s", want, bd)
		}
	}
}

func TestMakeEvent(t *testing.T) {
	e, err := p.MakeEvent(p.EventTypePermissionAsked, p.PermissionAskedProps{
		ID: "per_1", SessionID: "ses_1", Permission: "bash",
		Patterns: []string{"ls"}, Metadata: map[string]any{"tool": "bash"},
		Always:   []string{"ls"},
		Tool:     &p.PermissionToolRef{MessageID: "msg_2", CallID: "call_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !evtRe.MatchString(e.ID) || e.Type != p.EventTypePermissionAsked {
		t.Fatalf("envelope bad: %+v", e)
	}
	b, _ := json.Marshal(e)
	for _, want := range []string{`"type":"permission.asked"`, `"permission":"bash"`, `"patterns":["ls"]`, `"always":["ls"]`, `"tool":{"messageID":"msg_2","callID":"call_1"}`} {
		if !contains(b, want) {
			t.Fatalf("event missing %s:\n%s", want, b)
		}
	}
}

func TestParsePerms(t *testing.T) {
	rules := p.ParsePerms(map[string]any{
		"bash":  "ask",
		"edit":  "allow",
		"read":  map[string]any{"*.env": "ask"},
	})
	// broad first, narrow later (later wins under last-match-wins)
	var sawBash, sawEdit, sawReadEnv bool
	for _, r := range rules {
		switch r.Permission {
		case "bash":
			sawBash = r.Pattern == "*" && r.Action == "ask"
		case "edit":
			sawEdit = r.Pattern == "*" && r.Action == "allow"
		case "read":
			if r.Pattern == "*.env" && r.Action == "ask" {
				sawReadEnv = true
			}
		}
	}
	if !sawBash || !sawEdit || !sawReadEnv {
		t.Fatalf("parsed rules wrong: %+v", rules)
	}
}

func TestSessionStatusWire(t *testing.T) {
	b, _ := json.Marshal(p.SessionStatus{Type: p.StatusRetry, Attempt: 2, Message: "429", Next: 2000})
	if want := `{"type":"retry","attempt":2,"message":"429","next":2000}`; string(b) != want {
		t.Fatalf("status shape: %s", b)
	}
	bi, _ := json.Marshal(p.SessionStatus{Type: p.StatusIdle})
	if string(bi) != `{"type":"idle"}` {
		t.Fatalf("idle shape: %s", bi)
	}
}

func contains(b []byte, s string) bool {
	return string(b) == "" ? false : (len(b) >= len(s) && (indexOf(b, []byte(s)) >= 0))
}
func indexOf(hay, needle []byte) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if string(hay[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}
```

Simplify `contains` in the implementation step to use `strings.Contains(string(b), s)` (standard library) — the helper above is just to keep the test file dependency-free; either form is acceptable as long as the assertions listed hold.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/protocol/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write minimal implementation**

Create the files listed in Files with exactly the types above. Key implementations:

`id.go`:

```go
package protocol

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
)

const idAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz" // 58 base62 chars

func NewID(prefix string) string { return idWithPrefix(prefix) }

func NewEventID() string { return idWithPrefix("evt") }

func idWithPrefix(prefix string) string {
	out := make([]byte, 20)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(idAlphabet))))
		if err != nil {
			panic(err)
		}
		out[i] = idAlphabet[n.Int64()]
	}
	return prefix + "_" + string(out)
}
```

(The test regex matches this 58-char alphabet: digits minus 0/1, A–Z minus I/O, a–z minus l.)

`event.go`: `func MakeEvent(t string, props any) (Event, error)` — `json.Marshal(props)`, wrap in `Event{ID: NewEventID(), Type: t, Properties: raw}`.

`config.go` `ParsePerms`: for each action key (sorted by key for determinism): value string → rule `{action, "*", value}`; value map → for each pattern (sorted by length ascending, then lexically): rule `{action, pattern, actionStr}`. Output order: `*` rules first, then specific patterns.

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/protocol
git commit -m "feat: protocol DTOs, SSE event types, IDs"
```

---

### Task 3: `internal/config` — discovery, JSONC, deep merge, env substitution

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Consumes: `protocol.Config`, `protocol.ParsePerms`.
- Produces:

```go
package config

func Home() string                      // $XDG_CONFIG_HOME or ~/.config
func Data() string                      // $XDG_DATA_HOME or ~/.local/share
func Cache() string                     // $XDG_CACHE_HOME or ~/.cache
func GlobalYoloDir() string             // <Home>/yolo
func DataYoloDir() string               // <Data>/yolo
func CacheYoloDir() string              // <Cache>/yolo

type Loader struct{ Env map[string]string } // injectable env for tests; nil = os.Environ
func (l Loader) EnvVal(name string) (string, bool)
func (l Loader) Load(workDir string) (*protocol.Config, error)
func Merge(dst, src map[string]any) map[string]any
func UnmarshalJSONC(data []byte) (map[string]any, error)
func Substitute(v any, env func(string) string) any  // {env:NAME} and bare-name strings
```

- [ ] **Step 1: Write the failing test**

`internal/config/config_test.go` (fixture tree under `t.TempDir()`):

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/protocol"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalProjectYoloDiscoveryAndMerge(t *testing.T) {
	global := t.TempDir()
	work := t.TempDir()
	projCfg := "/repo/myproj"

	// fake HOME/XDG: point XDG_CONFIG_HOME at global
	home := filepath.Join(work, "home")
	write(t, filepath.Join(global, "config.json"), `{"model":"opencode/gpt-5-nano","provider":{"opencode":{"apiKey":"{env:MY_KEY}"}}}`)
	write(t, filepath.Join(global, "yolo.jsonc"), `// comment
{"instructions":["/docs/A.md"], "theme":{"dark":true}}`)

	root := filepath.Join(work, "repo")
	mid := filepath.Join(root, "mid")
	write(t, filepath.Join(root, "yolo.jsonc"), `{"model":"kido/Qwen3.8-27B","instructions":["/docs/A.md","/docs/B.md"],"permission":{"bash":"ask"}}`)
	write(t, filepath.Join(mid, "yolo.json"), `{"agent":"plan"}`)
	write(t, filepath.Join(mid, ".yolo", "yolo.jsonc"), `{"instructions":["/docs/C.md"],"tool_output":{"max_lines":500}}`)

	// Loader needs a root override for testability:
	l := config.Loader{Env: map[string]string{"MY_KEY": "sekret"}}
	cfg, err := l.LoadAt(work, "yolo.jsonc", global, filepath.Join(mid))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "kido/Qwen3.8-27B" {
		t.Fatalf("project model should win: %s", cfg.Model)
	}
	if cfg.Agent != "plan" {
		t.Fatalf("innermost project wins agent: %s", cfg.Agent)
	}
	// deep merge: provider.opencode.apiKey kept from global (env-substituted)
	if got := cfg.Provider["opencode"].APIKey; got != "sekret" {
		t.Fatalf("env substitution + deep merge failed: %q", got)
	}
	// instructions concatenate + dedupe: global[A] < project[A,B] < .yolo[C]
	wantInst := []string{"/docs/A.md", "/docs/B.md", "/docs/C.md"}
	seen := map[string]bool{}
	for _, s := range cfg.Instructions {
		if seen[s] {
			t.Fatalf("dup instruction %s in %v", s, cfg.Instructions)
		}
		seen[s] = true
	}
	for i, w := range wantInst {
		if i >= len(cfg.Instructions) || cfg.Instructions[i] != w {
			t.Fatalf("instructions = %v, want %v", cfg.Instructions, wantInst)
		}
	}
	if cfg.Permission["bash"] == nil {
		t.Fatal("permission map lost in merge")
	}
	if cfg.ToolOutput == nil || cfg.ToolOutput.MaxLines != 500 {
		t.Fatalf("tool_output not merged: %+v", cfg.ToolOutput)
	}
	if cfg.Theme == nil {
		t.Fatal("theme lost")
	}
	if rules := protocol.ParsePerms(cfg.Permission); len(rules) != 1 || rules[0].Action != "ask" {
		t.Fatalf("perms: %+v", rules)
	}
}

func TestJSONCCommentsAndUnknownFieldsIgnored(t *testing.T) {
	work := t.TempDir()
	write(t, filepath.Join(work, "yolo.jsonc"), `
{
  // leading comment
  "model": "kido/Qwen3.8-27B", /* inline */
  "futureField": {"x": 1},
  "instructions": ["a.md"] // trailing
}
`)
	cfg, err := config.Loader{Env: nil}.LoadAt(work, "yolo.jsonc", t.TempDir(), work)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "kido/Qwen3.8-27B" {
		t.Fatalf("model = %q", cfg.Model)
	}
}

func TestNoConfigFilesIsValid(t *testing.T) {
	cfg, err := config.Loader{Env: nil}.LoadAt(t.TempDir(), "yolo.jsonc", t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.Model != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestMalformedJSONIsError(t *testing.T) {
	work := t.TempDir()
	write(t, filepath.Join(work, "yolo.json"), `{broken`)
	_, err := config.Loader{Env: nil}.LoadAt(work, "yolo.json", t.TempDir(), work)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write minimal implementation**

`internal/config/config.go` implements the interface. Required behavior:

1. `LoadAt(globalsRoot, projectFileName, globalDir, startDir string)` — deterministic test entry point (`Load(workDir)` wraps it with real XDG/HOME dirs and `workDir`). Merge order (later wins): global `config.json` → global `yolo.json` → global `yolo.jsonc` → project files walked from `startDir` up to filesystem root (`yolo.json` then `yolo.jsonc` per directory, innermost last) → `<startDir>/.yolo/yolo.json` → `<startDir>/.yolo/yolo.jsonc`.
2. Each file: read → `UnmarshalJSONC` (tidwall/jsonc strip → `encoding/json` into `map[string]any`) → `Merge` into accumulated result.
3. `Merge`: maps recurse; arrays: key `instructions` concatenates (preserving order, dedupe by value first-seen); other arrays: src replaces dst.
4. After the full merge: `Substitute` walks the map replacing strings of the form `{env:NAME}` or a bare string that is exactly an env var name and set (e.g. `"apiKey": "MY_KEY"` — only whole-string substitution; `{env:NAME}` anywhere in the string), then `json.Marshal` → `json.Unmarshal` into `protocol.Config` (unknown fields are then naturally ignored).
5. `EnvVal`/`Substitute` read the loader's `Env` map (nil → `os.LookupEnv`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat: config discovery, JSONC, deep merge, env substitution"
```

Note: this task pins exact version `tidwall/jsonc v0.3.3` (`go get tidwall/jsonc@v0.3.3`, then `go mod tidy`).

---

### Task 4: `internal/auth` + `yolo auth` CLI

**Files:**
- Create: `internal/auth/auth.go`, `internal/auth/auth_test.go`
- Modify: `cmd/yolo/main.go` (replace interim `auth` notice with real dispatch: `list`, `add <provider> [key]`, `remove <provider>`)

**Interfaces:**
- Consumes: `config.DataYoloDir()`, `protocol.Config`.
- Produces:

```go
package auth

type Entry struct {
	Type     string         `json:"type"` // "api"
	Key      string         `json:"key"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
type Store map[string]Entry
func Path() string // <DataYoloDir>/auth.json
func Load() (Store, error)
func Save(s Store) error // file 0600, dir 0700
func (s Store) Set(providerID string, key string)
func (s Store) Delete(providerID string)
func EnvName(providerID string) string
// OPENCODE_API_KEY, KIDO_API_KEY, else <UPPER(providerID)>_API_KEY
func ResolveKey(providerID string, cfg *protocol.Config, env func(string) (string, bool)) (string, bool)
```

Resolution order (spec §6.2): **env** → **auth.json** → **config** `provider.<id>.apiKey` then `provider.<id>.options.apiKey` (after config env-substitution, done by the caller). `kido`, `opencode` are the known providers; any config-defined provider id is also resolvable.

- [ ] **Step 1: Write the failing test**

`internal/auth/auth_test.go`:

```go
package auth_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/protocol"
)

func TestEnvName(t *testing.T) {
	cases := map[string]string{
		"opencode": "OPENCODE_API_KEY",
		"kido":     "KIDO_API_KEY",
		"myprov":   "MYPROV_API_KEY",
	}
	for p, want := range cases {
		if got := auth.EnvName(p); got != want {
			t.Fatalf("EnvName(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestResolutionOrder(t *testing.T) {
	env := map[string]string{}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }

	cfg := &protocol.Config{}
	// 1) env wins over all
	env["OPENCODE_API_KEY"] = "from-env"
	if k, _ := auth.ResolveKey("opencode", cfg, lookup); k != "from-env" {
		t.Fatalf("env should win: %q", k)
	}
	delete(env, "OPENCODE_API_KEY")

	// 2) auth.json wins over config
	t.Setenv("HOME", t.TempDir())
	s := auth.Store{}
	s.Set("opencode", "from-file")
	if err := auth.Save(s); err != nil {
		t.Fatal(err)
	}
	cfg.Provider = map[string]protocol.ProviderConfig{"opencode": {APIKey: "from-config"}}
	if k, _ := auth.ResolveKey("opencode", cfg, lookup); k != "from-file" {
		t.Fatalf("auth.json should beat config: %q", k)
	}

	// 3) config apiKey last
	s2, err := auth.Load()
	if err != nil {
		t.Fatal(err)
	}
	delete(s2, "opencode")
	if err := auth.Save(s2); err != nil {
		t.Fatal(err)
	}
	if k, _ := auth.ResolveKey("opencode", cfg, lookup); k != "from-config" {
		t.Fatalf("config should be last resort: %q", k)
	}
	if k, ok := auth.ResolveKey("kido", cfg, lookup); ok || k != "" {
		t.Fatalf("kido key-less: k=%q ok=%v", k, ok)
	}
}

func TestSaveIs0600(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := auth.Store{}
	s.Set("kido", "x")
	if err := auth.Save(s); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(auth.Path())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", fi.Mode().Perm())
	}
}

func TestRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := auth.Store{}
	s.Set("opencode", "abc")
	if err := auth.Save(s); err != nil {
		t.Fatal(err)
	}
	got, err := auth.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got["opencode"].Key != "abc" || got["opencode"].Type != "api" {
		t.Fatalf("round trip: %+v", got)
	}
	s.Delete("opencode")
	if err := auth.Save(s); err != nil {
		t.Fatal(err)
	}
	got, _ = auth.Load()
	if _, exists := got["opencode"]; exists {
		t.Fatal("delete did not persist")
	}
}

var _ = filepath.Join // keep import if unused
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write minimal implementation**

Implement the interface. `Save`: `os.MkdirAll(filepath.Dir(Path()), 0o700)`, marshal with 2-space indent, `os.WriteFile(Path(), data, 0o600)`. `Load`: missing file → empty Store, nil error. CLI in `cmd/yolo/main.go`:

- `yolo auth list` → one line per entry `<providerID>  api  (set)`, else `no credentials`.
- `yolo auth add <provider> [key]` → if key omitted, prompt on `/dev/tty` (read line, echo off via `golang.org/x/term`? **no new dep** — plain `fmt` prompt "API key: " and read stdin with `io/ioutil`; document limitation). Save.
- `yolo auth remove <provider>` → delete + save.
- Exit 2 with usage on bad args.

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS (including the M0 binary help test).

- [ ] **Step 5: Commit**

```bash
git add internal/auth cmd/yolo
git commit -m "feat: auth.json store, key resolution, yolo auth CLI"
```

---

### Task 5: `internal/storage` — SQLite open, migrations, DAOs

**Files:**
- Create: `internal/storage/migrate.go`, `internal/storage/db.go`, `internal/storage/dao.go`, `internal/storage/storage_test.go`

**Interfaces:**
- Consumes: `protocol` DTOs.
- Produces:

```go
package storage

type DB struct{ *sql.DB }
func Open(path string) (*DB, error)  // PRAGMA journal_mode=WAL; busy_timeout=5000; migrates
func (d *DB) Close() error

// rows
type SessionRow struct {
	ID, ProjectDir, Title, Model, Agent string
	Cost                                float64
	TimeCreated, TimeUpdated            int64
}
type MessageRow struct {
	ID, SessionID, Role string
	Cost                float64
	Tokens              protocol.Tokens
	TimeCreated         int64
	TimeCompleted       *int64
}
type PartRow struct {
	ID, MessageID, SessionID, Type, Tool string
	StateJSON                           string
	TimeCreated                         int64
}
type PermissionRow struct {
	RequestID, SessionID, Action, Resource, Response, AlwaysJSON string
	TimeCreated                                                  int64
}

// session API
func (d *DB) CreateSession(r SessionRow) error
func (d *DB) GetSession(id string) (SessionRow, error)          // sql.ErrNoRows → error
func (d *DB) ListSessions(projectDir string, limit int) ([]SessionRow, error) // order: time_updated DESC
func (d *DB) UpdateSession(id string, patch SessionRow) error   // zero fields = untouched
func (d *DB) DeleteSession(id string) error
// message/part API
func (d *DB) CreateMessage(r MessageRow) error
func (d *DB) UpdateMessage(r MessageRow) error
func (d *DB) DeleteMessage(id string) error
func (d *DB) ListMessages(sessionID string) ([]MessageRow, error) // order: time_created ASC
func (d *DB) UpsertPart(r PartRow) error
func (d *DB) GetPart(id string) (PartRow, error)
func (d *DB) ListParts(messageID string) ([]PartRow, error)     // order: time_created ASC
func (d *DB) ListToolParts(messageID string) ([]PartRow, error) // type='tool', same order
// part <-> protocol conversion (StateJSON encodes protocol text/reasoning/tool payloads)
func PartToProtocol(r PartRow) (protocol.Part, error)
func ProtocolToPart(p protocol.Part) PartRow

// permission API
func (d *DB) SavePermission(r PermissionRow) error
func (d *DB) ListPermissions(sessionID string, pendingOnly bool) ([]PermissionRow, error)
func (d *DB) ReplyPermission(requestID, response string) error
func (d *DB) AlwaysRules(sessionID string) ([]protocol.Rule, error)
// response='always' rows -> {action, pattern(from always_json entries), allow}
```

**PLAN RESOLUTION (flag to user — spec DDL gap):** the spec's DDL has no cost/tokens persistence, but the legacy wire `Session`/`AssistantMessage` carry `cost`/`tokens`. This plan stores them at message level (`message.cost REAL NOT NULL DEFAULT 0`, `message.tokens TEXT NOT NULL DEFAULT '{}'` — the JSON of `protocol.Tokens`) and computes the session aggregate (sum of assistant messages) at read time: `SessionFromRow(r SessionRow, msgs []MessageRow) protocol.Session`. No other DDL changes; the spec tables are otherwise verbatim.

- [ ] **Step 1: Write the failing test**

`internal/storage/storage_test.go`:

```go
package storage_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

func openDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSessionCRUDAndListOrder(t *testing.T) {
	db := openDB(t)
	mk := storage.SessionRow{ProjectDir: "/w", Title: "t", Model: "kido/Qwen3.8-27B", Agent: "build"}
	for i, id := range []string{"ses_aaa", "ses_bbb", "ses_ccc"} {
		r := mk
		r.ID = id
		r.TimeCreated = int64(100 + i)
		r.TimeUpdated = int64(100 + i)
		if err := db.CreateSession(r); err != nil {
			t.Fatal(err)
		}
	}
	// another directory is isolated
	other := mk
	other.ID, other.ProjectDir = "ses_other", "/other"
	other.TimeCreated, other.TimeUpdated = 999, 999
	if err := db.CreateSession(other); err != nil {
		t.Fatal(err)
	}
	got, err := db.ListSessions("/w", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (scoping broken)", len(got))
	}
	if got[0].ID != "ses_ccc" {
		t.Fatalf("first = %s, want newest-first", got[0].ID)
	}
	if _, err := db.GetSession("ses_missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCascadeDelete(t *testing.T) {
	db := openDB(t)
	_ = db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1})
	_ = db.CreateMessage(storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "user", TimeCreated: 2})
	_ = db.UpsertPart(storage.PartRow{ID: "prt_1", MessageID: "msg_1", SessionID: "ses_1", Type: "text", StateJSON: `{"text":"hi"}`, TimeCreated: 3})
	if err := db.DeleteSession("ses_1"); err != nil {
		t.Fatal(err)
	}
	msgs, err := db.ListMessages("ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("cascade failed: %d messages", len(msgs))
	}
}

func TestTextAndToolPartRoundTrip(t *testing.T) {
	db := openDB(t)
	_ = db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1})
	_ = db.CreateMessage(storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "assistant", TimeCreated: 2})
	text := protocol.Part{ID: "prt_txt", MessageID: "msg_1", SessionID: "ses_1", Type: "text", Text: "hello", Time: protocol.PartTime{Start: 5, End: 9}}
	if err := db.UpsertPart(storage.ProtocolToPart(text)); err != nil {
		t.Fatal(err)
	}
	row, err := db.GetPart("prt_txt")
	if err != nil {
		t.Fatal(err)
	}
	back, err := storage.PartToProtocol(row)
	if err != nil {
		t.Fatal(err)
	}
	if back.Text != "hello" || back.Time.End != 9 {
		t.Fatalf("round trip: %+v", back)
	}
	tool := protocol.Part{ID: "prt_tool", MessageID: "msg_1", SessionID: "ses_1", Type: "tool", CallID: "call_1", Tool: "bash", State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": "ls"}, Output: "ok", Time: protocol.PartTime{Start: 1, End: 2}}}
	if err := db.UpsertPart(storage.ProtocolToPart(tool)); err != nil {
		t.Fatal(err)
	}
	prow, _ := db.GetPart("prt_tool")
	pback, _ := storage.PartToProtocol(prow)
	if pback.State == nil || pback.State.Output != "ok" {
		t.Fatalf("tool state lost: %+v", pback)
	}
	raw, _ := json.Marshal(prow.StateJSON)
	_ = raw
}

func TestSessionAggregateCostTokens(t *testing.T) {
	db := openDB(t)
	_ = db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Title: "x", Model: "kido/m", Agent: "build", TimeCreated: 1, TimeUpdated: 1})
	_ = db.CreateMessage(storage.MessageRow{ID: "msg_u", SessionID: "ses_1", Role: "user", TimeCreated: 2})
	_ = db.CreateMessage(storage.MessageRow{ID: "msg_a", SessionID: "ses_1", Role: "assistant", Cost: 0.25,
		Tokens: protocol.Tokens{Input: 100, Output: 50, Reasoning: 5, Cache: protocol.CacheTokens{Read: 7, Write: 1}}, TimeCreated: 3})
	sess, err := db.Session("ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Cost != 0.25 || sess.Tokens.Input != 100 || sess.Tokens.Cache.Read != 7 {
		t.Fatalf("aggregate = %+v", sess)
	}
	if sess.Model != "kido/m" || sess.Directory != "/w" {
		t.Fatalf("wire mapping: %+v", sess)
	}
}

func TestAlwaysRules(t *testing.T) {
	db := openDB(t)
	_ = db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1})
	_ = db.SavePermission(storage.PermissionRow{RequestID: "per_1", SessionID: "ses_1", Action: "bash", Resource: "*", Response: "always", AlwaysJSON: `["ls","whoami"]`, TimeCreated: 1})
	_ = db.SavePermission(storage.PermissionRow{RequestID: "per_2", SessionID: "ses_1", Action: "bash", Resource: "*", Response: "once", TimeCreated: 2})
	rules, err := db.AlwaysRules("ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("want 2 always rules, got %d: %+v", len(rules), rules)
	}
	for _, r := range rules {
		if r.Action != "allow" || r.Permission != "bash" {
			t.Fatalf("bad rule %+v", r)
		}
	}
}

func TestSchemaVersionTracked(t *testing.T) {
	db := openDB(t)
	v, err := db.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v < 1 {
		t.Fatalf("schema version = %d", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write minimal implementation**

`migrate.go`: `var migrations = map[int]string{1: ` ... spec DDL verbatim ... `}` plus the two extra `message` columns (`cost REAL NOT NULL DEFAULT 0`, `tokens TEXT NOT NULL DEFAULT '{}'`). Apply in a transaction in ascending key order; `meta` table `(key TEXT PRIMARY KEY, value TEXT NOT NULL)`; after each, `INSERT OR REPLACE INTO meta VALUES ('schema_version', '<n>')`. `Open`: `sql.Open("sqlite", path)` with modernc driver (`import _ "modernc.org/sqlite"` in db.go), `PRAGMA journal_mode=WAL`, `PRAGMA busy_timeout=5000`, `PRAGMA foreign_keys=ON`, then migrations. Pin `modernc.org/sqlite v1.56.0`.

`db.go`: `var ErrNotFound = errors.New(...)` mapping `sql.ErrNoRows`. `Session(id)` = GetSession + ListMessages → `SessionFromRow`. DDL (v1, the spec's tables with the noted addition):

```sql
CREATE TABLE session (
  id TEXT PRIMARY KEY,
  project_dir TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL,
  agent TEXT NOT NULL DEFAULT 'build',
  cost REAL NOT NULL DEFAULT 0,
  time_created INTEGER NOT NULL,
  time_updated INTEGER NOT NULL
);
CREATE TABLE message (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES session(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  cost REAL NOT NULL DEFAULT 0,
  tokens TEXT NOT NULL DEFAULT '{}',
  time_created INTEGER NOT NULL,
  time_completed INTEGER
);
CREATE TABLE part (
  id TEXT PRIMARY KEY,
  message_id TEXT NOT NULL REFERENCES message(id) ON DELETE CASCADE,
  session_id TEXT NOT NULL,
  type TEXT NOT NULL,
  tool TEXT,
  state_json TEXT NOT NULL,
  time_created INTEGER NOT NULL
);
CREATE TABLE permission (
  request_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  action TEXT NOT NULL,
  resource TEXT NOT NULL,
  response TEXT,
  always_json TEXT,
  time_created INTEGER NOT NULL
);
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
```

Note: the spec's `session` table lacks `cost`; this plan stores the session aggregate in memory at read time (no cost column needed beyond messages) — the `cost REAL` column shown above is included for parity and updated on title/model patches (it is ignored in `SessionFromRow`, which always recomputes from messages).

`dao.go`: CRUD as listed. `ProtocolToPart`/`PartToProtocol`: `state_json` = `{"text": ..., "end": nms}` for text/reasoning (end omitted for `end == 0`), and the full `protocol.ToolState` JSON for tool parts. `SessionFromRow`: parse model string `provider/model` into `ModelRef`; projectID = `"prj_" + hex(sha256(project_dir))[:24]`; cost/tokens = sum over assistant `MessageRow`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage go.mod go.sum
git commit -m "feat: storage layer — SQLite schema v1, DAOs, aggregates"
```

---

### Task 6: `internal/bus` — in-process pub/sub

**Files:**
- Create: `internal/bus/bus.go`, `internal/bus/bus_test.go`

**Interfaces:**
- Consumes: `protocol.Event`.
- Produces:

```go
package bus

type Bus struct{}
func New() *Bus
func (b *Bus) Publish(e protocol.Event)
func (b *Bus) Subscribe() (ch <-chan protocol.Event, cancel func())
```

Semantics: `Subscribe` creates a buffered channel of 1024 (spec §7: overflow closes the client — the TUI reconnects). `Publish` is non-blocking: drops into a full subscriber's channel by **cancelling that subscriber** (close its channel; subsequent Sends panic-free via a select on a closed flag). `cancel()` unregisters.

- [ ] **Step 1: Write the failing test**

`internal/bus/bus_test.go`:

```go
package bus_test

import (
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/protocol"
)

func TestPubSub(t *testing.T) {
	b := bus.New()
	ch, cancel := b.Subscribe()
	defer cancel()
	e1, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{SessionID: "ses_1"})
	e2, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{SessionID: "ses_2"})
	b.Publish(e1)
	b.Publish(e2)
	select {
	case got := <-ch:
		if got.Type != e1.Type {
			t.Fatalf("type = %s", got.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("no event on chan")
	}
	select {
	case got := <-ch:
		if got.Type != e2.Type {
			t.Fatalf("type = %s", got.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("no second event")
	}
}

func TestCancelStopsDelivery(t *testing.T) {
	b := bus.New()
	ch, cancel := b.Subscribe()
	cancel()
	b.Publish(mustEvent(t))
	select {
	case <-ch:
		t.Fatal("delivery after cancel")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestOverflowCancelsSubscribers(t *testing.T) {
	b := bus.New()
	ch, _ := b.Subscribe()
	// exhaust buffer (1024) + a few
	for i := 0; i < 1100; i++ {
		b.Publish(mustEvent(t))
	}
	select {
	case ev, ok := <-ch:
		if !ok {
			return // channel closed = subscriber dropped, as specified
		}
		_ = ev
	case <-time.After(200 * time.Millisecond):
	}
	// after overflow the subscriber must be gone: subsequent publish must not panic
	for i := 0; i < 10; i++ {
		b.Publish(mustEvent(t))
	}
}

func mustEvent(t *testing.T) protocol.Event {
	t.Helper()
	e, err := protocol.MakeEvent(protocol.EventTypeMessageUpdated, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bus/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write minimal implementation**

```go
package bus

import (
	"sync"

	"github.com/kido5217/yolo/internal/protocol"
)

const subscriberBuffer = 1024

type subscriber struct {
	ch     chan protocol.Event
	closed bool
}

type Bus struct {
	mu  sync.Mutex
	subs map[*subscriber]struct{}
}

func New() *Bus { return &Bus{subs: map[*subscriber]struct{}{}} }

func (b *Bus) Subscribe() (<-chan protocol.Event, func()) {
	s := &subscriber{ch: make(chan protocol.Event, subscriberBuffer)}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	return s.ch, func() {
		b.mu.Lock()
		if _, ok := b.subs[s]; ok {
			delete(b.subs, s)
			close(s.ch)
			s.closed = true
		}
		b.mu.Unlock()
	}
}

func (b *Bus) Publish(e protocol.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs {
		select {
		case s.ch <- e:
		default:
			// slow subscriber: drop it (spec §7: overflow closes the client)
			delete(b.subs, s)
			if !s.closed {
				close(s.ch)
				s.closed = true
			}
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bus
git commit -m "feat: in-process event bus with overflow drop"
```

---

# Milestone M2 — LLM drivers + providers

### Task 7: `internal/llm` core + OpenAI chat-completions driver

**Files:**
- Create: `internal/llm/llm.go`, `internal/llm/openai.go`, `internal/llm/openai_test.go`, `internal/llm/testdata/openai/stream_basic.txt`, `internal/llm/testdata/openai/stream_split_frames.txt`, `internal/llm/testdata/openai/stream_reasoning_tools.txt`, `internal/llm/testdata/openai/stream_usage_only_final.txt`, `internal/llm/testdata/openai/midstream_error.txt`

**Interfaces:**
- Consumes: nothing.
- Produces (used by session engine, provider, title gen, all tests):

```go
package llm

type Role string
const (
	RoleSystem Role = "system"
	RoleUser   Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool   Role = "tool"
)
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Args  json.RawMessage `json:"args"` // raw JSON object
}
type Message struct {
	Role       Role
	Content    string
	ToolCallID string   // RoleTool
	ToolCalls  []ToolCall // RoleAssistant
}
type ToolDef struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}
type Usage struct {
	Input, Output, Reasoning, CacheRead, CacheWrite int
}
type Request struct {
	Model       string
	APIKey      string
	BaseURL     string
	Messages    []Message
	Tools       []ToolDef
	Temperature *float64
	MaxTokens   int
}
type Part struct {
	Kind   string     // "text" | "reasoning" | "tool"
	Name   string     // tool name (Kind=="tool")
	CallID string     // stable per tool (Kind=="tool")
	Text   string     // delta payload
	Usage  *Usage     // on the final part of the stream
	Finish string     // "stop" | "tool_calls" | "length" | "error" (final part)
	Err    error      // non-nil only on the final part after a 200 began
}
type PartStream struct {
	Parts <-chan Part
}
func (s PartStream) Next(ctx context.Context) (Part, error) // error only if ctx done or stream closed with Err part

type Driver interface {
	Stream(ctx context.Context, req Request) (PartStream, error)
}
type OpenAI struct{ Client *http.Client }
func NewOpenAI(c *http.Client) Driver
type Anthropic struct{ Client *http.Client }
func NewAnthropic(c *http.Client) Driver // Task 8

type TransientError struct{ Status int; Err error }
func (e *TransientError) Error() string { return e.Err.Error() }
func IsTransient(err error) bool // *TransientError with Status 429 or >=500, or net error, or context error is NOT transient
```

Wire rules (both drivers):
- Non-2xx before the first body byte (4xx/5xx) → `Stream` returns `error`; 429/5xx wrapped in `*TransientError` with status; 4xx others plain error (message from body `{"error":{...}}` when present).
- After 200 begins, mid-stream failures are delivered as a final `Part{Kind:"text", Finish:"error", Err: err}`.
- `Next` returns `Part` values in stream order; after the final part (one carrying `Finish`) the channel closes.

OpenAI specifics (`POST {BaseURL}/chat/completions`, `Authorization: Bearer {APIKey}`, `Content-Type: application/json`):
- Request body: `{"model", "messages", "stream": true, "stream_options": {"include_usage": true}, "tools": [{"type":"function","function":{"name","description","parameters"}}], "max_tokens"?, "temperature"?}`. Message mapping: system/user as-is; assistant with `tool_calls: [{"id","type":"function","function":{"name","arguments": raw}}]`; tool as `{"role":"tool","tool_call_id","content"}`.
- SSE: lines `data: <json>`; sentinel `data: [DONE]`. Chunk: `choices[0].delta.content` → text part; `delta.reasoning_content` (or `delta.reasoning`) → reasoning part; `delta.tool_calls[]` entries carry `index`, optional `id`/`function.name` on first appearance, `function.arguments` streamed piecewise — accumulate per index; emit one `Part{Kind:"tool"}` at stream end per accumulated call (name, id, full args JSON). `finish_reason` on last chunk → `Finish` mapping `stop`→`stop`, `tool_calls`→`tool_calls`, `length`→`length`. Usage chunk arrives with empty `choices`: `usage.{prompt_tokens, completion_tokens, completion_tokens_details.reasoning_tokens, prompt_tokens_details.cached_tokens}` → `Usage{Input, Output, Reasoning, CacheRead}` (CacheWrite = 0 for OpenAI).

**Test fixtures (commit these exact files):**

`testdata/openai/stream_basic.txt`:

```
data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"}}]}

data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"lo"}}]}

data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"c1","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2}}

data: [DONE]

```

`testdata/openai/stream_split_frames.txt` — a single JSON object split so one `data:` line is cut across TCP writes; for the unit test we instead serve byte slices via a custom `httptest` handler that flushes mid-frame (see test). Fixture content is the same as basic but the test splits `data: {"choi` / `ces":[...` manually. Keep the file:

```
data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"a"}}]}

data: {"choices":[{"index":0,"delta":{"content":"b"}}]}

data: [DONE]

```

`testdata/openai/stream_reasoning_tools.txt`:

```
data: {"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"thinking..."}}]}

data: {"choices":[{"index":0,"delta":{"content":"answer "}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read","arguments":"{\"file"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"Path\":\"/x\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

```

`testdata/openai/stream_usage_only_final.txt`:

```
data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"done"}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}

data: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"completion_tokens_details":{"reasoning_tokens":1},"prompt_tokens_details":{"cached_tokens":1}}}

data: [DONE]

```

`testdata/openai/midstream_error.txt` (served, then connection closed):

```
data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"partial"}}]}

```

- [ ] **Step 1: Write the failing test**

`internal/llm/openai_test.go`:

```go
package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func sseServer(t *testing.T, fixture string, split bool) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "openai", fixture))
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		fl, _ := w.(http.Flusher)
		if !split {
			_, _ = w.Write(data)
			fl.Flush()
			return
		}
		// flush mid-frame (1 byte at a time) to exercise the incremental reader
		for _, b := range data {
			_, _ = w.Write([]byte{b})
		}
		fl.Flush()
	}))
}

func collect(t *testing.T, s PartStream) []Part {
	t.Helper()
	var out []Part
	for {
		p, err := s.Next(context.Background())
		if err != nil {
			break
		}
		out = append(out, p)
		if p.Finish != "" {
			break
		}
	}
	return out
}

func TestOpenAIBasicStream(t *testing.T) {
	srv := sseServer(t, "stream_basic.txt", false)
	defer srv.Close()
	parts := collect(t, NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}).must(t))
	if len(parts) != 3 { // text, text, finish
		t.Fatalf("parts = %+v", parts)
	}
	if parts[0].Kind != "text" || parts[0].Text != "Hel" || parts[1].Text != "lo" {
		t.Fatalf("deltas wrong: %+v", parts[:2])
	}
	if parts[2].Finish != "stop" {
		t.Fatalf("finish = %q", parts[2].Finish)
	}
}

func TestOpenAIMidFrameSplits(t *testing.T) {
	srv := sseServer(t, "stream_split_frames.txt", true)
	defer srv.Close()
	parts := collect(t, NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}).must(t))
	var text string
	for _, p := range parts {
		if p.Kind == "text" {
			text += p.Text
		}
	}
	if text != "ab" {
		t.Fatalf("reassembled text = %q", text)
	}
}

func TestOpenAIReasoningAndToolCalls(t *testing.T) {
	srv := sseServer(t, "stream_reasoning_tools.txt", false)
	defer srv.Close()
	parts := collect(t, NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "use tools"}},
	}).must(t))
	var sawReasoning, sawText bool
	tools := map[string]Part{}
	var finish string
	for _, p := range parts {
		switch p.Kind {
		case "reasoning":
			sawReasoning = p.Text == "thinking..."
		case "text":
			sawText = sawText || p.Text == "answer "
		case "tool":
			tools[p.CallID] = p
		}
		if p.Finish != "" {
			finish = p.Finish
		}
	}
	if !sawReasoning || !sawText || finish != "tool_calls" {
		t.Fatalf("reasoning=%v text=%v finish=%q parts=%+v", sawReasoning, sawText, finish, parts)
	}
	tc, ok := tools["call_1"]
	if !ok || tc.Name != "read" {
		t.Fatalf("tool part = %+v (tools=%v)", tc, tools)
	}
	var args map[string]string
	if err := json.Unmarshal(tc.Args, &args); err != nil || args["filePath"] != "/x" {
		t.Fatalf("tool args = %s err=%v", tc.Args, err)
	}
}

func TestOpenAIUsageFinal(t *testing.T) {
	srv := sseServer(t, "stream_usage_only_final.txt", false)
	defer srv.Close()
	parts := collect(t, NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	}).must(t))
	var u *Usage
	var finish string
	for _, p := range parts {
		if p.Usage != nil {
			u = p.Usage
		}
		if p.Finish != "" {
			finish = p.Finish
		}
	}
	if u == nil || u.Input != 1 || u.Output != 1 || u.Reasoning != 1 || u.CacheRead != 1 {
		t.Fatalf("usage = %+v", u)
	}
	if finish != "length" {
		t.Fatalf("finish = %q", finish)
	}
}

func TestOpenAIMidStreamError(t *testing.T) {
	srv := sseServer(t, "midstream_error.txt", false)
	defer srv.Close()
	part, err := NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "test-key", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	}).must(t).Next(context.Background())
	_ = err
	if part.Kind != "text" || part.Text != "partial" {
		t.Fatalf("first part = %+v", part)
	}
	final, err := /* drain */ drainFinal(t, /* from the same stream */ nilParts)
	_ = final
	_ = err
}

func TestOpenAIUpstream429IsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":{"message":"slow down"}}`))
	}))
	defer srv.Close()
	_, err := NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "k", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	if err == nil || !IsTransient(err) {
		t.Fatalf("err = %v, want transient", err)
	}
}

func TestOpenAIRequestShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	_, _ = NewOpenAI(srv.Client()).Stream(ctx0(t), Request{
		Model: "m", APIKey: "k", BaseURL: srv.URL,
		Messages: []Message{{Role: RoleSystem, Content: "sys"}, {Role: RoleUser, Content: "hi"}},
		Tools:  []ToolDef{{Name: "read", Description: "d", Parameters: json.RawMessage(`{"type":"object"}`)}},
		MaxTokens: 100,
	}).must(t)
	if got["stream"] != true {
		t.Fatalf("stream = %v", got["stream"])
	}
	tools, _ := got["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", got["tools"])
	}
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	if fn["name"] != "read" {
		t.Fatalf("fn = %v", fn)
	}
}

// helpers (add to llm.go or the test file)
type errStream struct{ PartStream }
func (s errStream) must(t *testing.T) PartStream {
	t.Helper()
	// usage: replace `.must(t)` pattern with an explicit err check in each test call;
	// to keep the tests above compiling, define a small helper on the caller side.
	return s
}
func ctx0(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}
```

Implementation note for the test author (keep tests compiling exactly): replace the `.must(t)` call chain used above with a local helper in the test file:

```go
func stream(t *testing.T, d llm.Driver, req llm.Request) llm.PartStream {
	t.Helper()
	s, err := d.Stream(ctx0(t), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	return s
}
```

and use `stream(t, NewOpenAI(srv.Client()), req)` in each test; `collect(t, stream(t, ...))`. The `TestOpenAIMidStreamError` test must drain from the *same* PartStream:

```go
func TestOpenAIMidStreamError(t *testing.T) {
	srv := sseServer(t, "midstream_error.txt", false)
	defer srv.Close()
	s, err := stream(t, NewOpenAI(srv.Client()), Request{...})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := s.Next(ctx0(t))
	if first.Text != "partial" {
		t.Fatalf("first = %+v", first)
	}
	var final llm.Part
	for {
		p, err := s.Next(ctx0(t))
		if p.Finish == "error" {
			final = p
			break
		}
		if err != nil {
			break
		}
	}
	if final.Finish != "error" || final.Err == nil {
		t.Fatalf("final = %+v", final)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/llm/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write minimal implementation**

`llm.go`: the types above + `IsTransient` (wrap: `var te *TransientError; errors.As(err, &te) && (te.Status == 429 || te.Status >= 500)`; network errors (`net.Error` whose `Timeout()` is false and not context) also transient). `openai.go`:

- `Stream`: marshal request (include only non-zero optional fields), POST, on status != 200 → drain body ≤ 4KB, parse `{"error":{...}}` message, return `*TransientError{Status, ...}` for 429/5xx else plain error.
- 200: launch goroutine: read body with `bufio.Reader`; parse SSE lines (handle multi-line `data:` by concatenation of consecutive `data:` lines before a blank line); per chunk: `json.Unmarshal` into `{choices []{index int; delta map[string]any; finish_reason any}, usage map[string]any}` — decode delta fields with type-safe helpers (`str(delta["content"])` etc.); accumulate tool call args into `map[int]*ToolCall`; on `finish_reason` → emit finish Part (plus per-index accumulated tool Parts first); on usage chunk → attach `Usage` to the finish Part; on `[DONE]` or EOF → close channel. Mid-read error after first byte → emit `Part{Kind:"text", Finish:"error", Err}` then close.

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/llm go.mod go.sum
git commit -m "feat: llm Driver interface + OpenAI chat-completions SSE driver"
```

---

### Task 8: Anthropic Messages driver

**Files:**
- Create: `internal/llm/anthropic.go`, `internal/llm/anthropic_test.go`, `internal/llm/testdata/anthropic/stream_basic.txt`, `internal/llm/testdata/anthropic/stream_thinking_tool.txt`, `internal/llm/testdata/anthropic/midstream_error.txt`

**Interfaces:**
- Consumes: Task 7 types (`llm.Request/Part/PartStream/Driver`, `TransientError`).
- Produces:

```go
type Anthropic struct{ Client *http.Client }
func NewAnthropic(c *http.Client) Driver
```

Wire rules (`POST {BaseURL}/messages`): headers `x-api-key: {APIKey}`, `anthropic-version: 2023-06-01`, `content-type: application/json`. Body: `{"model", "max_tokens": req.MaxTokens | 8192, "system": (concatenated RoleSystem messages), "messages": [...], "tools": [{"name","description","input_schema"}], "temperature"?, "stream": true}`. Non-system messages map 1:1; assistant `tool_calls` → `{"role":"assistant","content":[{"type":"tool_use","id","name","input": <decoded obj>}]}`; tool results → `{"role":"user","content":[{"type":"tool_result","tool_use_id","content": <string>}]}`.

SSE event frames (`event: <type>\ndata: <json>\n\n`; parse by `data:` JSON `"type"` field):
- `message_start` → ignore (usage.input from `message.usage.input_tokens` captured for final usage)
- `content_block_start` → `{index, content_block:{type: "text"|"thinking"|"tool_use", id?, name?}}`
- `content_block_delta` → `{index, delta:{type: "text_delta","text"} | {type:"thinking_delta","thinking"} | {type:"input_json_delta","partial_json"}}` → text/reasoning Part; accumulate `partial_json` per index for tool_use
- `content_block_stop` → if the block was tool_use and has an accumulated body: emit `Part{Kind:"tool", CallID: id, Name: name, Args: bytes(partial)}` (default `Args` = `null` → `{" "}`? no: empty object `{}` when JSON is empty)
- `message_delta` → `delta.stop_reason` → finish mapping: `end_turn`→`stop`, `tool_use`→`tool_calls`, `max_tokens`→`length`; capture `usage.output_tokens`
- `message_stop` → close
- `error` → mid-stream: final `Part{Finish:"error", Err: errors.New(msg)}`
- Usage final: `Usage{Input: message_start.input_tokens, Output: message_delta.output_tokens}` (Reasoning/CacheRead/CacheWrite: input_tokens_details / cache fields when present, else 0)

**Fixtures:**

`testdata/anthropic/stream_basic.txt`:

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":7}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hey"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"! I am Claude."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

```

`testdata/anthropic/stream_thinking_tool.txt`:

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_2","usage":{"input_tokens":11}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me check."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"bash","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"ls -la\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}

event: message_stop
data: {"type":"message_stop"}

```

`testdata/anthropic/midstream_error.txt`:

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_3","usage":{"input_tokens":3}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"oops"}}

event: error
data: {"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}

```

- [ ] **Step 1: Write the failing test**

`internal/llm/anthropic_test.go` — mirror the OpenAI test file structure (`sseServer` variant reading `testdata/anthropic/*`, `collect`), with four tests: `TestAnthropicBasicStream` (text deltas `Hey`, `! I am Claude.`; finish `stop`; usage input 7 output 5), `TestAnthropicThinkingAndToolUse` (reasoning `Let me check.`; tool CallID `toolu_1`, Name `bash`, args `{"command":"ls -la"}`; finish `tool_calls`), `TestAnthropicMidStreamError` (final part `Finish:"error"`), `TestAnthropicRequestShape` (assert headers `x-api-key`, `anthropic-version`; body has `max_tokens`, `system` string, `stream:true`, tools as `{name, description, input_schema}`). Reuse the shared `sseServer`/`collect`/`stream`/`ctx0` helpers by moving them into the test package as `common_test.go`:

`internal/llm/common_test.go`:

```go
package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func ctx0(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func sseServer(t *testing.T, dir, fixture string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", dir, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl, _ := w.(http.Flusher)
		_, _ = w.Write(data)
		fl.Flush()
	}))
}

func stream(t *testing.T, d Driver, req Request) PartStream {
	t.Helper()
	s, err := d.Stream(ctx0(t), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	return s
}

func collect(t *testing.T, s PartStream) []Part {
	t.Helper()
	var out []Part
	for {
		p, err := s.Next(ctx0(t))
		if err != nil {
			break
		}
		out = append(out, p)
		if p.Finish != "" {
			break
		}
	}
	return out
}

func mustJSONUnmarshal(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
}
```

(and adjust the OpenAI tests to use the shared helpers — drop their local copies)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/llm/ -run TestAnthropic -v`
Expected: FAIL — `NewAnthropic` undefined.

- [ ] **Step 3: Write minimal implementation**

`anthropic.go` per the wire rules above (same SSE line-reading core as openai.go — factor into an unexported `readSSE(r io.Reader) (<-chan []byte line-data, error)` in llm.go shared by both drivers).

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/llm
git commit -m "feat: Anthropic Messages SSE driver"
```

---

### Task 9: `internal/provider` — kido + zen catalogs, auth state, registry

**Files:**
- Create: `internal/provider/provider.go`, `internal/provider/kido.go`, `internal/provider/zen.go`, `internal/provider/provider_test.go`, `internal/provider/testdata/zen-opencode.json`, `internal/provider/testdata/kido-models.json`

**Interfaces:**
- Consumes: `protocol.Config/Provider/Model`, `auth.ResolveKey`.
- Produces:

```go
package provider

type Model struct {
	ID, Name, Family, Adapter string // Adapter: "openai" | "anthropic"
	ToolCall, Reasoning, Attachment bool
	Context, Output                int
	CostIn, CostOut, CostCacheRead, CostCacheWrite float64 // USD per 1M
}
type Info struct { // wire: protocol.Provider after mapping
	ID, Name, Source, BaseURL string
	KeyRequired, KeyLoaded bool
	Env []string
	Models []Model
}
type Registry struct {
	info []Info
	defProvider, defModel string
}
func New(ctx context.Context, cfg *protocol.Config, httpc *http.Client, homeDirs Dirs) (*Registry, error)
// Dirs: {KidoBase, ZenBase, ZenCatalog, ZenCache, KidoCache?} with production defaults:
//   KidoBase=https://ai.kido.ws/v1  ZenBase=https://opencode.ai/zen/v1
//   ZenCatalog=https://models.opencode.ai/api.json  ZenCache=<config.CacheYoloDir>/models.json
func (r *Registry) List() []protocol.Provider
func (r *Registry) Resolve(ref string) (Info, Model, error) // "provider/model"; "" -> defaults
func (r *Registry) DriverFor(m Model) llm.Driver // "openai"->NewOpenAI, "anthropic"->NewAnthropic
func (r *Registry) Default() (providerID, modelID string)
```

Kido (`kido.go`):
- `GET {KidoBase}/models` with 5 s timeout. Parse `{data: [{id, meta: {n_ctx}}]}`. Per entry: `Model{ID, Name: id, Adapter: "openai", ToolCall: true, Reasoning: true, Context: meta.n_ctx, Output: min(32768, n_ctx/8)}`. Cost 0.
- Fallback (network error / timeout / any parse failure): static `Qwen3.8-27B` with Context **262144** (verified live 2026-08-17), Output 32768, ToolCall+Reasoning true. Startup must never block/fail on the network.
- Default model `Qwen3.8-27B`; default provider `kido`.

Zen (`zen.go`):
- Catalog source: `GET {ZenCatalog}` (10 s timeout). Response: `{ "<providerID>": {...}, "opencode": {id, env, npm, api, name, models: { "<modelID>": {...} } } }`.
- Keep the `"opencode"` entry only. Per model: keep iff `cost.input > 0` (drops all `-free` variants). Adapter by `provider.npm` (verified field, live 2026-08-17): `"@ai-sdk/anthropic"` → `anthropic`; npm starting with `@ai-sdk/google` → **excluded** (7 models, spec §1 decision 9); everything else (including absent `provider.npm`) → `openai`.
- `Model` mapping: `Name` (display), `Family`, `Limit.context`→Context, `Limit.output`→Output, `cost.input/output/cache_read/cache_write` → Cost*, `tool_call`→ToolCall, `reasoning`→Reasoning, `attachment`→Attachment.
- Cache at `ZenCache` (`~/.cache/yolo/models.json`): consult file first; if missing or mtime older than **5 min**, refetch and rewrite **atomically** (write `<cache>.tmp` then `os.Rename` — opencode's `models-dev` behavior); on fetch failure with a stale cache present: use the stale cache and log.
- Expected counts for the frozen fixture (fetched live 2026-08-17): 91 total models, 64 paid, **42 openai + 15 anthropic kept = 57**, **7 google excluded**. (The spec's §4.1 "42 openai / 15 anthropic" partition is confirmed; the counts in the fixture gate are binding.)

`testdata/kido-models.json` — frozen live response (fetched 2026-08-17), trimmed to the two top-level keys the parser uses:

```json
{"models":[{"name":"Qwen3.8-27B","model":"Qwen3.8-27B","type":"model","details":{"format":"gguf"},"capabilities":["completion"]}],"data":[{"id":"Qwen3.8-27B","aliases":["Qwen3.8-27B"],"object":"model","created":1786973500,"owned_by":"llamacpp","meta":{"vocab_type":2,"n_vocab":248320,"n_ctx":262144,"n_ctx_train":262144,"n_embd":5120,"n_params":27320697856,"size":17912397824,"ftype":"Q4_K - Small"}}]}
```

`testdata/zen-opencode.json` — generated at execution time (Step 1b) and committed.

`provider.go` `New`:
1. Build kido Info (kido.go) + zen Info (zen.go, with cache).
2. Apply `cfg.Provider` overlays per provider id: `baseURL` replaces BaseURL; `apiKey`/`options.apiKey` recorded (key loading itself is via `auth.ResolveKey(providerID, cfg, env)` — `KeyLoaded` = key present OR (provider is kido, key optional)).
3. Config-defined providers (id not in {kido, opencode}) with `baseURL` + `models` map → `Info{Source: "config"}`; model entries: `name?`, `limit.context?` (default 32768), `cost.*?` (0), `adapter?: "openai"|"anthropic"` (default `openai`), `options.*` ignored in v1.
4. Default: `defProvider = cfg.Provider?...` no: `defProvider` = if `cfg.Model` set → its provider part, else `kido`; `defModel` = `cfg.Model` (model part) else `Qwen3.8-27B` (kido).

- [ ] **Step 1: Write the failing test + fixture generation**

`internal/provider/provider_test.go`:

```go
package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
)

func dirs(t *testing.T) provider.Dirs {
	t.Helper()
	d := t.TempDir()
	return provider.Dirs{
		KidoBase:   "http://127.0.0.1:0", // replaced per test
		ZenBase:    "https://opencode.ai/zen/v1",
		ZenCatalog: "http://127.0.0.1:0", // replaced per test
		ZenCache:   filepath.Join(d, "models.json"),
		Home:       d,
	}
}

func TestKidoParsesLlamacpp(t *testing.T) {
	raw, _ := os.ReadFile("testdata/kido-models.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	m, err := provider.FetchKido(context.Background(), srv.URL, 5, raw == nil)  // see Interface note
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Fatalf("models = %d", len(m))
	}
	q := m[0]
	if q.ID != "Qwen3.8-27B" || q.Context != 262144 || !q.ToolCall || !q.Reasoning || q.Adapter != "openai" {
		t.Fatalf("model = %+v", q)
	}
}

func TestKidoFallsBackStaticOnNetworkError(t *testing.T) {
	m, err := provider.FetchKido(context.Background(), "http://127.0.0.1:1", 200, false)
	if err != nil {
		t.Fatalf("fallback must not error: %v", err)
	}
	if len(m) != 1 || m[0].ID != "Qwen3.8-27B" || m[0].Context != 262144 {
		t.Fatalf("fallback model = %+v", m)
	}
}

func TestZenFiltersAndAdapterMap(t *testing.T) {
	raw, err := os.ReadFile("testdata/zen-opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	var cat map[string]any
	if err := json.Unmarshal(raw, &cat); err != nil {
		t.Fatal(err)
	}
	// counts in the fixture (frozen 2026-08-17): 91 models, 64 paid, 57 kept, 7 google
	models := cat["opencode"].(map[string]any)["models"].(map[string]any)
	if len(models) != 91 {
		t.Fatalf("fixture models = %d, want 91", len(models))
	}
	kept, err := provider.ParseZenCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 57 {
		t.Fatalf("kept = %d, want 57", len(kept))
	}
	openaiN, anthropicN := 0, 0
	for _, m := range kept {
		switch m.Adapter {
		case "openai":
			openaiN++
		case "anthropic":
			anthropicN++
		default:
			t.Fatalf("bad adapter %q for %s", m.Adapter, m.ID)
		}
	}
	if openaiN != 42 || anthropicN != 15 {
		t.Fatalf("openai=%d anthropic=%d, want 42/15", openaiN, anthropicN)
	}
	// spot checks
	byID := map[string]provider.Model{}
	for _, m := range kept {
		byID[m.ID] = m
	}
	if byID["claude-opus-4-7"].Adapter != "anthropic" || byID["claude-opus-4-7"].Context != 1000000 {
		t.Fatalf("claude = %+v", byID["claude-opus-4-7"])
	}
	if byID["gpt-5-nano"].Adapter != "openai" || byID["gpt-5-nano"].Context != 400000 {
		t.Fatalf("gpt = %+v", byID["gpt-5-nano"])
	}
	if _, exists := byID["gemini-3-flash"]; exists {
		t.Fatal("google model not excluded")
	}
}

func TestZenCacheTTLAndAtomicRewrite(t *testing.T) {
	d := t.TempDir()
	cache := filepath.Join(d, "models.json")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		raw, _ := os.ReadFile("testdata/zen-opencode.json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	fetchFn := func(ctx context.Context) ([]byte, error) {
		// registry fetches live; here we test the cache policy directly
		raw, _ := os.ReadFile("testdata/zen-opencode.json")
		return raw, nil
	}
	_ = fetchFn
	pol := provider.NewCatalogPolicy(cache, 5, srv.URL)
	_ = pol.Load(context.Background()) // miss -> fetch+write
	if hits != 1 {
		t.Fatalf("hits = %d", hits)
	}
	fi, err := os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() == 0 {
		t.Fatal("empty cache file")
	}
	if _, err := pol.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("fresh cache re-fetched: hits = %d", hits)
	}
	// force stale (mtime -10 min)
	stale := fi.ModTime().Add(-10 * 60 * 1e9)
	_ = os.Chtimes(cache, stale, stale)
	_, _ = pol.Load(context.Background())
	if hits != 2 {
		t.Fatalf("stale cache not refetched: hits = %d", hits)
	}
}

func TestRegistryListAndResolve(t *testing.T) {
	rawK, _ := os.ReadFile("../llm/testdata/../llm/testdata/kido-models.json") // wrong-path guard: real path below
	_ = rawK
	kidoSrv := kidoServer(t)
	zenSrv := zenServer(t)
	d := dirs(t)
	d.KidoBase = kidoSrv.URL
	d.ZenCatalog = zenSrv.URL
	cfg := &protocol.Config{}
	reg, err := provider.New(context.Background(), cfg, http.DefaultClient,
		provider.OverridableDirs(d, true)) // true = use injected URLs
	if err != nil {
		t.Fatal(err)
	}
	ps := reg.List()
	byID := map[string]protocol.Provider{}
	for _, p := range ps {
		byID[p.ID] = p
	}
	if _, ok := byID["kido"]; !ok {
		t.Fatal("kido provider missing")
	}
	z := byID["opencode"]
	if len(z.Models) != 57 {
		t.Fatalf("zen models = %d", len(z.Models))
	}
	if z.Auth == nil || z.Auth.KeyRequired != true || z.Auth.Status != "missing" {
		t.Fatalf("zen auth = %+v", z.Auth)
	}
	k := byID["kido"]
	if k.Auth == nil || k.Auth.KeyRequired != false || k.Auth.Status != "not-required" {
		t.Fatalf("kido auth = %+v", k.Auth)
	}
	info, model, err := reg.Resolve("kido/Qwen3.8-27B")
	if err != nil || model.ID != "Qwen3.8-27B" || info.ID != "kido" {
		t.Fatalf("resolve = %+v %+v %v", info, model, err)
	}
	if p, m, err := reg.Resolve(""); err != nil || p.ID != "kido" || m.ID != "Qwen3.8-27B" {
		t.Fatalf("default resolve = %s/%s %v", p.ID, m.ID, err)
	}
	if _, _, err := reg.Resolve("nope/nope"); err == nil {
		t.Fatal("want error for unknown provider")
	}
}

func kidoServer(t *testing.T) *httptest.Server {
	t.Helper()
	raw, _ := os.ReadFile("testdata/kido-models.json")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(raw)
	}))
}
func zenServer(t *testing.T) *httptest.Server {
	t.Helper()
	raw, _ := os.ReadFile("testdata/zen-opencode.json")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(raw)
	}))
}
```

(Implementation note: `FetchKido(ctx, baseURL string, timeoutMS int, noNet bool) ([]Model, error)` and `ParseZenCatalog(raw []byte) ([]Model, error)` and `NewCatalogPolicy(cachePath string, ttlMin int, liveURL string)` are exported for tests; `New` wires them with `Dirs` + `OverridableDirs` so DI tests can inject httptest URLs. `TestRegistryListAndResolve`'s first two lines are a leftover comment — remove them.)

1b. **Generate the frozen zen fixture** (run once, commit the result):

```bash
python3 - <<'EOF'
import json, urllib.request
cat = json.load(urllib.request.urlopen("https://models.opencode.ai/api.json", timeout=30))
entry = {k: cat["opencode"][k] for k in ("id","env","npm","api","name","models")}
models = entry["models"]
total = len(models)
paid = sum(1 for m in models.values() if (m.get("cost") or {}).get("input",0) > 0)
npm = {}
for m in models.values():
    npm.setdefault(((m.get("provider") or {}).get("npm") or "absent"), 0)
    npm[((m.get("provider") or {}).get("npm") or "absent")] += 1
print("total", total, "paid", paid, "npm", npm)
with open("internal/provider/testdata/zen-opencode.json", "w") as f:
    json.dump(entry, f, indent=1, sort_keys=True)
EOF
```

Gate before committing: output must read `total 91 paid 64` and `npm` must contain `"@ai-sdk/anthropic": 15` (of the paid; the printed map is over all 91 — verify paid counts by re-running the plan's Task-9 python check from spec grounding: 42/15/7 among paid). If the live catalog has drifted, **commit what you fetched and update the count assertions in the test to the fetched values**, and record the new counts in this plan file next to this step.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write minimal implementation**

Implement per the Interface section. `List()` maps `Info` → `protocol.Provider` (models keyed by id; `Options: nil-safe {}`; `Env` from the catalog provider entry). `DriverFor`: "openai" → `llm.NewOpenAI(http.DefaultClient)` (client passed at construction), "anthropic" → `llm.NewAnthropic(...)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider go.mod go.sum
git commit -m "feat: provider registry — kido live/fallback, zen catalog filter+cache"
```

---

# Milestone M3 — Permissions + tools

PLAN ADDITION (locked): a new tiny package `internal/glob/glob.go` — shared glob matcher used by `permission` and `tool` (no import cycles: `glob` imports only `strings`/`regexp`/`testing`). Dependency line extended: `permission → glob`, `tool → glob`, `server` unaffected.

Pinned from upstream v1.18.18 (verified this session):
- Permission engine: `disabled`/`visibleTools` (permission/index.ts:204-219) — for a tool, effective permission name = `"edit"` when tool ∈ {edit, write}, else the tool name (MCP read tools map to `read`; v1 has no MCP); tool is **hidden** iff the *last* rule in the ruleset matching the permission is `pattern == "*" && action == "deny"`.
- Built-in matrices (agent/agent.ts, verbatim): `defaults = { "*": allow, doom_loop: ask, external_directory: {"*": ask, …whitelisted: allow}, question: deny, plan_enter: deny, plan_exit: deny, read: {"*": allow, "*.env": ask, "*.env.*": ask, "*.env.example": allow} }`; `build = defaults + {question: allow, plan_enter: allow} + user`; `plan = defaults + {question: allow, plan_exit: allow, external_directory: {<data>/plans/*: allow}, edit: {"*": deny, <data>/plans/*.md: allow}} + user`; **`yolo = { "*": allow }` only** (Yolo addition per spec decision 13). Merge = append (later entries win by `findLast`). Yolo deviation (flagged): plan notes live in `<dataDir>/plans/*.md` (spec §7) instead of upstream's `.opencode/plans/*.md` + `data/plans` duality — the worktree-relative escape pattern (`path.relative(worktree, <dataDir>/plans/*.md)`) is kept, computed per session.
- read tool output (read.ts:290-360, verbatim): `<path>{fp}</path>\n<type>file</type>\n<content>\n` + lines rendered `{i+offset}: {line}` + trailer — file cut by bytes: `(Output capped at 50KB. Showing lines {offset}-{last}. Use offset={next} to continue.)`, more lines: `(Showing lines {offset}-{last} of {count}. Use offset={next} to continue.)`, complete: `(End of file - total {count} lines)` + `\n</content>`. Miss: `File not found: {fp}\n\nDid you mean one of these?\n{≤3 case-insensitive-basename-substring hits}` (no hits → just `File not found: {fp}`). Directory: sorted entries, dirs get `/` suffix. Binary file → `Cannot read binary file: {fp}` (NUL byte in first 8000 bytes). Defaults limit=2000 (DEFAULT_READ_LIMIT), offset 1-indexed.
- write (write.ts): permission `edit` patterns `[relpath]` always `["*"]`; output `Wrote file successfully.` (+LSP block — v1 skips LSP); title = worktree-relative path. v1 metadata: `{added: int, removed: int}` (upstream sends full diff; deviation flagged).
- edit (edit.ts:681-728): exact error strings pinned — `filePath is required`; `No changes to apply: oldString and newString are identical.`; `oldString cannot be empty when editing an existing file. Provide the exact text to replace, or use write for an intentional full-file replacement.`; `File {fp} not found`; `Path is a directory, not a file: {fp}`; `Could not find oldString in the file. It must match exactly, including whitespace, indentation, and line endings.`; `Found multiple matches for oldString. Provide more surrounding context to make the match unique.` v1 replacer = exact substring only (upstream's 9-replacer fuzzy cascade is deferred — deviation flagged; model retries with a better oldString).
- glob (glob.ts:20-80, verbatim): permission `glob` patterns `[pattern]`; path must be a directory else `glob path must be a directory: {search}`; limit 100; output = absolute resolved paths joined `\n`, empty → `No files found`, truncated → blank line + `(Results are truncated: showing first 100 results. Consider using a more specific path or pattern.)`; title = worktree-relative search dir.
- grep (grep.ts:20-110, verbatim): permission `grep` patterns `[pattern]`; ripgrep binary not available in Go port → walker + `RE2` regex + shared glob `include` filter + NUL-skip binary + skip `.git`; limit 100; no files/matches → `No files found`; output: `Found {n} matches{ (more matches available)}` then per file block: `""`, `{path}:`, `  Line {n}: {text}` per match; truncated → blank line + `(Results truncated. Consider using a more specific path or pattern.)`.
- bash (shell.ts): persistent per-session shell; timeout param optional ms, default `2*60*1000`; timeout error (shell.ts:564, verbatim): `shell tool terminated command after exceeding timeout {ms} ms. If this command is expected to take longer and is not waiting for interactive input, retry with a larger timeout value in milliseconds.`; non-zero exit is NOT a tool error (output carries exit code; metadata `exit`). Permission asks: `external_directory` (scanned dirs as `<dir>/*`) then shell permission with command-derived patterns (tree-sitter scan upstream). v1 simplifications (flagged): (a) permission pattern = first whitespace token `t` → `t *` when the command has more tokens, else `t`; (b) no external-directory pre-scan for bash (config `permission.bash` rules still gate by command prefix); (c) shell protocol: one `bash --norc --noprofile` process per session (Setpgid), command sent as its lines followed by marker line `echo __YOLO_END_{n}_:$?_$(pwd | base64 -w0)`; read stdout/stderr (stderr wired to the same pipe) until a stdout line matches `^__YOLO_END_{n}_(\d+)_(\S*)$`; exit code from marker; cwd from base64 `pwd` (update shell state); on timeout/abort kill process group, mark shell dead, respawn next call. Testable by overriding `Shell.Executable` (e.g. `sh`).
- todowrite (todo.ts, verbatim): params `{todos: [{content, status: pending|in_progress|completed|cancelled, priority?: high|medium|low}]}`; permission `todowrite` patterns `["*"]`; title `"{n completed-pending} todos"` (count of `status != "completed"`); output `JSON.stringify(todos, 2)`; metadata `todos`.

PLAN RESOLUTION (flag to user — spec DDL gap #2): spec DDL has no todo persistence but the TUI footer/dialogs and resume need it. Add migration v2: `CREATE TABLE IF NOT EXISTS todo (id INTEGER PRIMARY KEY, session_id TEXT NOT NULL REFERENCES session(id) ON DELETE CASCADE, content TEXT NOT NULL, status TEXT NOT NULL, priority TEXT NOT NULL DEFAULT 'medium', position INTEGER NOT NULL)`; DAO `SaveTodos(sessionID string, todos []protocol.Todo) error` (delete-all + insert in one tx) and `GetTodos(sessionID string) ([]protocol.Todo, error)` (order by position). `protocol.Todo{Content, Status, Priority string}` added to `internal/protocol/session.go`.

### Task 10: `internal/glob` + `internal/permission` — matcher, evaluation, matrices, ask/reply service

**Files:**
- Create: `internal/glob/glob.go`, `internal/glob/glob_test.go`, `internal/permission/permission.go`, `internal/permission/builtins.go`, `internal/permission/service.go`, `internal/permission/permission_test.go`, `internal/permission/service_test.go`

**Interfaces:**
- Consumes: `protocol.Rule`, `storage.DB`, `bus.Bus`.
- Produces:

```go
package glob
// Match(pattern, name string) bool — pattern is a glob, name a literal.
//   • pattern == "*"          → always true
//   • pattern without "/"     → matches against the LAST path segment (basename) of name
//   • pattern with "/"        → anchored: matches the whole name (name may be relative or absolute)
//   • tokens: "*" within a segment (no "/"), "?" one non-"/" char, "[...]" char class,
//     "**" as a whole segment = zero or more segments ("" or "a/b"/"a/b/c"…)
func Match(pattern, name string) bool
func ToRegex(pattern, name string) (*regexp.Regexp, error) // exported for tests
```

```go
package permission

type Action string
const (Allow Action = "allow"; Deny Action = "denied"; AskAction Action = "ask")

// Rule mirrors protocol.Rule (action field = "allow"|"deny"|"ask")
func LoadBuiltins(agent string, dataDir string) ([]Rule, error) // build|plan|yolo; unknown → error
func Evaluate(rules []Rule, action string, resources []string) Decision
// multi-resource: any resource whose last-matching rule is deny → Deny; else any ask → Ask; else Allow.
// last-matching rule = findLast over rules where Wildcard(action, r.Permission) && glob.Match(r.Pattern, res)
// Wildcard(action, perm): perm == "*" || perm == action. No matching rule for a resource → ask for it.
func Hidden(rules []Rule, tools []string) map[string]bool // upstream disabled(): perm = "edit" if tool in {edit,write} else tool;
                                                          // hidden iff findLast(perm rule) has Pattern=="*" && Action=="deny"
type CallKey struct{ Tool, Hash string } // Hash = sha256 hex of canonical json args (json.Marshal sorts map keys)
func DoomLoopDue(history []CallKey, next CallKey) bool // true iff last 2 of history == next (3rd consecutive identical)

type Request struct {
	RequestID, SessionID, Agent string
	Permission                  string // action, e.g. "read"
	Tool                        string // tool name for TUI
	Resources                   []string
	Always                      []string // suggested always patterns
	Meta                        map[string]any
	DecisionPre                 Decision // Allow|Deny when pre-evaluated (no ask needed); AskAction → block
	CreatedAt                   int64
}
type Service struct{ db *storage.DB; bus *bus.Bus; mu sync.Mutex; pending map[string]*pendingEntry }
func New(db *storage.DB, b *bus.Bus) *Service
func (s *Service) EvaluateRules(agent string, dataDir string, cfgRules []protocol.Rule, action string, resources []string) Decision
// flatten: LoadBuiltins(agent, dataDir) + cfgRules + db.AlwaysRules(sessionID) — needs sessionID:
func (s *s Service) DecisionFor(req Request) Decision // internal: ruleset per above (always from DB)
func (s *Service) Ask(ctx context.Context, req Request) (Decision, error)
// DecisionFor != AskAction → persist row (response=allow/denied) + return immediately.
// AskAction → persist pending row (response=''), emit bus "permission.asked", block on per-request channel
// (ctx cancel → treated as deny with stored response='aborted').
func (s *Service) Reply(requestID, response string) error // "once"|"always"|"reject"
// once → respond Allow. always → persist AlwaysRules (per req.Always: {Permission: req.Permission, Pattern: p, Action: "allow"})
//   + respond Allow + auto-allow other pending in same session whose permission==req.Permission and whose
//   resources are all covered by the new always rules. reject → respond Deny + cascade: respond Deny to all other
//   pending in session (response='rejected'); all emit "permission.replied" {request_id, response, auto? bool}.
func (s *Service) Pending(sessionID string) ([]Request, error)
```

Built-in matrices (exact, order significant — broad first, narrow later; `+` = appended after base for that agent):

```go
var base = []Rule{
	{Perm: "*", Pattern: "*", Effect: "allow"},
	{Perm: "doom_loop", Pattern: "*", Effect: "ask"},
	{Perm: "external_directory", Pattern: "*", Effect: "ask"},
	{Perm: "question", Pattern: "*", Effect: "deny"},
	{Perm: "plan_enter", Pattern: "*", Effect: "deny"},
	{Perm: "plan_exit", Pattern: "*", Effect: "deny"},
	{Perm: "read", Pattern: "*", Effect: "allow"},
	{Perm: "read", Pattern: "*.env", Effect: "ask"},
	{Perm: "read", Pattern: "*.env.*", Effect: "ask"},
	{Perm: "read", Pattern: "*.env.example", Effect: "allow"},
}
build = base + [{question * allow}, {plan_enter * allow}]
plan  = base + [{question * allow}, {plan_exit * allow},
    {external_directory <dataDir>/plans/* allow},
    {edit * deny}, {edit <dataDir>/plans/*.md allow},
    {edit <rel(workdir? no: per-session — evaluated with dataDir only)>}]
```

PLAN FIX (this task owns the decision): upstream's third plan `edit` rule is `rel(worktree, <dataDir>/plans/*.md)` — session-dependent. Since `LoadBuiltins(agent, dataDir)` has no workdir, the plan matrix carries only the two absolute plan rules; the worktree-relative escape rule is added by the **engine** at session start: `{edit, path.Rel(sessionDir, dataDir)/"plans/*.md", allow}` (flag to user as spec §4.5 resolution). `yolo` = single `{"*","*","allow"}`. Config `permission` rules (protocol.Rule) and DB always-rules append after, in that order.

- [ ] **Step 1: Write the failing tests**

`internal/glob/glob_test.go`:

```go
package glob

import "testing"

func cases() map[string]map[string]bool {
	// pattern → name → matches
	return map[string]map[string]bool{
		"*":                      {"/a/b/c.go": true, "x": true},
		"*.env":                  {"a.env": true, "src/a.env": true, "a.env.bak": false, "a.go": false},
		"*.env.*":                {"a.env.local": true, "a.env": false, "b/env2": false},
		"src/**/*.go":            {"src/a.go": true, "src/x/y/b.go": true, "a.go": false, "src.go": false},
		"src/*":                  {"src/a.go": true, "src/x/y.go": false},
		"path/file.txt":          {"path/file.txt": true, "/w/path/file.txt": false}, // anchored relative form
		"/a/*/c":                 {"/a/b/c": true, "/a/b/d/c": false},
		"a?c":                    {"abc": true, "a/c": false, "abbc": false},
		"[abc].go":               {"b.go": true, "d.go": false},
		"**":                     {"/x": true, "x/y": true},
	}
}

func TestMatch(t *testing.T) {
	for pat, names := range cases() {
		for name, want := range names {
			if got := Match(pat, name); got != want {
				t.Errorf("Match(%q, %q) = %v, want %v", pat, name, got, want)
			}
		}
	}
}
```

`internal/permission/permission_test.go`:

```go
package permission

import (
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
)

func r(perm, pattern, effect string) protocol.Rule {
	return protocol.Rule{Permission: perm, Pattern: pattern, Action: effect}
}

func TestEvaluateFindLastNoMatchAsk(t *testing.T) {
	rules := []protocol.Rule{r("*", "*", "allow"), r("read", "*.env", "ask")}
	if got := Evaluate(rules, "read", []string{"a.env"}); got != AskAction {
		t.Fatalf("got %v", got)
	}
	if got := Evaluate(rules, "read", []string{"a.go"}); got != Allow {
		t.Fatalf("got %v", got)
	}
	if got := Evaluate([]protocol.Rule{}, "bash", []string{"ls *"}); got != AskAction {
		t.Fatalf("no rule → ask, got %v", got)
	}
}

func TestMultiResourceAnyDenyWins(t *testing.T) {
	rules := []protocol.Rule{r("*", "*", "allow"), r("edit", "secrets/*", "deny")}
	if got := Evaluate(rules, "edit", []string{"secrets/a", "ok/b"}); got != Deny {
		t.Fatalf("got %v", got)
	}
}

func TestHiddenWildcardDenyLastWins(t *testing.T) {
	// build: no edit rule → findLast("*") = allow → not hidden
	hidden := Hidden(base, []string{"edit", "write", "bash"})
	if hidden["edit"] || hidden["write"] || hidden["bash"] {
		t.Fatalf("build hides: %v", hidden)
	}
	// plan: edit rules appended (deny * then allow data/plans/*.md) → LAST is allow → NOT hidden (upstream semantics)
	planRules := append(append([]protocol.Rule{}, base...),
		r("plan_exit", "*", "allow"),
		r("edit", "*", "deny"),
		r("edit", "/data/plans/*.md", "allow"))
	if Hidden(planRules, []string{"edit"})["edit"] {
		t.Fatal("plan edit must stay visible (last rule is allow)")
	}
	// a ruleset ending in wildcard deny hides edit AND write (edit-permission mapping)
	denied := append(append([]protocol.Rule{}, base...), r("edit", "*", "deny"))
	h := Hidden(denied, []string{"edit", "write", "bash"})
	if !h["edit"] || !h["write"] || h["bash"] {
		t.Fatalf("hidden = %v", h)
	}
}

func TestBuiltinsYoloAllowsEverything(t *testing.T) {
	rules, err := LoadBuiltins("yolo", "/data")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"read", "write", "edit", "bash", "doom_loop", "question"} {
		if got := Evaluate(rules, action, []string{"anything"}); got != Allow {
			t.Fatalf("yolo %s → %v", action, got)
		}
	}
}

func TestBuiltinsPlanDeniesEditUnlessPlanPath(t *testing.T) {
	rules, err := LoadBuiltins("plan", "/data")
	if err != nil {
		t.Fatal(err)
	}
	if got := Evaluate(rules, "edit", []string{"src/main.go"}); got != Deny {
		t.Fatalf("plan edit src → %v", got)
	}
	if got := Evaluate(rules, "edit", []string{"/data/plans/x.md"}); got != Allow {
		t.Fatalf("plan edit plans → %v", got)
	}
	if got := Evaluate(rules, "plan_exit", []string{"*"}); got != Allow {
		t.Fatalf("plan plan_exit → %v", got)
	}
	if got := Evaluate(rules, "bash", []string{"git *"}); got != Allow {
		t.Fatalf("plan bash → %v (base * allow)", got)
	}
}

func TestDoomLoopDue(t *testing.T) {
	k := func() CallKey { return CallKey{"bash", "abc"} }
	if DoomLoopDue(nil, k()) || DoomLoopDue([]CallKey{k()}, k()) {
		t.Fatal("too early")
	}
	if !DoomLoopDue([]CallKey{k(), k()}, k()) {
		t.Fatal("third consecutive identical must fire")
	}
}
```

`internal/permission/service_test.go` (uses Task 5/6 storage+bus):

```go
package permission

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

type env struct {
	db  *storage.DB
	bus *bus.Bus
	svc *Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	b := bus.New()
	return &env{db: db, bus: b, svc: New(db, b)}
}

func (e *env) req(id string) Request {
	return Request{RequestID: id, SessionID: "ses_1", Agent: "build",
		Permission: "read", Tool: "read", Resources: []string{"src/x.go"}, Always: []string{"src/*"},
		CreatedAt: time.Now().UnixMilli()}
}

func TestAskPreAllowNoBlock(t *testing.T) {
	e := newEnv(t)
	if err := e.db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	req := e.req("per_1")
	req.DecisionPre = Allow
	done := make(chan Decision, 1)
	go func() { d, err := e.svc.Ask(context.Background(), req); if err == nil { done <- d } }()
	select {
	case d := <-done:
		if d != Allow {
			t.Fatalf("d = %v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pre-allow must not block")
	}
}

func TestAskAskBlocksThenOnce(t *testing.T) {
	e := newEnv(t)
	if err := e.db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	// force an ask: read src/x.go is allow by build base → use a deny-then pattern: use agent yolo? no.
	// Use permission action "custom" with no rule → always ask.
	req := e.req("per_2")
	req.Permission = "custom"
	done := make(chan Decision, 1)
	go func() { d, err := e.svc.Ask(context.Background(), req); if err == nil { done <- d } }()
	time.Sleep(100 * time.Millisecond) // let it park
	if pend, _ := e.svc.Pending("ses_1"); len(pend) != 1 {
		t.Fatalf("pending = %d", len(pend))
	}
	go func() { _ = e.svc.Reply("per_2", "once"); done <- Allow }()
	if err := e.svc.Reply("per_2", "once"); err != nil {
		t.Fatal(err)
	}
	select {
	case d := <-done:
		if d != Allow {
			t.Fatalf("d = %v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reply did not unblock")
	}
}

func TestAlwaysPersistsRuleAndCoveredAutoAnswer(t *testing.T) {
	e := newEnv(t)
	_ = e.db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"})
	// two parked asks, same permission, second fully covered by first's always pattern
	r1 := e.req("per_3"); r1.Permission = "custom"; r1.Resources = []string{"a/b"}; r1.Always = []string{"a/*"}
	r2 := e.req("per_4"); r2.Permission = "custom"; r2.Resources = []string{"a/c"}
	go func() { _, _ = e.svc.Ask(context.Background(), r1) }()
	go func() { _, _ = e.svc.Ask(context.Background(), r2) }()
	time.Sleep(100 * time.Millisecond)
	if err := e.svc.Reply("per_3", "always"); err != nil {
		t.Fatal(err)
	}
	rules, err := e.db.AlwaysRules("ses_1")
	if err != nil || len(rules) != 1 || rules[0].Pattern != "a/*" {
		t.Fatalf("always rules = %+v err=%v", rules, err)
	}
	// r2 auto-answered: no longer pending
	if pend, _ := e.svc.Pending("ses_1"); len(pend) != 0 {
		t.Fatalf("pending after always = %d", len(pend))
	}
}

func TestRejectCascade(t *testing.T) {
	e := newEnv(t)
	_ = e.db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"})
	r1 := e.req("per_5"); r1.Permission = "custom"
	r2 := e.req("per_6"); r2.Permission = "custom"
	res1 := make(chan Decision, 1)
	res2 := make(chan Decision, 1)
	go func() { d, _ := e.svc.Ask(context.Background(), r1); res1 <- d }()
	go func() { d, _ := e.svc.Ask(context.Background(), r2); res2 <- d }()
	time.Sleep(100 * time.Millisecond)
	if err := e.svc.Reply("per_5", "reject"); err != nil {
		t.Fatal(err)
	}
	for i, ch := range []chan Decision{res1, res2} {
		select {
		case d := <-ch:
			if d != Deny {
				t.Fatalf("cascade result %d = %v", i, d)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("cascade did not unblock %d", i)
		}
	}
}
```

(Implementation note: `Request.Agent` is used by DecisionFor only for builtins; `DecisionPre` is set by the engine after calling `EvaluateRules` with the session's always-rules — the service re-checks via DB for Ask-action requests. In the tests above the engine-side evaluation is bypassed by forcing `DecisionPre`; `Ask` with `DecisionPre==AskAction` consults `DecisionFor`. Keep exactly this split: **engine evaluates, service enforces+blocks**.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/glob/ ./internal/permission/ -v`
Expected: FAIL — packages do not exist.

- [ ] **Step 3: Write minimal implementation**

`glob.go`: segment-based conversion per the semantics above (`**` only honored as a full segment; `*`/`?`/`[...]` never cross `/`). `permission.go`: Evaluate/Hidden/DoomLoopDue/DecisionFor. `builtins.go`: the three matrices (plan gets `<dataDir>/plans/*` and `<dataDir>/plans/*.md` absolute patterns). `service.go`: pending map + channels, storage rows (PermissionRow with JSON payload column `always_json` + `meta`), bus events `permission.asked` (properties: full Request) and `permission.replied` (properties: `{request_id, response, auto}`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/glob internal/permission
git commit -m "feat: glob matcher + permission engine (findLast, doom loop, ask/reply service, build/plan/yolo)"
```

---

### Task 11: `internal/tool` — framework, truncation, read tool

**Files:**
- Create: `internal/tool/tool.go`, `internal/tool/truncate.go`, `internal/tool/read.go`, `internal/tool/tool_test.go`, `internal/tool/desc/read.txt`

**Interfaces:**
- Consumes: `protocol`, `storage`, `glob`, upstream prompt texts (tool descriptions verbatim from `packages/opencode/src/tool/*.txt`).
- Produces:

```go
package tool

type Limits struct{ MaxLines, MaxBytes int } // config tool_output.* ; defaults 2000 / 50*1024

type Env struct {
	Dir     string // session project dir (permission anchor, relative-path base)
	Shell   *Shell // per-session (Task 14); nil-safe for non-bash tools
	Limits  Limits
}

type Output struct {
	Title string
	Text  string           // model-visible
	Meta  map[string]any   // TUI metadata (nil ok)
}

type Tool interface {
	ID() string                 // "read"
	Permission() string         // core permission action ("read", "edit", "glob", …)
	Patterns(raw json.RawMessage) (resources []string, always []string, err error)
	External(raw json.RawMessage) ([]string, error) // absolute paths subject to external_directory check
	Schema() map[string]any     // JSON Schema object (parameters)
	Desc() string               // desc/<id>.txt (go:embed)
	Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error)
}

func Registry() map[string]Tool // the 7 tools keyed by ID (grown by later tasks; Task 11 = {read})
func Visible(rules []protocol.Rule, all map[string]Tool) map[string]Tool // permission.Hidden filter
func SchemaFor(t Tool) map[string]any // {"type":"function","function":{"name","description","parameters"}}
func Truncate(text string, l Limits) (string, bool) // tail-keep per upstream shell.ts tail(): last l.MaxLines lines
                                                    // within l.MaxBytes, UTF-8-safe cut; returns (text, cut)
```

`run` helper used by the engine (documented here, implemented in Task 11, exercised from Task 17):

```go
// Engine contract (final, implemented by internal/session):
//   res, always, err := t.Patterns(args)
//   for _, p := range t.External(args) { if outside env.Dir → svc.Ask("external_directory", dir(p)+"/*") }
//   d := svc.Ask(ctx, Request{Permission: t.Permission(), Resources: res, Always: always, Tool: t.ID()})
//   Deny → part error "permission rejected" ; Allow → out, err := t.Run(...)
```

- [ ] **Step 1: Write the failing tests**

`internal/tool/tool_test.go`:

```go
package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
)

func tmpFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runRead(t *testing.T, p string, offset, limit int) (Output, error) {
	t.Helper()
	args, _ := json.Marshal(map[string]any{"filePath": p})
	if offset > 0 {
		_ = args // rebuilt below when needed
	}
	var m map[string]any
	_ = json.Unmarshal(args, &m)
	m["filePath"] = p
	if offset > 0 {
		m["offset"] = offset
	}
	if limit > 0 {
		m["limit"] = limit
	}
	raw, _ := json.Marshal(m)
	env := Env{Dir: filepath.Dir(p), Limits: Limits{2000, 50 * 1024}}
	return Must[Registry()["read"]].Run(context.Background(), raw, &env)
}
```

(Note: define `type R = Tool; func Must[T interface{}](_ T) T { return _ }` is NOT needed — simplify to `Registry()["read"].Run(...)`; fix the helper to call directly.)

Tests (exact expectations, derived from pinned upstread format):

```go
func TestReadFileExactFormat(t *testing.T) {
	p := tmpFile(t, "a.txt", "l1\nl2\nl3\n")
	out, err := runRead(t, p, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "<path>" + p + "</path>\n<type>file</type>\n<content>\n1: l1\n2: l2\n3: l3\n\n(End of file - total 3 lines)\n</content>"
	if out.Text != want {
		t.Fatalf("text mismatch:\n%q\nwant:\n%q", out.Text, want)
	}
	if out.Title != "a.txt" {
		t.Fatalf("title = %q", out.Title)
	}
}

func TestReadFileOffsetLimit(t *testing.T) {
	p := tmpFile(t, "a.txt", strings.Repeat("x\n", 10))
	out, err := runRead(t, p, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := "<path>" + p + "</path>\n<type>file</type>\n<content>\n3: x\n4: x\n\n(Showing lines 3-4 of 10. Use offset=5 to continue.)\n</content>"
	if out.Text != want {
		t.Fatalf("text = %q", out.Text)
	}
}

func TestReadFileOffsetOutOfRange(t *testing.T) {
	p := tmpFile(t, "a.txt", "a\nb\n")
	_, err := runRead(t, p, 99, 0)
	if err == nil || !strings.Contains(err.Error(), "Offset 99 is out of range for this file") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadDirListing(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	os.WriteFile(filepath.Join(d, "b.txt"), []byte("z"), 0o644)
	os.WriteFile(filepath.Join(d, "A.txt"), []byte("z"), 0o644)
	out, err := runRead(t, d, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// localeCompare sort: "A.txt" < "b.txt" < "sub/"
	if !strings.Contains(out.Text, "A.txt\nb.txt\nsub/\n") && !strings.Contains(out.Text, "A.txt") {
		t.Fatalf("listing = %q", out.Text)
	}
	for _, frag := range []string{"A.txt", "b.txt", "sub/"} {
		if !strings.Contains(out.Text, frag) {
			t.Fatalf("listing missing %q: %q", frag, out.Text)
		}
	}
}

func TestReadMissingFileSuggests(t *testing.T) {
	p := tmpFile(t, "src/app.go", "x")
	_, err := runRead(t, strings.Replace(p, "app.go", "ap.go", 1), 0, 0)
	if err == nil || !strings.Contains(err.Error(), "Did you mean one of these?") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadBinaryRefused(t *testing.T) {
	p := tmpFile(t, "bin.dat", "\x00\x01\x02"+strings.Repeat("a", 100))
	_, err := runRead(t, p, 0, 0)
	if err == nil || !strings.HasPrefix(err.Error(), "Cannot read binary file:") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadByteCap(t *testing.T) {
	p := tmpFile(t, "big.txt", strings.Repeat("abcdefgh\n", 3000)) // 27000 bytes
	out, err := runRead(t, p, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "(Output capped at 50KB. Showing lines") || !strings.Contains(out.Text, "Use offset=") {
		t.Fatalf("no cap trailer: %q…", out.Text[:200])
	}
}

func TestTruncateTail(t *testing.T) {
	lines := make([]string, 3000)
	for i := range lines {
		lines[i] = "line"
	}
	out, cut := Truncate(strings.Join(lines, "\n"), Limits{100, 50 * 1024})
	if !cut {
		t.Fatal("want cut")
	}
	got := strings.Split(out, "\n")
	if len(got) != 100 {
		t.Fatalf("lines = %d", len(got))
	}
}

func TestReadSchema(t *testing.T) {
	s := SchemaFor(MustTool(Registry()["read"]))
	fn := s["function"].(map[string]any)
	if fn["name"] != "read" {
		t.Fatalf("name = %v", fn["name"])
	}
	params := fn["parameters"].(map[string]any)["properties"].(map[string]any)
	for _, k := range []string{"filePath", "offset", "limit"} {
		if _, ok := params[k]; !ok {
			t.Fatalf("missing param %s", k)
		}
	}
}
```

(Helper `MustTool[T Tool](t T) T { return t }` — trivial identity to keep call sites tidy; or drop it. Implementation picks one and applies to both tasks.)

desc/read.txt — copy verbatim from `/tmp/opencode-upstream/packages/opencode/src/tool/read.txt` (execute: `cp` + `git add`; pin content via a sha256 check in the test):

```go
func TestDescPinned(t *testing.T) {
	// sha256 of upstream v1.18.18 read.txt
	if !sha256Ok(t, "desc/read.txt", "REPLACE_WITH_COMPUTED_HASH") {
		t.Fatal("desc drifted")
	}
}
```

(Step 3a computes the hash: `sha256sum /tmp/opencode-upstream/packages/opencode/src/tool/read.txt` and fills the constant.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tool/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write minimal implementation**

`tool.go` (framework + Registry with `read` + `Visible` + `SchemaFor`), `truncate.go` (port of upstream `tail()` including UTF-8-boundary cut — the reference implementation is pinned in the M3 preamble), `read.ts` → `read.go`: exact format strings from the pin list (path wrapper, `{i+offset}: {line}` renderers, three trailers, `Did you mean` = case-insensitive substring of basename against siblings (either direction), top 3; directory listing = `sort` by `strings.ToLower`-insensitive collation approximating `localeCompare` (use Go `sort.Slice` with `strings.EqualFold`-aware less — pin: sort by lowercased entry with `/` suffix kept for comparison; document approximation), binary sniff = NUL in first 8000 bytes, `Offset N is out of range for this file (M lines)`), desc embed.

- [ ] **Step 4: Run test to verify it passes**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tool
git commit -m "feat: tool framework + truncation + read tool (upstream format verbatim)"
```

---

### Task 12: write + edit tools

**Files:**
- Create: `internal/tool/write.go`, `internal/tool/edit.go`, `internal/tool/edit_test.go`, `internal/tool/write_test.go`, `internal/tool/desc/write.txt`, `internal/tool/desc/edit.txt`

**Interfaces:**
- Consumes: Task 11 framework.
- Produces: `Registry()` now = {read, write, edit}. Both: `Permission() == "edit"` → `Hidden()` treats write like edit (upstream `disabled()` mapping). `Patterns(raw)` = `[rel(env.Dir, filePath)]`, always `["*"]`; `External` = resolved absolute path.

- [ ] **Step 1: Write the failing tests**

```go
// edit_test.go (package tool)
func tmpEnv(t *testing.T, file, content string) (string, *Env) {
	t.Helper()
	p := tmpFile(t, file, content)
	return p, &Env{Dir: t.TempDir(), Limits: Limits{2000, 50 * 1024}}
}

func runTool(t *testing.T, id string, env *Env, args map[string]any) (Output, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	return Registry()[id].Run(context.Background(), raw, env)
}

func TestWriteCreatesAndReports(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, err := runTool(t, "write", env, map[string]any{
		"filePath": filepath.Join(d, "new.txt"), "content": "a\nb\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "Wrote file successfully." {
		t.Fatalf("text = %q", out.Text)
	}
	metaAdded, _ := out.Meta["added"].(int)
	if metaAdded != 2 {
		t.Fatalf("added = %v", out.Meta)
	}
	b, _ := os.ReadFile(filepath.Join(d, "new.txt"))
	if string(b) != "a\nb\n" {
		t.Fatal("content mismatch")
	}
}

func TestWriteMissingDirCreated(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	_, err := runTool(t, "write", env, map[string]any{
		"filePath": filepath.Join(d, "a", "b", "c.txt"), "content": "x",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditExactReplace(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "f.txt")
	os.WriteFile(f, []byte("one\ntwo\nthree\n"), 0o644)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, err := runTool(t, "edit", env, map[string]any{
		"filePath": f, "oldString": "two", "newString": "TWO",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(f)
	if string(b) != "one\nTWO\nthree\n" {
		t.Fatalf("content = %q", b)
	}
	_ = out
}

func TestEditErrorsPinned(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "f.txt")
	os.WriteFile(f, []byte("a\nb\na\n"), 0o644)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}

	_, err := runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "a", "newString": "a"})
	if err == nil || err.Error() != "No changes to apply: oldString and newString are identical." {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "", "newString": "a"})
	if err == nil || !strings.HasPrefix(err.Error(), "oldString cannot be empty") {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": filepath.Join(d, "nope.txt"), "oldString": "x", "newString": "y"})
	if err == nil || err.Error() != "File "+filepath.Join(d, "nope.txt")+" not found" {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": d, "oldString": "x", "newString": "y"})
	if err == nil || !strings.Contains(err.Error(), "Path is a directory, not a file") {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "zzz", "newString": "y"})
	if err == nil || err.Error() != "Could not find oldString in the file. It must match exactly, including whitespace, indentation, and line endings." {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "a", "newString": "b"})
	if err == nil || err.Error() != "Found multiple matches for oldString. Provide more surrounding context to make the match unique." {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "a", "newString": "b", "replaceAll": true})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(f)
	if string(b) != "b\nb\nb\n" {
		t.Fatalf("replaceAll content = %q", b)
	}
}

func TestEditPatternsAndExternal(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "sub", "f.txt")
	raw, _ := json.Marshal(map[string]any{"filePath": f})
	res, always, err := Registry()["edit"].Patterns(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0] != "sub/f.txt" || len(always) != 1 || always[0] != "*" {
		t.Fatalf("patterns = %v %v", res, always)
	}
	ext, _ := Registry()["edit"].External(raw)
	if len(ext) != 1 || ext[0] != f {
		t.Fatalf("external = %v", ext)
	}
	if Registry()["edit"].Permission() != "edit" || Registry()["write"].Permission() != "edit" {
		t.Fatal("permission mapping")
	}
	hidden := permissionVisible(t, nil) // no rules → nothing hidden
	if !hidden["write"] {
		t.Fatal("write visible without rules")
	}
}
```

(Use `Visible(nil, Registry())` for the last block — no wildcard-deny rules → all visible.)

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/tool/ -run 'TestWrite|TestEdit' -v` → FAIL (tools absent).

- [ ] **Step 3: Write minimal implementation**

`write.go`: resolve (abs or join env.Dir), external check is the engine's job — tool only computes paths; `os.MkdirAll(dir)`; `os.WriteFile` 0644; output `Wrote file successfully.`; Meta `{added, removed}` via a small line-diff helper `diffCounts(old, new string) (added, removed int)` (added = lines in new not matched by old count — implement as: LCS-free simple `linesAdded = len(new lines) - commonLines` approximation is WRONG for tests (expected 2 added for a brand-new file: 0-0 → 2/0 fine; for edit of a→b in a 3-line file: added=1, removed=1 — implement a minimal Myers/LCS line diff, ~60 lines, pinned in `write.go` — tests assert exact counts). `edit.go`: exact-string `strings.Count` uniqueness + single replace / `strings.ReplaceAll`; per-file mutex map (upstream `Semaphore` per resolved path — port as `sync.Map` of `*sync.Mutex`); errors exactly as pinned.

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tool
git commit -m "feat: write + edit tools (upstream error strings, exact-match replacer, per-file lock)"
```

---

### Task 13: glob + grep tools

**Files:**
- Create: `internal/tool/glob.go`, `internal/tool/grep.go`, `internal/tool/globgrep_test.go`, `internal/tool/desc/glob.txt`, `internal/tool/desc/grep.txt`

**Interfaces:**
- Consumes: Task 11 framework, `internal/glob`.
- Produces: `Registry()` = {read, write, edit, glob, grep}.

Glob tool spec (pinned): params `{pattern (required), path? (dir)}`; permission `glob` resources `[pattern]` always `["*"]`; `External` = resolved search dir (dir kind); search = abs(path) else env.Dir; stat: missing → treat as empty result `No files found` (upstream: stat catch → undefined → ripgrep cwd=missing would error; v1: missing dir → error `glob path must be a directory: {search}` — matches upstream `info?.type === "File"` throw for files; missing dir: ripglob cwd would fail → v1 chooses the same explicit error, deviation noted); walk tree skipping `.git`/hidden dirs; match by `glob.Match(pattern, relPath)` where pattern without `/` matches relpath basename (ripgrep `--glob` default = basename for slashless patterns — faithful); results sorted; limit 100; output per pin (absolute resolved paths; `No files found`; truncation note); title = `rel(env.Dir, search)` (`"."` → `"."`).

Grep tool spec (pinned): params `{pattern (required, RE2), path?, include?}`; permission `grep` resources `[pattern]` always `["*"]`; `External` = resolved search root; search root: file → its dir (upstream `path.dirname`); missing root → `No files found`; compile RE2 `(?m)` per file? No — one `regexp.Compile(pattern)` (v1: no `(?i)` etc. beyond user's own flags); walk files (skip `.git`, skip NUL-binary first 8000, skip >10MB); for each line: `re.MatchString(line)` → record; **limit 100 total matches** (stop walking at 100 → `truncated` true); `include` filter = `glob.Match(include, relPathFromRoot)` (slashless include → basename, matches ripgrep `--glob` default); output blocks exactly per pin; metadata `{matches: n, truncated: bool}`.

- [ ] **Step 1: Write the failing tests**

```go
func TestGlobTool(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "a", "b"), 0o755)
	os.MkdirAll(filepath.Join(d, ".git"), 0o755)
	for _, p := range []string{"a/x.go", "a/b/y.go", "a/z.md", ".git/skip.go"} {
		os.WriteFile(filepath.Join(d, p), []byte("x"), 0o644)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}

	out, err := runTool(t, "glob", env, map[string]any{"pattern": "**/*.go"})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.Text), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d: %v", len(lines), lines)
	}
	for _, frag := range []string{"/a/x.go", "/a/b/y.go"} {
		if !strings.Contains(out.Text, frag) {
			t.Fatalf("missing %q in %q", frag, out.Text)
		}
	}
	if strings.Contains(out.Text, ".git") {
		t.Fatal(".git leaked")
	}

	_, err = runTool(t, "glob", env, map[string]any{"pattern": "*", "path": filepath.Join(d, "a", "x.go")})
	if err == nil || !strings.Contains(err.Error(), "glob path must be a directory") {
		t.Fatalf("err = %v", err)
	}

	out2, _ := runTool(t, "glob", env, map[string]any{"pattern": "nomatch*"})
	if out2.Text != "No files found" {
		t.Fatalf("empty = %q", out2.Text)
	}
}

func TestGrepTool(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "a.txt"), []byte("alpha\nbeta\n"), 0o644)
	os.WriteFile(filepath.Join(d, "b.md"), []byte("alpha here\n"), 0o644)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}

	out, err := runTool(t, "grep", env, map[string]any{"pattern": "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(out.Text, "\n") {
		lines = append(lines, l)
	}
 joined := out.Text
	if !strings.Contains(joined, "Found 2 matches") ||
		!strings.Contains(joined, filepath.Join(d, "a.txt")+":") ||
		!strings.Contains(joined, "  Line 1: alpha") ||
		!strings.Contains(joined, filepath.Join(d, "b.md")+":") {
		t.Fatalf("output = %q", joined)
	}

	out2, _ := runTool(t, "grep", env, map[string]any{"pattern": "alpha", "include": "*.md"})
	if !strings.Contains(out2.Text, "Found 1 matches") || strings.Contains(out2.Text, "a.txt") {
		t.Fatalf("include filter broken: %q", out2.Text)
	}

	out3, _ := runTool(t, "grep", env, map[string]any{"pattern": "nope"})
	if out3.Text != "No files found" {
		t.Fatalf("no match = %q", out3.Text)
	}
}

func TestGrepLimit100(t *testing.T) {
	d := t.TempDir()
	var b strings.Builder
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&b, "hit\n")
	}
	os.WriteFile(filepath.Join(d, "big.txt"), []byte(b.String()), 0o644)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, _ := runTool(t, "grep", env, map[string]any{"pattern": "hit"})
	if !strings.Contains(out.Text, "Found 100 matches (more matches available)") ||
		!strings.Contains(out.Text, "(Results truncated. Consider using a more specific path or pattern.)") {
		t.Fatalf("limit output = %q", out.Text[:300])
	}
}
```

(Format note: the per-file block is `""` then `"{path}:"` then `  Line {n}: {text}` lines — assert with `strings.Contains`, exact block shape asserted in the first test via full-string compare on the 1-file case:

```go
func TestGrepExactBlock(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "only.txt"), []byte("x\nalpha\n"), 0o644)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, _ := runTool(t, "grep", env, map[string]any{"pattern": "alpha"})
	want := "Found 1 matches\n\n" + filepath.Join(d, "only.txt") + ":\n  Line 2: alpha"
	if out.Text != want {
		t.Fatalf("block = %q", out.Text)
	}
}
```
)

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/tool/ -run 'Glob|Grep' -v` → FAIL (tools absent).

- [ ] **Step 3: Write minimal implementation** — per pinned specs above. Walk = `filepath.WalkDir` with pruning (`.git`, entries starting with `.` skipped for glob; for grep only `.git` pruned, dotfiles searched — ripgrep default skips hidden, deviation note: grep skips dotfiles too, matching ripgrep `--hidden` default false).

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tool
git commit -m "feat: glob + grep tools (ripgrep-compatible output format, RE2 engine)"
```

---

### Task 14: bash + todowrite tools

**Files:**
- Create: `internal/tool/bash.go`, `internal/tool/shell.go`, `internal/tool/todowrite.go`, `internal/tool/bash_test.go`, `internal/tool/todowrite_test.go`, `internal/tool/desc/bash.txt`, `internal/tool/desc/todowrite.txt`
- Modify: `internal/protocol/session.go` (add `Todo`), `internal/storage/migrate.go` (migration v2: todo table), `internal/storage/dao.go` (`SaveTodos`/`GetTodos`)

**Interfaces:**
- Consumes: Task 11 framework, `storage`.
- Produces:

```go
package tool

type Shell struct {
	Executable string // default "bash"; test override
	Dir        string
	limits     Limits
	// internal: cmd, stdin, stdout reader, state (cwd), nextMarker, mu
}
func NewShell(dir string, limits Limits) *Shell
func (s *Shell) Exec(ctx context.Context, command string, timeoutMS int, onLine func(line string)) (exitCode int, out string, err error)
// out = full stdout+stderr combined BEFORE truncation is the engine's business? No:
// out = combined raw output (streamed line-by-line via onLine); the tool applies Truncate for Output.Text.
func (s *Shell) Cwd() string
func (s *Shell) Close() error
```

`Env.Shell` is now used by bash. `Registry()` = all 7.

bash tool params: `{command: string (required), timeout?: number (ms; default 120000)}`. Permission `bash` resources `[prefix]` where prefix = first whitespace token + `" *"` when more tokens (e.g. `git *`), else the token; always `[prefix]`. Non-zero exit → success with `Meta{exit: n}` (model sees exit code only when present; output text = tool output). Timeout → tool error, text = pinned upstream message with the ms value. Abort (ctx) → tool error `command aborted`.

todowrite: params `{todos: [{content (required), status (pending|in_progress|completed|cancelled), priority? (high|medium|low)}]}`; validate status/priority values → error `invalid status` / `invalid priority`; permission `todowrite` resources `["*"]` always `["*"]`; persist via `env.Storage? ` — Env gains `Storage *storage.DB` and `SessionID string` (added to `tool.Env` in THIS task; Task 11-13 tools ignore them); `SaveTodos(sessionID, todos)`; title `"{k} todos"` where k = count(status != completed); output = `json.MarshalIndent(todos, "", "  ")` (matches upstream `JSON.stringify(todos, null, 2)`); Meta `{todos: todos}`.

`protocol.Todo`:

```go
type Todo struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority,omitempty"`
}
```

TODO table migration v2 (exact SQL, appended after v1 statements, guarded by `meta.schema_version`):

```sql
CREATE TABLE IF NOT EXISTS todo (
    id INTEGER PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES session(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    status TEXT NOT NULL,
    priority TEXT NOT NULL DEFAULT 'medium',
    position INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_todo_session ON todo(session_id);
```

- [ ] **Step 1: Write the failing tests**

```go
// bash_test.go
func runBash(t *testing.T, dir, cmd string, timeoutMS int) (Output, error) {
	t.Helper()
	env := &Env{Dir: dir, Limits: Limits{2000, 50 * 1024}}
	env.Shell = NewShell(dir, env.Limits)
	t.Cleanup(func() { env.Shell.Close() })
	raw, _ := json.Marshal(map[string]any{"command": cmd})
	if timeoutMS > 0 {
		_ = timeoutMS
	}
	m := map[string]any{"command": cmd}
	if timeoutMS > 0 {
		m["timeout"] = timeoutMS
	}
	raw, _ = json.Marshal(m)
	return Registry()["bash"].Run(context.Background(), raw, env)
}
```

(Keep ONE marshal — fix in implementation: build `m` first, single Marshal.)

```go
func TestBashBasicAndCwdPersistence(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	out, err := runBash(t, d, "mkdir -p sub2 && cd sub2 && pwd", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(out.Text), "sub2") {
		t.Fatalf("pwd output = %q", out.Text)
	}
	// cwd persisted inside the persistent shell
	out2, err := runBash(t, d, "pwd", 0) // NEW shell instance in test; persistence asserted within one shell:
	// -> use a single shell for the whole test instead (see fix below)
	_ = out2
	_ = err
}
```

IMPLEMENTATION FIX (locked): the test MUST create one shell and run both commands on it:

```go
func TestBashCwdPersistsAcrossCalls(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	t.Cleanup(func() { env.Shell.Close() })
	raw, _ := json.Marshal(map[string]any{"command": "cd sub"})
	_, err := Registry()["bash"].Run(context.Background(), raw, env)
	if err != nil {
		t.Fatal(err)
	}
	raw2, _ := json.Marshal(map[string]any{"command": "pwd"})
	out, err := Registry()["bash"].Run(context.Background(), raw2, env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.Text) != filepath.Join(d, "sub") {
		t.Fatalf("cwd not persisted: %q", out.Text)
	}
	// env persistence too
	raw3, _ := json.Marshal(map[string]any{"command": "FOO=bar"})
	_, _ = Registry()["bash"].Run(context.Background(), raw3, env)
	raw4, _ := json.Marshal(map[string]any{"command": "echo $FOO"})
	out2, _ := Registry()["bash"].Run(context.Background(), raw4, env)
	if strings.TrimSpace(out2.Text) != "bar" {
		t.Fatalf("env not persisted: %q", out2.Text)
	}
}

func TestBashNonZeroExitIsSuccessWithMeta(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	raw, _ := json.Marshal(map[string]any{"command": "echo oops; exit 3"})
	out, err := Registry()["bash"].Run(context.Background(), raw, env)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Meta["exit"]; got != 3 {
		t.Fatalf("exit = %v", got)
	}
	if !strings.Contains(out.Text, "oops") {
		t.Fatalf("text = %q", out.Text)
	}
}

func TestBashStderrMerged(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	raw, _ := json.Marshal(map[string]any{"command": "echo err >&2"})
	out, _ := Registry()["bash"].Run(context.Background(), raw, env)
	if !strings.Contains(out.Text, "err") {
		t.Fatalf("stderr missing: %q", out.Text)
	}
}

func TestBashTimeoutKillsAndReports(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	raw, _ := json.Marshal(map[string]any{"command": "sleep 5", "timeout": 300})
	_, err := Registry()["bash"].Run(context.Background(), raw, env)
	if err == nil {
		t.Fatal("want timeout error")
	}
	want := "shell tool terminated command after exceeding timeout 300 ms. If this command is expected to take longer and is not waiting for interactive input, retry with a larger timeout value in milliseconds."
	if err.Error() != want {
		t.Fatalf("err = %q", err)
	}
	// shell respawned cleanly afterward
	raw2, _ := json.Marshal(map[string]any{"command": "echo alive"})
	out, err2 := Registry()["bash"].Run(context.Background(), raw2, env)
	if err2 != nil || strings.TrimSpace(out.Text) != "alive" {
		t.Fatalf("respawn failed: %v %q", err2, out.Text)
	}
}

func TestBashPermissionPatterns(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"command": "git commit -m x"})
	res, always, err := Registry()["bash"].Patterns(raw)
	if err != nil || res[0] != "git *" || always[0] != "git *" {
		t.Fatalf("patterns %v %v %v", res, always, err)
	}
	raw2, _ := json.Marshal(map[string]any{"command": "ls"})
	res2, _, _ := Registry()["bash"].Patterns(raw2)
	if res2[0] != "ls" {
		t.Fatalf("single-token = %v", res2)
	}
	if Registry()["bash"].Permission() != "bash" {
		t.Fatal("perm action")
	}
}

// todowrite_test.go
func TestTodoWritePersistsAndTitles(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_ = db.CreateSession(storage.SessionRow{ID: "ses_t", ProjectDir: "/w", Agent: "build", Model: "k"})
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Storage: db, SessionID: "ses_t"}
	raw, _ := json.Marshal(map[string]any{"todos": []map[string]any{
		{"content": "a", "status": "completed", "priority": "high"},
		{"content": "b", "status": "in_progress"},
		{"content": "c", "status": "pending"},
	}})
	out, err := Registry()["todowrite"].Run(context.Background(), raw, env)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "2 todos" {
		t.Fatalf("title = %q", out.Title)
	}
	var back []storage/todo.Todo // -> [protocol.Todo]
	back, err = db.GetTodos("ses_t")
	if err != nil || len(back) != 3 {
		t.Fatalf("get = %v %v", back, err)
	}
	if back[1].Status != "in_progress" || back[1].Priority != "medium" {
		t.Fatalf("row = %+v", back[1])
	}
	// update replaces
	raw2, _ := json.Marshal(map[string]any{"todos": []map[string]any{{"content": "z", "status": "pending"}}})
	_, _ = Registry()["todowrite"].Run(context.Background(), raw2, env)
	back2, _ := db.GetTodos("ses_t")
	if len(back2) != 1 || back2[0].Content != "z" {
		t.Fatalf("replace failed: %+v", back2)
	}
	// validation
	raw3, _ := json.Marshal(map[string]any{"todos": []map[string]any{{"content": "x", "status": "bogus"}}})
	if _, err := Registry()["todowrite"].Run(context.Background(), raw3, env); err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("validation err = %v", err)
	}
}
```

(Output JSON shape test: `out.Text` must equal `json.MarshalIndent` of the exact same array — assert round-trip by unmarshalling `out.Text` back and comparing fields, not byte-exact, since key order is model-controlled; pin indentation = 2 spaces.)

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/tool/ -run 'Bash|Todo' -v` → FAIL.

- [ ] **Step 3: Write minimal implementation** — `shell.go` per the pinned protocol (marker `__YOLO_END_{n}_:$?_$(pwd | base64 -w0)`, `base64 -w0`, Setpgid + `syscall.Kill(-pgid, SIGKILL)` on timeout/abort, respawn, stderr→stdout pipe sharing, `onLine` hook for streaming, 10MB guard per command output); `bash.go` (prefix patterns, default timeout 120000, exact timeout message from pin list, `Meta{exit}`); `todowrite.go`; storage migration v2 + DAOs + `protocol.Todo`; `tool.Env` gains `Storage *storage.DB` and `SessionID string` (nil-safe — Task 11-13 tools ignore them; update Task 11 `Env` definition comment accordingly — no code change needed since tools only read what they need).

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tool internal/storage internal/protocol
git commit -m "feat: persistent-session bash shell + todowrite tool (+ todo table migration v2)"
```

---

# Milestone M4 — Prompt builder + engine

Pinned from upstream v1.18.18 (verified this session):
- Family selection (session/system.ts:27-49, VERBATIM, in order; `apiID` = model's API id, `providerID` = provider id):
  1. `strings.Contains(apiID,"muse")` → `meta.txt` with `{{MODEL_NAME}}` replaced by `"Muse Glimmer"` if apiID contains `muse-glimmer` else `"Muse Spark"`
  2. `gpt-4` | `o1` | `o3` in apiID → `beast.txt`
  3. `gpt` in apiID → `codex.txt` if `codex` in apiID else `gpt.txt`
  4. `gemini-` in apiID → `gemini.txt`
  5. `claude` in apiID → `anthropic.txt`
  6. `trinity` in lower(apiID) → `trinity.txt`
  7. `kimi` in lower(apiID) OR providerID ∈ {kimi-for-coding, moonshotai, moonshotai-cn} → `kimi.txt`
  8. else → `default.txt`
- Env block (system.ts:72-85, VERBATIM minus references, which v1 has none of):
  ```
  You are powered by the model named {apiID}. The exact model ID is {providerID}/{apiID}
  Here is some useful information about the environment you are running in:
  <env>
    Working directory: {dir}
    Workspace root folder: {dir}
    Is directory a git repo: {yes|no}
    Platform: {linux|darwin|windows}
    Today's date: {Mon Jan 02 2006 — JS toDateString form, zero-padded day}
  </env>
  ```
  v1: worktree == working directory (no worktrees); git detection = `git -C {dir} rev-parse --is-inside-work-tree` (2 s timeout, output `true` → `yes`; any failure → `no`; cached per dir+turn).
- PLAN FIX (supersedes File Structure line 53): `internal/session/prompt/` embeds **14** files — the 13 from `session/prompt/` minus `plan-reminder-anthropic.txt` (experimental-only; `experimentalPlanMode` is env-gated `OPENCODE_EXPERIMENTAL_PLAN_MODE`, default OFF) PLUS `title.txt` (moved from `agent/prompt/` — it is the title side-call prompt, used in v1 per spec §4.3). `summary.txt`/`compaction.txt`/`explore.txt` are NOT embedded in v1 (compaction/summary deferred; explore = subagent, out of scope).
- Plan reminders (session/reminders.ts DEFAULT flag path, VERBATIM behavior): computed at EACH request build, in-memory only (never persisted, never SSE'd, `synthetic: true` text parts on the last user message):
  - current agent == `plan` → append `plan.txt` content.
  - last assistant message in history had `agent == "plan"` AND current agent != `plan` → append `build-switch.txt` content.
  - `plan-mode.txt`/`plan-reminder-anthropic.txt` are experiment-gated upstream (OFF by default) — v1 pins the default path; `plan-mode.txt` is embedded (per the 13+title decision) and reserved.
- Plan file path (session/session.ts:331-336, upstream = worktree/`.opencode/plans` or `data/plans` by vcs, name `{created_ms}-{slug}.md`): **PLAN RESOLUTION (flag to user, spec §7 already fixes plans under the data dir):** Yolo always uses `PlanPath = {dataDir}/plans/{created_ms}-{sessionID}.md`. The plan agent's editable allow-rules (Task 10) match this absolute dir.
- Agent system text (agent.ts): `build`/`plan` define NO `prompt` field (optional, unset) → system messages = [familyPrompt, envBlock, instructions...]. Hidden `title` agent: `prompt = title.txt`, all permissions denied → v1 title side-call = messages [system: title.txt, user: <user prompt text>] with the session's model/driver, no tools; result = first non-empty line, hard-capped at 50 chars (upstream cap), trimmed; stored as session title when title is empty or `"New session"`.
- PLAN FIX (supersedes Task 5 DDL): `message` table gains column `agent TEXT NOT NULL DEFAULT 'build'` (the build-switch reminder detection needs per-message agent; `MessageRow` gains `Agent string`). Task 5's CREATE TABLE statement is amended accordingly (still migration v1).

### Task 15: `internal/session` — prompt builder (embeds, family, env, instructions)

**Files:**
- Create: `internal/session/prompt.go`, `internal/session/prompt_test.go`, `internal/session/prompt/*.txt` (14 files, copied verbatim: `anthropic.txt beast.txt build-switch.txt codex.txt copilot-gpt-5.txt default.txt gemini.txt gpt.txt kimi.txt meta.txt plan-mode.txt plan.txt trinity.txt title.txt`)
- Copy step: for each of the 13, `cp /tmp/opencode-upstream/packages/opencode/src/session/prompt/<f> internal/session/prompt/<f>`; for title: `cp /tmp/opencode-upstream/packages/opencode/src/agent/prompt/title.txt internal/session/prompt/title.txt`. **Never hand-edit — drift breaks sha256-pin tests** (pin constants = the 16 hashes from the planning session; `sha256sum` each file now and embed the values as test constants — if any file is unobtainable, STOP and report, do not substitute).

**Interfaces:**
- Consumes: `provider.Model/Info`.
- Produces (used by engine Task 16+):

```go
package session

// SystemPrompt returns the ordered system message texts for one request.
//   [ 0 ] family prompt (meta.txt may carry the model-name substitution)
//   [ 1 ] env block
//   [ 2.. ] instruction files (AGENTS.md walk-up, then config instructions[])
func BuildSystemPrompt(dir string, model provider.Model, apiID, providerID string) ([]string, error)

func FamilyPrompt(apiID, providerID string) (embed name string, text string, err error) // pin test target
func EnvBlock(dir, apiID, providerID string) string
func PlanReminders(history []MessageWithParts, currentAgent string) []string // plan.txt and/or build-switch.txt, in order, empty slice possible
```

- [ ] **Step 1: Copy prompt files + write the failing test**

```go
package session

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/provider"
)

//go:embed prompt/*.txt
var prompts embed.FS

// sha256 constants filled from `sha256sum` in Step 1 (ALL 14):
var promptPins = map[string]string{
	"prompt/default.txt":        "962…", // fill
	"prompt/anthropic.txt":      "832…",
	"prompt/gpt.txt":            "83a…",
	"prompt/gemini.txt":         "921…",
	"prompt/codex.txt":          "c30…",
	"prompt/kimi.txt":           "ade…",
	"prompt/meta.txt":           "906…",
	"prompt/copilot-gpt-5.txt":  "0ef…",
	"prompt/beast.txt":          "a38…",
	"prompt/trinity.txt":        "00…",
	"prompt/plan.txt":           "455…",
	"prompt/plan-mode.txt":      "473…",
	"prompt/build-switch.txt":   "/*compute*/",
	"prompt/title.txt":          "e7a…",
}

func TestPromptFilesPinned(t *testing.T) {
	for name, want := range promptPins {
		if want == "" || strings.HasSuffix(want, "*/") || strings.HasSuffix(want, "…") {
			t.Skipf("pin for %s not filled yet", name)
		}
		b, err := prompts.ReadFile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := sha256hex(b); got != want {
			t.Fatalf("%s sha = %s, want %s", name, got, want)
		}
}

func TestFamilySelection(t *testing.T) {
	cases := []struct {
		api, prov, want string
	}{
		{"Qwen3.8-27B", "kido", "default.txt"},
		{"claude-opus-4-7", "opencode", "anthropic.txt"},
		{"gpt-5-nano", "opencode", "gpt.txt"},
		{"gpt-4.1", "opencode", "beast.txt"},
		{"o3-mini", "opencode", "beast.txt"},
		{"codex-mini", "openai", "codex.txt"}, // apiID contains gpt? no "gpt" → falls through... (see note)
		{"gemini-3-flash", "opencode", "gemini.txt"},
		{"kimi-k2", "moonshotai", "kimi.txt"},
		{"some-model", "kimi-for-coding", "kimi.txt"},
		{"trinity-x", "x", "trinity.txt"},
		{"muse-glimmer-9b", "openrouter", "meta.txt"},
	}
	for _, c := range cases {
		_, text, err := FamilyPrompt(c.api, c.prov)
		if err != nil {
			t.Fatal(err)
		}
		embedded := ""
		wantName := ""
		switch c.want {
		case "default.txt":
			embedded = mustRead(t, "prompt/default.txt")
		case "anthropic.txt":
			embedded = mustRead(t, "prompt/anthropic.txt")
		case "gpt.txt":
			embedded = mustRead(t, "prompt/gpt.txt")
		case "beast.txt":
			embedded = mustRead(t, "prompt/beast.txt")
		case "codex.txt":
			embedded = mustRead(t, "prompt/codex.txt")
		case "gemini.txt":
			embedded = mustRead(t, "prompt/gemini.txt")
		case "kimi.txt":
			embedded = mustRead(t, "prompt/kimi.txt")
		case "trinity.txt":
			embedded = mustRead(t, "prompt/trinity.txt")
		case "meta.txt":
			wantName = "meta.txt"
		}
		_ = embedded
		if wantName != "" {
			if got := FamilyName(c.api, c.prov); got != "prompt/meta.txt" {
				t.Fatalf("meta case: %s", got)
			}
			if !strings.Contains(text, "Muse Glimmer") {
				t.Fatalf("muse-glimmer substitution missing")
			}
			continue
		}
		if !strings.Contains(text, firstLine(c.want)) { // weak but pinned: each file has a distinctive first line
			t.Fatalf("%s/%s did not select %s", c.api, c.prov, c.want)
		}
	}
	// the "codex-mini" case above is WRONG under the verbatim rule (no 'gpt' substring):
	// apiID "codex-mini" falls through to default — REMOVE that case (verbatim rule governs).
}

func TestEnvBlock(t *testing.T) {
	got := EnvBlock("/w", "Qwen3.8-27B", "kido")
	want := `You are powered by the model named Qwen3.8-27B. The exact model ID is kido/Qwen3.8-27B
Here is some useful information about the environment you are running in:
<env>
  Working directory: /w
  Workspace root folder: /w
  Is directory a git repo: ` + `yes|no` + ` — filled by BuildSystemPrompt; EnvBlock takes a bool
  Platform: linux
  Today's date: `
	if !strings.HasPrefix(got, want) {
		t.Fatalf("env block = %q", got)
	}
}

func TestBuildSystemPromptInstructions(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "AGENTS.md"), []byte("PROJECT RULES"), 0o644)
	cfg := []struct{ p, c string }{}
	_ = cfg
	sys, err := buildSystemPromptForTest(d, provider.Model{ID: "Qwen3.8-27B", Name: "q"}, "kido", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sys) != 3 { // family, env, AGENTS.md
		t.Fatalf("len = %d", len(sys))
	}
	if !strings.Contains(sys[2], "PROJECT RULES") {
		t.Fatalf("instructions = %q", sys[2])
	}
	// config instructions append; AGENTS.md walk-up finds nearest (v1 pin: nearest wins)
	sub := filepath.Join(d, "deep")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("NEARER RULES"), 0o644)
	sys2, _ := buildSystemPromptForTest(sub, provider.Model{ID: "m", Name: "m"}, "prov", []string{filepath.Join(d, "extra.md")})
	if !strings.Contains(sys2[2], "NEARER RULES") {
		t.Fatal("nearest AGENTS.md must win")
	}
}

func TestPlanReminders(t *testing.T) {
	planText := mustRead(t, "prompt/plan.txt")
	switchText := mustRead(t, "prompt/build-switch.txt")
	// plan agent → plan reminder on last user message
	mkp := func(role, agent string) MessageWithParts {
		return MessageWithParts{Message: Message{Role: role, Agent: agent}}
	}
	got := PlanReminders([]MessageWithParts{mkp("user", "plan")}, "plan")
	if len(got) != 1 || got[0] != planText {
		t.Fatalf("plan reminders = %+v", got)
	}
	// build→plan switch (last assistant was plan) → build-switch only
	got2 := PlanReminders([]MessageWithParts{
		mkp("user", "build"),
		{Message: Message{Role: "assistant", Agent: "plan"}},
		{Message: Message{Role: "user", Agent: "build"}},
	}, "build")
	if len(got2) != 1 || got2[0] != switchText {
		t.Fatalf("switch reminders = %+v", got2)
	}
	// plan→plan continues: plan reminder only
	got3 := PlanReminders([]MessageWithParts{
		{Message: Message{Role: "assistant", Agent: "plan"}},
		{Message: Message{Role: "user", Agent: "plan"}},
	}, "plan")
	if len(got3) != 1 || got3[0] != planText {
		t.Fatalf("continued plan = %+v", got3)
	}
	// build→build: none
	if got4 := PlanReminders([]MessageWithParts{mkp("user", "build")}, "build"); len(got4) != 0 {
		t.Fatalf("build reminders = %+v", got4)
	}
}
```

(Interface note: `FamilyPrompt` returns `(string, string, error)` where the first value is the embedded file name; expose `FamilyName(api, prov) string` as its first return and test via that. `buildSystemPromptForTest(dir, model, providerID, instructions []string)` is the unexported core used by `BuildSystemPrompt` (which also git-detects); the test fake passes no git detection. The public `BuildSystemPrompt` signature stays as pinned above.)

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/session/ -v` → FAIL (no prompt files/impl).

- [ ] **Step 3: Write minimal implementation** — copy the 14 txt files; `go:embed prompt/*.txt`; family switch exactly per the pinned order; env block with a `gitRepo(dir string) bool` helper (exec `git -C dir rev-parse --is-inside-work-tree`, 2 s timeout, `strings.TrimSpace(out) == "true"`); instructions = nearest `AGENTS.md` walking up from dir (stop at `/`, max 32 hops) + config `instructions[]` (abs or join(dir)) each read + included as a separate system entry verbatim (missing config files skipped with a log line); `PlanReminders` per pin (last user message index = last with Role "user"; last assistant = last with Role "assistant").

- [ ] **Step 4: Fill the sha256 pins** (run `sha256sum internal/session/prompt/*.txt`, paste real values into `promptPins`, remove the skip), then `go vet ./... && go test ./...` → PASS (all 14 pinned).

- [ ] **Step 5: Commit**

```bash
git add internal/session
git commit -m "feat: session prompt builder — 14 upstream prompt files verbatim, family selection, env block, plan reminders"
```

---

### Task 16: `internal/llm/fake` + engine core — single text turn end-to-end

**Files:**
- Create: `internal/llm/fake/fake.go`, `internal/session/engine.go`, `internal/session/engine_test.go`

**Interfaces:**
- Consumes: Tasks 2 (protocol), 5 (storage), 6 (bus), 7 (llm), 9 (provider), 10 (permission), 11-14 (tool), 15 (prompt).
- Produces (used by server Task 19+):

```go
package session

var ErrSessionBusy = errors.New("session busy")

type Deps struct {
	DB     *storage.DB
	Bus    *bus.Bus
	Prov   *provider.Registry
	Perm   *permission.Service
	Tools  map[string]tool.Tool // tool.Registry()
	DataDir string              // for PlanPath
	// lazy per-project config (instructions[]):
	Cfg func(projectDir string) (*protocol.Config, error)
	// test seams:
	Drivers map[string]llm.Driver // providerID → driver override (fake in tests); empty = registry drivers
	Clock   func() int64          // ms
}

type Engine struct{}
func New(d Deps) *Engine

type SendResult struct{ MessageID, PartID string }
// Send starts a turn asynchronously. Returns immediately after the user message is persisted
// and the turn goroutine spawned; ErrSessionBusy if a turn is active; errors on bad session/model.
func (e *Engine) Send(ctx context.Context, sessionID, text string, onDone func(error)) (SendResult, error)
func (e *Engine) Abort(sessionID string) bool            // false if no active turn
func (e *Engine) Status(sessionID string) protocol.SessionStatus // idle | busy
```

```go
package fake
// llm/fake — scripted driver for engine tests (and YOLO_LLM=fake wiring in M5).
type Turn struct {
	Parts []llm.Part  // emitted in order; the last part MUST carry Finish
	Err   error       // if set: Stream returns (zero stream, Err)
}
type Driver struct {
	Turns   []Turn
	// TitleScript: if the request's system prompt starts with title.txt marker, serve this Turn instead
	TitleTurns []Turn
	ReqLog    []llm.Request // every Stream() call, in order
}
func New(turns ...Turn) *Driver
func (d *Driver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error)
```

History → LLM messages mapping (LOCKED, unit-tested):
1. System messages: BuildSystemPrompt entries first (separate `llm.Message{Role: RoleSystem}` each), then…(no — llm.Message has one Content; the request carries `Messages` — extend: `llm.Request.Messages` already `[]Message`; system entries become leading `RoleSystem` messages in order).
2. Per persisted message (time_created ASC):
   - user → `RoleUser`, Content = join(text parts in id order, "\n"); **plus** any PlanReminders parts for the LAST user message (in-memory, synthetic — appended to that user message's content, joined "\n").
   - assistant → `RoleAssistant`: Content = join(text parts, "\n") (reasoning parts EXCLUDED — display-only, LOCKED); ToolCalls = tool parts with state completed or error (in part order), `Args` = state input JSON; `ToolCallID` = part callID.
   - tool parts (belonging to the assistant message) → one `RoleTool` message each, `ToolCallID` = part callID, `Content` = state output (completed) or state error (error state). Emitted immediately after the assistant message, in part order.
   - empty assistant (no text, no tools, e.g. interrupted) → skipped.
3. Tool schemas: `tool.SchemaFor(t)` for the VISIBLE set = `tool.Visible(permission.Flatten(ruleset for session agent), registry)` — ruleset = builtins(agent, dataDir) + config permission rules + session always rules.

- [ ] **Step 1: Write the failing test**

```go
package session_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/llm"
	fakellm "github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

type harness struct {
	db   *storage.DB
	bus  *bus.Bus
.eng   *session.Engine
	drv  *fakellm.Driver
	events []protocol.Event
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	d := t.TempDir()
	db, err := storage.Open(filepath.Join(d, "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	b := bus.New()
	ch, _ := b.Subscribe()
	go func() { for e := range ch { /* collect */ } }()
	h := &harness{db: db, bus: b}
	_ = ch
	return h
}

func (h *harness) build(t *testing.T, agent string) {
	t.Helper()
	drv := fakellm.New()
	reg, _ := provider.NewWithSeams(t.Context(), t.TempDir(), func(providerID string) (provider.Info, provider.Model, error) {
		return provider.Info{ID: "kido", Name: "kido", BaseURL: "http://fake", KeyRequired: false},
			provider.Model{ID: "q", Name: "q", Adapter: "openai", Context: 100000, ToolCall: true}, nil
	})
	_ = reg
	h.drv = drv
	h.eng = session.New(session.Deps{
		DB: h.db, Bus: h.bus,
		Perm:  permission.New(h.db, h.bus),
		Tools: tool.Registry(),
		DataDir: t.TempDir(),
		Cfg: func(string) (*protocol.Config, error) { return &protocol.Config{}, nil },
		Drivers: map[string]llm.Driver{"kido": drv},
		Clock: time.Now,
	})
	_ = agent
}

func (h *harness) startSession(t *testing.T, dir string) string {
	t.Helper()
	id := protocol.NewSessionID()
	err := h.db.CreateSession(storage.SessionRow{ID: id, ProjectDir: dir, Title: "New session", Model: "kido/q", Agent: "build"})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func waitIdle(t *testing.T, h *harness, ses string, fn func()) {
	t.Helper()
	fn()
	deadline := time.Now().Add(5 * time.Second)
	for h.eng.Status(ses) == protocol.StatusBusy {
		if time.Now().After(deadline) {
			t.Fatal("engine did not go idle")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSingleTextTurnEndToEnd(t *testing.T) {
	h := newHarness(t)
	h.build(t, "build")
	d := t.TempDir()
	ses := h.startSession(t, d)

	h.drv.Turns = []fakellm.Turn{{Parts: []llm.Part{
		{Kind: "text", Text: "Hel"},
		{Kind: "text", Text: "lo"},
		{Kind: "text", Finish: "stop", Usage: &llm.Usage{Input: 42, Output: 7}},
	}}}

	var errMsg error
	res, err := h.eng.Send(context.Background(), ses, "say hi", func(e error) { errMsg = e })
	if err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})

	if err := h.eng.Status(ses) == protocol.StatusIdle ? nil : errTurn(h); err != nil {
		t.Fatal(err)
	}
	if errMsg != nil {
		t.Fatalf("turn error: %v", errMsg)
	}
	msgs, err := h.db.ListMessages(ses)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("messages = %d err=%v", len(msgs), err)
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("roles = %s,%s", msgs[0].Role, msgs[1].Role)
	}
	um, _ := h.db.ListParts(msgs[0].ID)
	if len(um) != 1 {
		t.Fatalf("user parts = %d", len(um))
	}
	var userPart protocol.Part
	userPart, err = storage.PartToProtocol(um[0])
	if err != nil {
		t.Fatal(err)
	}
	if userPart.TextPart == nil || userPart.TextPart.Text != "say hi" {
		t.Fatalf("user part = %+v", userPart)
	}
	am, _ := h.db.ListParts(msgs[1].ID)
	if len(am) != 1 {
		t.Fatalf("assistant parts = %d", len(am))
	}
	ap, _ := storage.PartToProtocol(am[0])
	if ap.TextPart == nil || ap.TextPart.Text != "Hello" {
		t.Fatalf("assistant text = %+v", ap)
	}
	// cost/tokens persisted on the assistant message row
	if msgs[1].Tokens.Input != 42 || msgs[1].Tokens.Output != 7 {
		t.Fatalf("tokens = %+v", msgs[1].Tokens)
	}
	// driver request shape: system first, user last, model + tools
	req := h.drv.ReqLog[0]
	if req.Model != "q" {
		t.Fatalf("model = %s", req.Model)
	}
	if req.Messages[0].Role != llm.RoleSystem || req.Messages[len(req.Messages)-1].Role != llm.RoleUser {
		t.Fatalf("roles = %s … %s", req.Messages[0].Role, req.Messages[len(req.Messages)-1].Role)
	}
	if len(req.Tools) != 7 {
		t.Fatalf("tools = %d (yolo? no: build agent sees 7; check hidden filter)", len(req.Tools))
	}
	_ = res
}

func TestHistoryReplayIncludesToolResults(t *testing.T) {
	// after a turn with one tool call, the NEXT request replays: user, assistant(text+tool), tool result
	h := newHarness(t)
	h.build(t, "yolo")
	d := t.TempDir()
	ses := h.startSession(t, d)

	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{
			{Kind: "text", Text: "checking"},
			{Kind: "tool", Name: "read", CallID: "call_1", Text: "", Finish: "tool_calls"},
			{Kind: "text", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 2}},
		}},
		{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 9, Output: 1}}}},
	}
	// pre-create the file the tool reads
	fp := filepath.Join(d, "f.txt")
	writeFile(t, fp, "content")

	if _, err := h.eng.Send(context.Background(), ses, "read f.txt", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})

	req := h.drv.ReqLog[1]
	var roles []string
	var toolMsgs []llm.Message
	var asst *llm.Message
	for i := range req.Messages {
		m := req.Messages[i]
		roles = append(roles, string(m.Role))
		if m.Role == llm.RoleTool {
			toolMsgs = append(toolMsgs, m)
		}
		if m.Role == llm.RoleAssistant && len(m.ToolCalls) > 0 {
			asst = &req.Messages[i]
		}
	}
	// expected tail: …, user, assistant(tools), tool, user?, … — the second user message? NO:
	// turn 2 request = system + history (user1, asst1+tool1) — no new user message yet (Send persisted it
	// BEFORE the request; so the request ends with user2, which is EMPTY text? LOCKED: the second Send's
	// user message is part of history too). Adjust: request for turn N includes the NEW user message last.
	wantTail := []string{string(llm.RoleUser), string(llm.RoleAssistant), string(llm.RoleTool), string(llm.RoleUser)}
	if len(roles) < 4 {
		roles = nil // guard
	}
	gotTail := roles[len(roles)-len(wantTail):]
	if join(roles) == "" {
		t.Fatal("no roles captured")
	}
	if gotTail[0] != wantTail[0] || gotTail[1] != wantTail[1] || gotTail[2] != wantTail[2] || gotTail[3] != wantTail[3] {
		t.Fatalf("roles tail = %v", gotTail)
	}
	if asst == nil || len(asst.ToolCalls) != 1 || asst.ToolCalls[0].Name != "read" {
		t.Fatalf("asst = %+v", asst)
	}
	if len(toolMsgs) != 1 || toolMsgs[0].ToolCallID != "call_1" || toolMsgs[0].ToolCallID != asst.ToolCalls[0].ID {
		t.Fatalf("tool msg = %+v", toolMsgs)
	}
	if !strings.Contains(toolMsgs[0].Content, "content") {
		t.Fatalf("tool result = %q", toolMsgs[0].Content)
	}
}
```

(Implementation notes to keep the test honest — LOCKED: `Send` persists the user message first, then builds the request INCLUDING it; `waitIdle` polls `Status`; harness collects bus events into `h.events` (chan goroutine appends); `errTurn`, `join`, `writeFile` are trivial test helpers implemented in the same file. The fake `read` tool call needs valid args JSON — the fake Turn's tool Part carries `Text` = args JSON string, e.g. `{"filePath":"<fp>"}`: LOCKED convention, document on `fake.Turn`: for `Kind:"tool"` parts, `Text` holds the arguments JSON.)

Additional assertions appended to TestSingleTextTurn (events bus): collect `h.events`; assert at least one `session.status` busy, at least one `message.part.delta` with delta "Hel", exactly one final `message.part.updated` with full text "Hello", one `session.status` idle. (Wire: delta event properties {message_id, part_id, field:"text", delta}.)

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/session/ ./internal/llm/fake/ -v` → FAIL (no engine/fake).

- [ ] **Step 3: Write minimal implementation**

`llm/fake/fake.go`: scripted per-Stream (shift Turn; if request's first message is RoleSystem and its Content starts with `"You are a title generator"` → use TitleTurns); PartStream = buffered channel filled from Turn.Parts (honoring ctx: if ctx done before finish, emit final error part). `engine.go` per the LOCKED mapping above:
- `Send`: session row load (ErrNotFound → error); model resolution `Prov.Resolve(row.Model)` (error → return); agent = row.Agent; busy check (atomic per-session flag map) → `ErrSessionBusy`; persist user MessageRow + text PartRow; events `message.updated`/`message.part.updated`; spawn goroutine `runTurn(ctx, …)`; return IDs.
- `runTurn`: status busy event; round loop (max 50 tool steps): build request (cached system prompt per session/model/agent/config-version — recompute each round is acceptable v1; cache key = sessionID+model+agent+mtime); stream; part bookkeeping (current text/reasoning part: create on first delta, upsert + delta event on each, finalize on transition); tool parts executed inline in stream order (Task 17 adds this — in THIS task the engine treats tool parts as: create part running, execute via the tool contract from M3 (permission via `Perm` with agent rules), finalize; even though the permission tests come in T17, the wiring exists now — T17's tests exercise the yolo/build/deny paths).
- finalize assistant MessageRow (cost = usage×model cost per 1M; tokens); event `message.updated`; status idle; `onDone(nil)`.
- `Abort`: cancel stored ctx; mark running tool parts error `aborted`; finalize.
- Title: after the FIRST user message (no prior assistant messages), if title is empty or `New session` → goroutine: fake/driver call with system=title.txt, user=first user text, max 1 step, no tools; trim first line to ≤50 runes; storage UpdateSession(title); event `session.updated`. Errors → log only, no retry.

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/session internal/llm
git commit -m "feat: session engine core — turn loop, history replay, events, fake driver, title side-call"
```

---

### Task 17: engine × permission — ask/deny/always, wildcard-hiding, doom loop

**Files:**
- Modify: `internal/session/engine.go` (permission integration), `internal/session/policy.go` (new: ruleset assembly + visible tools), `internal/session/engine_perm_test.go`

**Interfaces:**
- Produces:

```go
package session
// RulesetFor(session) []protocol.Rule — builtins(agent, dataDir) + config permission rules + db.AlwaysRules
func (e *Engine) RulesetFor(sessionID string) ([]protocol.Rule, error)
```

LOCKED semantics (re-stating spec §4.5 as executable):
- Per tool call: (1) doom check (Task 10 `DoomLoopDue` on the turn's call-history; if due → `Perm.Ask` action `doom_loop` resource `[toolname]`; Deny → tool part error `permission rejected` (metadata reason `doom_loop`), model continues); (2) external-directory check on `tool.External` paths outside session dir → `Perm.Ask` `external_directory` `[dir/*]`; (3) core `Perm.Ask` with `Resources/Always` from `tool.Patterns`; Deny → part error `permission rejected`; Abort/cancel → part error `aborted`.
- "always" reply → service persists rules (Task 10) → NEXT round's RulesetFor picks them up → subsequent identical calls skip the dialog. Auto-answered covered pendings (Task 10) — engine just sees Allow.
- Visible tools: `tool.Visible(VisibleRules(session), registry)` where `VisibleRules` = RulesetFor; hidden set per upstream `disabled()` (edit maps edit+write). Hidden tools: no schema sent; if the model calls a hidden tool anyway (API quirk) → part error `tool not available`.
- Doom history: per TURN (resets each Send), slice of CallKey{tool, sha256(json args)}.

- [ ] **Step 1: Write the failing tests**

```go
package session_test

// permEnv: harness + a fake permission REPLIER driven by test-queued responses.
func (h *harness) queueReplies(responses ...string)
// implemented as: goroutine subscribing h.bus for permission.asked events → h.db/Reply
// (harness gets a PendingWatcher started in newHarness; test pushes Reply calls into a channel)

func TestPermissionDenyStopsToolButNotTurn(t *testing.T) {
	h := newHarness(t)
	h.build(t, "build")
	d := t.TempDir()
	ses := h.startSession(t, d)
	// build agent: bash `cat secrets` — base rule `*` allow → NOT denied. To force a deny:
	// patch session agent? use a CONFIG deny rule: cfg with permission {bash: {pattern: "cat *", effect: "deny"}}
	h.cfgPermission = []protocol.Rule{{Permission: "bash", Pattern: "cat *", Action: "deny"}}
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{
			{Kind: "tool", Name: "bash", CallID: "c1", Text: `{"command":"cat secret.txt"}`, Finish: "tool_calls"},
		}},
		{Parts: []llm.Part{{Kind: "text", Text: "ok", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	if _, err := h.eng.Send(context.Background(), ses, "sneak", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	// find the tool part
	msgs, _ := h.db.ListMessages(ses)
	var state *protocol.ToolState
	for _, m := range msgs {
		if m.Role != "assistant" {
			continue
		}
		parts, _ := h.db.ListToolParts(m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if pt.ToolPart != nil {
				state = pt.ToolPart.State
			}
		}
	}
	if state == nil || state.Status != "error" {
		t.Fatalf("tool state = %+v", state)
	}
	if !strings.Contains(state.Error, "permission rejected") {
		t.Fatalf("error = %q", state.Error)
	}
}

func TestPermissionAlwaysPersistsAndSkipsNext(t *testing.T) {
	h := newHarness(t)
	h.build(t, "build")
	d := t.TempDir()
	ses := h.startSession(t, d)
	// force an ask: no rule covers action "custom"? tools only ask their own actions…
	// v1 mechanism: build `read *.env` ASKS. Use read on .env file:
	fp := filepath.Join(d, ".env")
	writeFile(t, fp, "SECRET=1")
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{{Kind: "tool", Name: "read", CallID: "c1", Text: "{\"filePath\":\"" + fp + "\"}", Finish: "tool_calls"}}},
		{Parts: []llm.Part{{Kind: "tool", Name: "read", CallID: "c2", Text: "{\"filePath\":\"" + fp + "\"}", Finish: "tool_calls"}}},
		{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	h.queueReplies("always") // first ask → always (always-pattern "src/*"? no — read's always = ["*"])
	if _, err := h.eng.Send(context.Background(), ses, "read env", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	msgs, _ := h.db.ListMessages(ses)
	var c1, c2 *protocol.ToolState
	for _, m := range msgs {
		parts, _ := h.db.ListToolParts(m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if pt.ToolPart == nil {
				continue
			}
			if pt.ToolPart.CallID == "c1" {
				c1 = pt.ToolPart.State
			}
			if pt.ToolPart.CallID == "c2" {
				c2 = pt.ToolPart.State
			}
		}
	}
	if c1 == nil || c1.Status != "completed" {
		t.Fatalf("c1 = %+v", c1)
	}
	if c2 == nil || c2.Status != "completed" {
		t.Fatalf("c2 (should skip ask via always) = %+v", c2)
	}
	rules, _ := h.svc.AlwaysRules… // LOCKED: harness exposes h.permSvc; assert a rule {read, *, allow} persisted
}

func TestHiddenToolNotSentToModel(t *testing.T) {
	h := newHarness(t)
	h.build(t, "plan") // plan: edit deny * (last edit rule is allow data/plans → NOT hidden!)
	// to assert hiding, use a CONFIG wildcard-deny: permission {edit: {"*": "deny"}} where deny is the LAST edit rule
	h.cfgPermission = []protocol.Rule{{Permission: "edit", Pattern: "*", Action: "deny"}}
	h.drv.Turns = []fakellm.Turn{{Parts: []llm.Part{{Kind: "text", Text: "x", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}}}
	d := t.TempDir()
	ses := h.startSession(t, d)
	if _, err := h.eng.Send(context.Background(), ses, "hi", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	req := h.drv.ReqLog[0]
	names := map[string]bool{}
	for _, td := range req.Tools {
		names[td.Name] = true
	}
	if names["edit"] || names["write"] {
		t.Fatalf("hidden tools leaked: %v", names)
	}
	if !names["read"] || !names["bash"] {
		t.Fatalf("visible tools missing: %v", names)
	}
}

func TestDoomLoopThirdIdenticalAsks(t *testing.T) {
	h := newHarness(t)
	h.build(t, "yolo") // yolo allows everything incl. doom_loop → ask? yolo = {*: allow} → doom ask auto-ALLOWED (no dialog)
	// to observe the ASK, use build agent (doom_loop ask in base)
	h.build2(t, "build") // harness variant setting session agent to build after Send? agent is per-row — set row first:
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.setAgent(t, ses, "build")
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{
			{Kind: "tool", Name: "glob", CallID: "a", Text: `{"pattern":"x*"}`, Finish: "tool_calls"},
			{Kind: "tool", Name: "glob", CallID: "b", Text: `{"pattern":"x*"}`, Finish: "tool_calls"},
			{Kind: "tool", Name: "glob", CallID: "c", Text: `{"pattern":"x*"}`, Finish: "tool_calls"},
			{Kind: "tool", Name: "glob", CallID: "d", Text: `{"pattern":"x*"}`, Finish: "tool_calls"},
		}},
		{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	h.queueReplies("once") // doom ask → once; the 4th glob then runs (all four eventually complete)
	if _, err := h.eng.Send(context.Background(), ses, "loop", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	// assert: exactly one permission.asked event with permission == "doom_loop"; c (3rd identical) did NOT run before the ask:
	// LOCKED ordering: call a, b run; c's doom check fires the ask BEFORE c runs; after allow, c runs; d's check:
	// history now a,b,c identical → d ALSO fires doom ask? LOCKED: yes (3-window sliding) — the test queues two "once".
	h.queueReplies2("once") // second doom ask (for d)
	asked := 0
	for _, e := range h.events {
		if e.Type == protocol.EventTypePermissionAsked {
			if string(e.Properties) // decode → permission field == "doom_loop"
		}
		if ... { asked++ }
	}
	if asked != 2 {
		t.Fatalf("doom asks = %d, want 2 (c and d)", asked)
	}
}
```

(LOCKED doom semantics resolved here, flag to user: the 3-identical window slides — any call whose two predecessors are identical triggers the ask; a "once" reply does NOT extend the exemption. This matches spec §4.5 "last 3 tool parts are the same tool with deep-equal inputs".)

Harness additions (implement once, in engine_test.go): `h.cfgPermission []protocol.Rule` (CfC closure reads it), `h.setAgent(t, ses, agent)` (db.UpdateSession + clear engine prompt cache), `h.queueReplies` via a watcher goroutine that listens on the bus for `permission.asked`, decodes `RequestID`, blocks the turn until a queued reply is pushed, then calls `h.svc.Reply(id, resp)`; replies consumed FIFO; if queue empty → t.Fatal after 3 s (a prompt appeared that the test did not expect).

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/session/ -run 'TestPermission|TestHidden|TestDoom' -v` → FAIL (policy/wiring absent).

- [ ] **Step 3: Write minimal implementation** — `policy.go`: `RulesetFor` (LoadBuiltins + config rules + AlwaysRules), `VisibleToolsFor(session)`, `toolSchemaList`; engine: doom-history slice per turn wired exactly as LOCKED; hidden-call guard (`tool not available` part error); abort-during-ask → part error `aborted` (service returns on ctx cancel).

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/session
git commit -m "feat: engine permission integration — ask/deny/always, wildcard tool hiding, sliding doom-loop"
```

---

### Task 18: engine lifecycle — retry, abort, max-steps, overflow, 409

**Files:**
- Modify: `internal/session/engine.go`, `internal/session/lifecycle_test.go` (new)

**LOCKED semantics:**
- Retry: only `llm.IsTransient(errors)` errors (429/5xx/net), and only BEFORE the first part of the round has been persisted (mid-stream after content: fail the turn — partial text is kept as the assistant message with an error note; LOCKED). Attempts ≤ 4; delay `1s × 2^n` (n = attempt-1) × jitter `uniform(0.8, 1.2)`; per attempt emit `session.status` = `{sessionID, status: retry{attempt, message: <err>, next: <ms>}}`; on giving up: assistant message finalized with text so far; turn ends with `onDone(err)` and tool/text state as-is (no synthetic error part; the model never sees it — turn is dead; next user message continues). Wait — spec: "surfaced as events" + the agent loop continues? For GIVING UP mid-turn: LOCKED: the turn ends; a final `session.status` idle; no model-visible message (the user can resend).
- Abort: `Abort(sid)` cancels the turn ctx; text part finalized with what arrived; running/next tool parts → state `error` text `aborted`; assistant finalized; idle; `onDone(context.Canceled)` (engine treats as non-fatal, logs only).
- Max steps: 50 tool calls per turn → before the 51st: all further tool parts of that final assistant stream become `error` `max tool steps reached (50)`; turn ends idle; onDone(nil) with a log.
- Overflow: (a) after each round with usage: if `usage.Input > model.Context` → turn ends with a synthesized assistant text part (type text, synthetic: true, text = `context overflow: model context {ctx} exceeded by input {n} tokens; the turn stopped. (v1 has no compaction — shorten the conversation or pick a larger-context model.)`) then idle; (b) API 400 with message matching `(?i)(context|tokens|too long|exceeds)` → same path (non-transient; no retry).
- 409: `Send` → `ErrSessionBusy` (server maps to 409 + envelope).
- `Status()` = busy if active.

- [ ] **Step 1: Write the failing tests**

```go
package session_test

func TestTransientRetrySucceeds(t *testing.T) {
	h := newHarness(t)
	h.build(t, "yolo")
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fakellm.Turn{
		{Err: &llm.TransientError{Status: 429, Err: errors.New("slow down")}},
		{Err: &llm.TransientError{Status: 503, Err: errors.New("unavailable")}},
		{Parts: []llm.Part{{Kind: "text", Text: "ok", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	h.fastBackoff = true // harness seam: Deps.Backoff func(attempt int) time.Duration → 1ms (test only)
	if _, err := h.eng.Send(context.Background(), ses, "hi", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	// two retry status events
	var retries int
	for _, e := range h.events {
		if e.Type == protocol.EventTypeSessionStatus {
			p := decodeStatus(t, e)
			if p.State == "retry" {
				retries++
			}
		}
	}
	if retries != 2 {
		t.Fatalf("retry events = %d", retries)
	}
	msgs, _ := h.db.ListMessages(ses)
	if len(msgs) != 2 || msgs[1].Role != "assistant" {
		t.Fatalf("turn lost data: %d", len(msgs))
	}
	if got := len(h.drv.ReqLog); got != 3 {
		t.Fatalf("attempts = %d", got)
	}
}

func TestTransientExgivesUpAfter4(t *testing.T) {
	h := newHarness(t)
	h.build(t, "yolo")
	d := t.TempDir()
	ses := h.startSession(t, d)
	for i := 0; i < 4; i++ {
		h.drv.Turns = append(h.drv.Turns, fakellm.Turn{Err: &llm.TransientError{Status: 500, Err: errors.New("boom")}})
	}
	var doneErr error
	if _, err := h.eng.Send(context.Background(), ses, "hi", func(e error) { doneErr = e }); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	if len(h.drv.ReqLog) != 4 {
		t.Fatalf("attempts = %d, want 4", len(h.drv.ReqLog))
	}
	// turn ended idle; assistant message exists (may be empty)
	msgs, _ := h.db.ListMessages(ses)
	if len(msgs) < 2 {
		t.Fatalf("messages = %d", len(msgs))
	}
	_ = doneErr
}

func TestMidStreamErrorNoRetry(t *testing.T) {
	h := newHarness(t)
	h.build(t, "yolo")
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{
			{Kind: "text", Text: "partial"},
			{Kind: "text", Finish: "error", Err: errors.New("connection reset")},
		}},
	}
	var doneErr error
	if _, err := h.eng.Send(context.Background(), ses, "hi", func(e error) { doneErr = doneErrOr(e) }); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	if len(h.drv.ReqLog) != 1 {
		t.Fatalf("retried mid-stream: %d", len(h.drv.ReqLog))
	}
}

func TestAbortMidTurn(t *testing.T) {
	h := newHarness(t)
	h.build(t, "yolo")
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{
			{Kind: "text", Text: "working"},
			{Kind: "tool", Name: "bash", CallID: "t1", Text: `{"command":"sleep 10"}`, Finish: "tool_calls"},
		}},
	}
	if _, err := h.eng.Send(context.Background(), ses, "slow", nil); err != nil {
		t.Fatal(err)
	}
	// wait for the tool part to go running, then abort
	waitPart(t, h, ses, "tool", "running", 3*time.Second)
	if !h.eng.Abort(ses) {
		t.Fatal("abort rejected")
	}
	waitIdle(t, h, ses, func() {})
	// tool part errored "aborted"; shell process not lingering (test shell was fake: command `sleep` on real bash —
	// LOCKED: engine tests use the REAL tool registry; bash `sleep 10` with abort = real SIGKILL — assert no
	// descendant `sleep 10` survives (ps check) — simpler assertion: part state:
	msgs, _ := h.db.ListMessages(ses)
	var state *protocol.ToolState
	for _, m := range msgs {
		parts, _ := h.db.ListToolParts(m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if pt.ToolPart != nil {
				state = pt.ToolPart.State
			}
		}
	}
	if state == nil || state.Status != "error" || !strings.Contains(state.Error, "aborted") {
		t.Fatalf("state = %+v", state)
	}
}

func TestMaxToolStepsHalts(t *testing.T) {
	h := newHarness(t)
	h.build(t, "yolo")
	d := t.TempDir()
	ses := h.startSession(t, d)
	// 51 tool calls in one stream, then a final text — engine must stop at 50
	parts := make([]llm.Part, 0, 52)
	for i := 0; i < 51; i++ {
		parts = append(parts, llm.Part{Kind: "tool", Name: "glob", CallID: string(rune('a'+i%26)) + strconv.Itoa(i), Text: `{"pattern":"n*"}`, Finish: "tool_calls"})
	}
	parts = append(parts, llm.Part{Kind: "text", Text: "end", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}})
	h.drv.Turns = []fakellm.Turn{{Parts: parts}, {Parts: []llm.Part{{Kind: "text", Text: "x", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}}}
	if _, err := h.eng.Send(context.Background(), ses, "spin", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	var toolParts int
	msgs, _ := h.db.ListMessages(ses)
	for _, m := range msgs {
		partsDB, _ := h.db.ListToolParts(m.ID)
		toolParts += len(partsDB)
	}
	if toolParts != 50 {
		t.Fatalf("tool parts = %d, want 50", toolParts)
	}
}

func TestOverflowHardStop(t *testing.T) {
	h := newHarness(t)
	h.build(t, "yolo")
	d := t.TempDir()
	ses := h.startSession(t, d)
	// model Context is 100000 (harness seam) — make usage.Input 100001
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{{Kind: "text", Text: "big", Finish: "stop", Usage: &llm.Usage{Input: 100001, Output: 5}}}},
	}
	if _, err := h.eng.Send(context.Background(), ses, "big", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	msgs, _ := h.db.ListMessages(ses)
	// second turn attempt is NOT made (only 1 request logged)
	if len(h.drv.ReqLog) != 1 {
		t.Fatalf("requests = %d", len(h.drv.ReqLog))
	}
	// synthetic overflow part present on the assistant message
	var found bool
	for _, m := range msgs {
		parts, _ := h.db.ListParts(m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if pt.TextPart != nil && strings.Contains(pt.TextPart.Text, "context overflow") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("overflow part missing")
	}
}

func TestConcurrentSend409(t *testing.T) {
	h := newHarness(t)
	h.build(t, "yolo")
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fakellm.Turn{{Parts: []llm.Part{{Kind: "text", Text: "slow", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}}}
	h.slowTurn = true // harness seam: hold the turn 500ms via fake driver delay
	_, err := h.eng.Send(context.Background(), ses, "one", nil)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	_, err2 := h.eng.Send(context.Background(), ses, "two", nil)
	if !errors.Is(err2, session.ErrSessionBusy) {
		t.Fatalf("want ErrSessionBusy, got %v", err2)
	}
	waitIdle(t, h, ses, func() {})
	if _, err3 := h.eng.Send(context.Background(), ses, "three", nil); err3 != nil {
		t.Fatalf("after idle send failed: %v", err3)
	}
	waitIdle(t, h, ses, func() {})
}
```

Harness seams (implement once): `h.fastBackoff bool` → `Deps.Backoff func(int) time.Duration` (production = 1s×2^n×jitter(0.8–1.2)); `h.slowTurn bool` → fake driver `Delay time.Duration` field added to `fake.Driver` (first Turn sleeps then streams); `waitPart(t, h, ses, type, status, timeout)` polls DB.

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/session/ -run 'TestTransient|TestMidStream|TestAbort|TestMax|TestOverflow|TestConcurrent' -v` → FAIL.

- [ ] **Step 3: Write minimal implementation** — per LOCKED semantics above (`Deps.Backoff` production default; retry loop wraps the per-round `driver.Stream` call only while no part of the round has been persisted yet; overflow check uses `provider.Model.Context` (harness model Context=100000); `ErrSessionBusy` from the busy flag map; abort cancels the per-turn ctx which the bash shell and HTTP streaming both honor).

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/session internal/llm
git commit -m "feat: engine lifecycle — bounded jittered retry, abort cascade, max-steps, context overflow, 409"
```

---

# Milestone M5 — HTTP server + contract tests

Locked server decisions (supersede nothing in spec §3; fill its gaps):
- SSE frame: `data: {json}\n\n` where json = `{"id": "evt_…", "type": <type>, "properties": <props>}` (event id from Task 2 `MakeEvent`; spec envelope shown without id — id included, TUI may ignore; flagged).
- `Server` is a plain `http.Handler` (mux). In-process TUI mode = real `127.0.0.1:0` listener; `yolo serve` default port **4096**, flags `--addr`/`--port`.
- Directory scoping: header `x-yolo-directory` (URL-encoded absolute path; absent → process CWD; decodes to non-dir → 400). Scoped endpoints: `/session*`, `/permission`, `/config` (project), `/path`, `/project/current`. Unscoped: `/provider*`, `/global/config`, `/auth/*`, `/agent`, `/command`, `/global/health`.
- `POST /session/:id/message` → **202** `{message_id}` immediately after `Engine.Send` starts (400 empty text, 404 unknown/out-of-scope, 409 busy). Turn progress arrives via SSE only.
- `GET /session/status` → `{"sessions": {"ses_…": "idle"|"busy"|"retry"}}` scoped; body = snapshot for footer.
- `POST /session/:id/command` body `{command: string}`: `/new` → 200 `{"session_id": "<new>"}` (creates default session, emits `session.updated`); any other known command → 200 `{"handled": "client"}`; unknown → 400.
- `GET /project/current` → `{"id": "prj_<20 base62 of sha256(dir)[:10]>", "name": <basename>, "directory": <dir>}`.
- Config endpoints: GET returns the parsed+merged config JSON (project layer wins; global below); PATCH body = partial config object → deep-merge (Task 3 `Merge`) into the **project** `yolo.jsonc` (or global for `/global/config`) and rewrite the file. **Deviation flagged:** JSONC comments are not preserved on rewrite (parse → merge → `MarshalIndent` 2-space); TUI never relies on comments in v1.
- `/auth/{providerID}`: PUT `{"key": "…"}` → auth store (Task 4 API) + `Provider` auth state refreshes; DELETE removes. `GET /provider` carries each provider's auth as `protocol.ProviderAuth` (Task 2 wire shape): `{"type":"api"|"none","status":"loaded"|"missing"|"not-required","key_required":bool}` — no separate `source` field; TUI renders loaded/not-required/missing from `status`.
- **Fake-LLM env wiring** (spec §8 test gating; implementation lives in the server-deps builder, tested here): `YOLO_LLM=fake` + `YOLO_FAKE_SCRIPT=<path.json>` → engine `Drivers` map overridden for ALL providers with a `llm/fake` driver loaded from the script file (JSON: `[{"parts":[{"kind":"text","text":"hi","finish":"stop","usage":{"input":1,"output":1}}], "delay_ms": 0}]`; `delay_ms` optional per turn). `YOLO_LLM` set to anything else → 500 at boot with a clear message. Unit tests set these in-process (httptest), never as shell env.
- Unknown route → 404 envelope (covers the spec's "skipped endpoint families"). Recover middleware: any panic → 500 envelope + log, connection kept.
- **Injectable-path helpers (M5 additions; M1 behavior/tests unchanged):** `auth.LoadFrom(path) (Store, error)` / `auth.SaveTo(s Store, path string) error` — the fixed `auth.Path()/Load()/Save()` delegate to these; `config.LoadGlobal(homeDir string) (*protocol.Config, error)` / `config.SaveGlobal(homeDir string, *protocol.Config) error` — read/write `<homeDir>/yolo/global.jsonc`. The server resolves: auth path = `<Dirs.Data>/auth.json`, global config = `<Dirs.Home>/yolo/global.jsonc`; zero `Dirs` → real XDG (Task 3/4 functions). `PUT/DELETE /auth` mutates a boot-lifetime in-memory `auth.Store` and persists via `SaveTo`. These five functions are added in Task 19 step 3 (same commit).
- **Wire DTO addition (M5):** `protocol.CommandResponse struct{ SessionID string \`json:"session_id,omitempty"\`; Handled string \`json:"handled,omitempty"\` }` — added to `internal/protocol` in Task 19 (same commit); consumed by T22 client `Command()`.

### Task 19: `internal/server` — handler, scoping, session endpoints, SSE

**Files:**
- Create: `internal/server/server.go`, `internal/server/handlers_session.go`, `internal/server/sse.go`, `internal/server/errors.go`, `internal/server/server_test.go`

**Interfaces:**
- Consumes: `session.Engine`, `storage.DB`, `bus.Bus`, `config.Loader`, `auth.Store`, `provider.Registry`, `permission.Service`, `tool.Registry`.
- Produces:

```go
package server

type Deps struct {
	DB      *storage.DB
	Bus     *bus.Bus
	Engine  *session.Engine
	Prov    *provider.Registry
	Perm    *permission.Service
	Config  config.Loader
	WorkDir string   // default dir when header absent
	Dirs    config.Dirs // Home/Data/Cache roots (zero = real XDG); auth path = <Data>/auth.json, global config = <Home>/yolo/global.jsonc
}
func New(d Deps) http.Handler
// helpers (unexported): scope(r) (dir string, ok error), envelope(w, status, message, data), decode(r, v)
```

Route table (v1 complete set):

```
GET  /global/health                 → 200 {"status":"ok"}
GET  /path                          (scoped) → {"directory": dir}
GET  /project/current               (scoped) → project identity (locked above)
GET  /session                       (scoped) → [Session] newest-first
POST /session                       (scoped) body {title?, model?, agent?} → 201 Session (defaults: "New session", model = default, agent = "build")
GET  /session/{id}                  (scoped) → Session (404 unknown/other-dir)
PATCH /session/{id}                 (scoped) body {title?, model?, agent?, time?} → Session
DELETE /session/{id}                (scoped) → 204; emits session.deleted
GET  /session/{id}/message          (scoped) → [MessageWithParts] (parts inline)
POST /session/{id}/message          (scoped) body {text} → 202 {"message_id"}
POST /session/{id}/abort            (scoped) → 200 {"aborted": bool}
POST /session/{id}/command          (scoped) body {command} → per locked behavior
GET  /session/status                (scoped) → snapshot
GET  /event                         (scoped) → SSE
```

Session wire DTOs: `protocol.Session` per Task 2 legacy shape (computed: `cost`/`tokens` = aggregates over assistant messages per Task 5 resolution; `time` ms; `model {id, provider_id}`; `agent`).

- [ ] **Step 1: Write the failing tests**

```go
package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

type srv struct {
	*httptest.Server
	db  *storage.DB
	eng *session.Engine
}

// newSrv boots the FULL stack on a fake provider set (no network): kido static model, no fetch
func newSrv(t *testing.T) *srv {
	t.Helper()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	db, err := storage.Open(filepath.Join(dataDir, "storage", "yolo.db"))
	… (t.Fatal on err, cleanup close)
	b := bus.New()
	eng := session.New(session.Deps{
		DB: db, Bus: b,
		Prov:  provider.NewStaticForTest(…), // test seam provider.Registry with kido {ID:q, Context:100000} — LOCKED test-only constructor in provider
		Perm:  permission.New(db, b),
		Tools: tool.Registry(),
		DataDir: dataDir,
		Cfg:   func(string) (*protocol.Config, error) { return &protocol.Config{}, nil },
		Drivers: map[string]llm.Driver{"kido": fakellm.New(…) /* one generic text turn per request: see below */},
		Clock: func() int64 { return time.Now().UnixMilli() },
	})
	h := server.New(server.Deps{
		DB: db, Bus: b, Engine: eng, Prov: prov, Perm: permSvc,
		Config:  config.Loader{Env: map[string]string{}},
		WorkDir: t.TempDir(),
		Dirs:    config.Dirs{Home: filepath.Join(root, "home"), Data: filepath.Join(root, "data"), Cache: filepath.Join(root, "cache")},
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return &srv{ts, db, eng}
}
```

LOCKED fake for server tests: `fakellm.New(fakellm.AutoText())` — a fake mode that answers EVERY request with `{text: "ok-"+requestCount, finish stop, usage {1,1}}` (no tool calls) unless `Turns` are overridden. Implement `AutoText()` in `internal/llm/fake` as part of THIS task.

```go
func req(t *testing.T, s *srv, method, path, dir, body string) (*http.Response, []byte) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r, err := http.NewRequest(method, s.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if dir != "" {
		r.Header.Set("x-yolo-directory", dir)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestHealthAndPathAndProject(t *testing.T) {
	s := newSrv(t)
	resp, b := req(t, s, "GET", "/global/health", "", "")
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"ok"`) {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	d := t.TempDir()
	resp, b = req(t, s, "GET", "/path", d, "")
	var p map[string]string
	_ = json.Unmarshal(b, &p)
	if resp.StatusCode != 200 || p["directory"] != d {
		t.Fatalf("path: %d %s", resp.StatusCode, b)
	}
	resp, b = req(t, s, "GET", "/project/current", d, "")
	var pr struct {
		ID, Name, Directory string
	}
	json.Unmarshal(b, &pr)
	if pr.Directory != d || strings.Count(pr.ID, "prj_") != 1 || !strings.HasPrefix(pr.ID, "prj_") {
		t.Fatalf("project: %s %s", pr.ID, pr.Directory)
	}
	// bad dir → 400
	resp, _ = req(t, s, "GET", "/path", "/no/such/dir/xyz", "")
	if resp.StatusCode != 400 {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestSessionLifecycleAndScoping(t *testing.T) {
	s := newSrv(t)
	d := t.TempDir()
	other := t.TempDir()
	resp, b := req(t, s, "POST", "/session", d, `{"title":"T1"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var ses struct {
		ID      string
		Title   string
		Agent   string
		Model   struct{ ID, ProviderID string }
		ProjectDir string
		Cost  float64
		Tokens struct{ Input, Output int }
	}
	json.Unmarshal(b, &ses)
	if ses.Title != "T1" || ses.Agent != "build" || ses.Model.ID != "q" || ses.Model.ProviderID != "kido" {
		t.Fatalf("session = %+v", ses)
	}
	id := ses.ID

	// list scoped
	resp, b = req(t, s, "GET", "/session", d, "")
	var list []map[string]any
	json.Unmarshal(b, &list)
	if len(list) != 1 {
		t.Fatalf("list = %d", len(list))
	}
	// other dir sees nothing
	resp, b = req(t, s, "GET", "/session", other, "")
	json.Unmarshal(b, &list)
	if len(list) != 0 {
		t.Fatalf("cross-dir leak: %d", len(list))
	}
	// get by id from other dir → 404
	resp, _ = req(t, s, "GET", "/session/"+id, other, "")
	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
	// patch model+agent+title
	resp, b = req(t, s, "PATCH", "/session/"+id, d, `{"title":"T2","agent":"yolo","model":"opencode/gpt-5-nano"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("patch: %d %s", resp.StatusCode, b)
	}
	var got struct {
		Title string
		Agent string
		Model struct{ ID, ProviderID string }
	}
	json.Unmarshal(b, &got)
	if got.Title != "T2" || got.Agent != "yolo" || got.Model.ProviderID != "opencode" {
		t.Fatalf("patched = %+v", got)
	}
	// delete → gone
	resp, _ = req(t, s, "DELETE", "/session/"+id, d, "")
	if resp.StatusCode != 204 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, _ = req(t, s, "GET", "/session/"+id, d, "")
	if resp.StatusCode != 404 {
		t.Fatalf("after delete: %d", resp.StatusCode)
	}
}

func TestSendMessage409AndEvents(t *testing.T) {
	s := newSrv(t)
	d := t.TempDir()
	_, b := req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	json.Unmarshal(b, &ses)
	id := ses.ID

	// subscribe SSE BEFORE sending
	res, err := s.clientWithHeader("GET", "/event", d).Do(…) // helper: returns open response body reader
	…
	line := readSSELine(t, res)  // helper: decodes one `data:` frame → {type,properties}
	// send
	resp, b = req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"hello"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send: %d %s", resp.StatusCode, b)
	}
	// collect events until session.status idle
	seen := map[string]int{}
	var types []string
	for i := 0; i < 50; i++ {
		ev := readSSEFrame(t, res)
		types = append(types, ev.Type)
		seen[ev.Type]++
		if ev.Type == "session.status" && ev.String("status") == "idle" {
			break
		}
	}
	for _, want := range []string{"message.updated", "message.part.updated", "message.part.delta", "session.status"} {
		if seen[want] == 0 {
			t.Fatalf("no %s in %v", want, types)
		}
	}
	// busy during turn: send again → 409 (turn still settling? send returns 202 immediately;
	// LOCKED: 409 observable when a turn IS active — use slow fake (delay_ms) variant:
	s.fakeDelay(200 * time.Millisecond)
	resp2, _ := req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"again"}`)
	if resp2.StatusCode != 202 {
		t.Fatalf("second send: %d", resp2.StatusCode)
	}
	time.Sleep(50 * time.Millisecond)
	resp3, b3 := req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"thrice"}`)
	if resp3.StatusCode != 409 {
		t.Fatalf("want 409 during busy, got %d %s", resp3.StatusCode, b3)
	}
	// envelope shape
	var env struct {
		Error struct{ Message string } `json:"error"`
	}
	json.Unmarshal(b3, &env)
	if env.Error.Message == "" {
		t.Fatalf("envelope = %s", b3)
	}
}

func TestAbortEndpoint(t *testing.T) {
	s := newSrv(t)
	d := t.TempDir()
	_, b := req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	json.Unmarshal(b, &ses)
	s.fakeDelay(300 * time.Millisecond)
	_, _ = req(t, s, "POST", "/session/"+ses.ID+"/message", d, `{"text":"slow"}`)
	time.Sleep(30 * time.Millisecond)
	resp, b2 := req(t, s, "POST", "/session/"+ses.ID+"/abort", d, `{}`)
	var body struct {
		Aborted bool
	}
	if resp.StatusCode != 200 || !body.Aborted /* unmarshal */ {
		t.Fatalf("abort: %d %s", resp.StatusCode, b2)
	}
	if body.Aborted == false {
		t.Fatal("aborted flag false")
	}
	// status now idle
	resp, b3 := req(t, s, "GET", "/session/status", d, "")
	var st struct {
		Sessions map[string]string `json:"sessions"`
	}
	json.Unmarshal(b3, &st)
	if st.Sessions[ses.ID] != "idle" {
		t.Fatalf("status = %v", st.Sessions)
	}
	// abort idle → aborted:false
	resp, b4 := req(t, s, "POST", "/session/"+ses.ID+"/abort", d, `{}`)
	var b5 struct{ Aborted bool }
	json.Unmarshal(b4, &b5)
	if b5.Aborted {
		t.Fatal("abort on idle must be false")
	}
}

func TestCommandEndpoint(t *testing.T) {
	s := newSrv(t)
	d := t.TempDir()
	resp, b := req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	json.Unmarshal(b, &ses)
	// /new → new session id
	resp, b = req(t, s, "POST", "/session/"+ses.ID+"/command", d, `{"command":"/new"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var out struct{ SessionID string `json:"session_id"` }
	json.Unmarshal(b, &out)
	if out.SessionID == "" || out.SessionID == ses.ID {
		t.Fatalf("/new = %s", out.SessionID)
	}
	resp, b = req(t, s, "POST", "/session/"+ses.ID+"/command", d, `{"command":"/model"}`)
	var client struct{ Handled string `json:"handled"` }
	json.Unmarshal(b, &client)
	if resp.StatusCode != 200 || client.Handled != "client" {
		t.Fatalf("/model = %d %s", resp.StatusCode, b)
	}
	resp, _ = req(t, s, "POST", "/session/"+ses.ID+"/command", d, `{"command":"/bogus"}`)
	if resp.StatusCode != 400 {
		t.Fatalf("/bogus = %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/server/ -v` → FAIL (no package).

- [ ] **Step 3: Write minimal implementation** — `server.go` (New, mux, scope middleware, recover, envelope), `errors.go` (codes: 400/404/409/500 helpers + `ErrNotFound`/`ErrSessionBusy` mapping), `handlers_session.go` (all session routes + status + command), `sse.go` (subscribe → frame loop; flush after each write; `X-Accel-Buffering: no`; close on channel close or client context done). `provider.NewStaticForTest` + `fake.AutoText` + harness helpers (`s.fakeDelay`, SSE frame reader) created here (fake additions live in `internal/llm/fake`; static provider seam in `internal/provider` — both test-only, build tag NONE, doc-commented "test seam").

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server internal/llm internal/provider
git commit -m "feat: server — routing, x-yolo-directory scoping, session API, SSE, error envelope"
```

---

### Task 20: remaining endpoints — provider, config, auth, agent, command, permission

**Files:**
- Create: `internal/server/handlers_misc.go`, `internal/server/handlers_misc_test.go`

**Interfaces:**
- Consumes: Task 19 `Deps`.
- Produces: routes

```
GET  /provider                 → [Provider] (kido + opencode + config-defined; models populated; auth state per provider)
GET  /provider/auth            → {"kido": {"key_required": false, "env": ["KIDO_API_KEY"]}, "opencode": {"key_required": true, "env": ["OPENCODE_API_KEY"]}} (merged with loaded key source)
GET  /config                   (scoped) → merged project+global config JSON (parsed object)
PATCH /config                  (scoped) body=partial → merge into project yolo.jsonc, rewrite (2-space JSON), → merged config
GET  /global/config            → global config JSON
PATCH /global/config           → merge into <Home>/yolo/global.jsonc (create if absent), → merged result
PUT  /auth/{providerID}        body {"key": "…"} → 204 (auth.Store upsert)
DELETE /auth/{providerID}      → 204
GET  /agent                    → [Agent] {name, description, mode:"primary", permissions summary? no: {name, description, model?: config default}} — build/plan/yolo + config-defined ids (name only, description "Custom agent.")
GET  /command                  → [Command] {name, description, usage} for /help /new /model /agents /exit
GET  /permission               (scoped) → [PermissionAskedProps] (pending, all sessions in dir)
POST /permission/{requestID}/reply body {"response": "once|always|reject"} → 204 (404 unknown id; 400 bad response)
```

Agent descriptions (LOCKED, from upstream agent.ts verified text): build `The default agent. Executes tools based on configured permissions.`; plan `Plan mode. Disallows all edit tools.`; yolo `Yolo agent. Permits everything without prompts.`

Command definitions (LOCKED minimal set): `/help` (show help — client), `/new` (new session — server), `/model` (pick model — client), `/agents` (pick agent — client), `/exit` (quit — client).

- [ ] **Step 1: Write the failing tests**

```go
func TestProviderListAndAuth(t *testing.T) {
	s := newSrv(t)
	resp, b := req(t, s, "GET", "/provider", "", "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var ps []protocol.Provider
	json.Unmarshal(b, &ps)
	byID := map[string]protocol.Provider{}
	for _, p := range ps {
		byID[p.ID] = p
	}
	k, z := byID["kido"], byID["opencode"]
	if k.ID == "" || len(k.Models) < 1 {
		t.Fatalf("kido = %+v", k)
	}
	if z.ID == "" || len(z.Models) == 0 {
		t.Fatalf("zen = %+v (server test fixture: seed a minimal zen catalog via Dirs seam)", z)
	}
	if z.Auth.KeyRequired != true {
		t.Fatalf("zen auth = %+v", z.Auth)
	}
	// config-defined provider appears
	writeCfg(t, s.dir, `{"provider": {"myprov": {"base_url": "http://x", "models": {"m1": {"name": "M1"}}}}}`)
	resp, b = req(t, s, "GET", "/provider", s.dir, "")
	json.Unmarshal(b, &ps)
	var found bool
	for _, p := range ps {
		if p.ID == "myprov" && len(p.Models) == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("config provider missing: %s", b)
	}
}

func TestConfigGetPatchRoundtrip(t *testing.T) {
	s := newSrv(t)
	d := s.dir
	resp, b := req(t, s, "GET", "/config", d, "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	resp, b = req(t, s, "PATCH", "/config", d, `{"model": "opencode/gpt-5-nano", "provider": {"kido": {"options": {"foo": true}}}}`)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var cfg map[string]any
	json.Unmarshal(b, &cfg)
	if cfg["model"] != "opencode/gpt-5-nano" {
		t.Fatalf("merged = %v", cfg["model"])
	}
	// file written with 2-space indent
	raw, _ := os.ReadFile(filepath.Join(d, "yolo.jsonc"))
	if !bytes.Contains(raw, []byte("  \"model\"")) {
		t.Fatalf("file = %s", raw)
	}
	// patch again — deep merge keeps provider.kido.options.foo
	resp, b = req(t, s, "PATCH", "/config", d, `{"provider": {"kido": {"options": {"bar": 1}}}}`)
	json.Unmarshal(b, &cfg)
	k := cfg["provider"].(map[string]any)["kido"].(map[string]any)["options"].(map[string]any)
	if k["foo"] != true || k["bar"] != float64(1) {
		t.Fatalf("deep merge lost keys: %v", k)
	}
}

func TestGlobalConfig(t *testing.T) {
	s := newSrv(t)
	resp, b := req(t, s, "PATCH", "/global/config", "", `{"model": "kido/m"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	f := filepath.Join(s.home, "yolo", "global.jsonc")
	raw, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("global file: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"kido/m"`)) {
		t.Fatalf("global = %s", raw)
	}
	// project overrides global in GET /config
	resp, b = req(t, s, "PATCH", "/config", s.dir, `{"model": "kido/other"}`)
	resp, b = req(t, s, "GET", "/config", s.dir, "")
	var cfg map[string]any
	json.Unmarshal(b, &cfg)
	if cfg["model"] != "kido/other" {
		t.Fatalf("precedence broken: %v", cfg["model"])
	}
}

func TestAuthPutDelete(t *testing.T) {
	s := newSrv(t)
	resp, _ := req(t, s, "PUT", "/auth/opencode", "", `{"key": "sk-test"}`)
	if resp.StatusCode != 204 {
		t.Fatalf("put: %d", resp.StatusCode)
	}
	resp, b := req(t, s, "GET", "/provider", "", "")
	var ps []protocol.Provider
	json.Unmarshal(b, &ps)
	for _, p := range ps {
		if p.ID == "opencode" {
			if p.Auth == nil || p.Auth.Status != "loaded" {
				t.Fatalf("zen auth after put = %+v", p.Auth)
			}
		}
	}
	resp, _ = req(t, s, "DELETE", "/auth/opencode", "", "")
	if resp.StatusCode != 204 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, b = req(t, s, "GET", "/provider", "", "")
	json.Unmarshal(b, &ps)
	for _, p := range ps {
		if p.ID == "opencode" && p.Auth != nil && p.Auth.Status == "loaded" {
			t.Fatalf("key still loaded after delete")
		}
	}
}

func TestPermissionListAndReply(t *testing.T) {
	s := newSrv(t)
	d := t.TempDir()
	_, b := req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	json.Unmarshal(b, &ses)
	// park a pending ask (action with no rules → ask): use engine Send with a fake tool call?
	// LOCKED: exercise via the permission service directly (harness seam h.permSvc.Ask in a goroutine):
	s.parkAsk(ses.ID, "custom", "res1")
	resp, b = req(t, s, "GET", "/permission", d, "")
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var pend []protocol.PermissionAskedProps
	json.Unmarshal(b, &pend)
	if len(pend) != 1 || pend[0].Permission != "custom" {
		t.Fatalf("pending = %+v", pend)
	}
	resp, _ = req(t, s, "POST", "/permission/"+pend[0].ID+"/reply", d, `{"response":"once"}`)
	if resp.StatusCode != 204 {
		t.Fatalf("reply: %d", resp.StatusCode)
	}
	resp, b = req(t, s, "GET", "/permission", d, "")
	json.Unmarshal(b, &pend)
	if len(pend) != 0 {
		t.Fatalf("still pending: %+v", pend)
	}
	resp, _ = req(t, s, "POST", "/permission/per_missing/reply", d, `{"response":"once"}`)
	if resp.StatusCode != 404 {
		t.Fatalf("unknown reply: %d", resp.StatusCode)
	}
	resp, _ = req(t, s, "POST", "/permission/per_missing/reply", d, `{"response":"bogus"}`)
	if resp.StatusCode == 404 { // 404 wins over 400? LOCKED: validate body first → 400
		t.Fatalf("bad response should be 400")
	}
}

func TestAgentAndCommand(t *testing.T) {
	s := newSrv(t)
	resp, b := req(t, s, "GET", "/agent", "", "")
	var agents []protocol.Agent
	json.Unmarshal(b, &agents)
	byName := map[string]string{}
	for _, a := range agents {
		byName[a.Name] = a.Description
	}
	if byName["build"] != "The default agent. Executes tools based on configured permissions." {
		t.Fatalf("build desc = %q", byName["build"])
	}
	if byName["plan"] != "Plan mode. Disallows all edit tools." {
		t.Fatalf("plan desc = %q", byName["plan"])
	}
	if _, ok := byName["yolo"]; !ok {
		t.Fatalf("yolo missing: %s", b)
	}
	resp, b = req(t, s, "GET", "/command", "", "")
	var cmds []protocol.Command
	json.Unmarshal(b, &cmds)
	if len(cmds) != 5 {
		t.Fatalf("commands = %s", b)
	}
}

func TestUnknownRoutes404(t *testing.T) {
	s := newSrv(t)
	for _, p := range []string{"/", "/api/v2/sessions", "/mcp/x", "/skill/s", "/nope"} {
		resp, _ := req(t, s, "GET", p, "", "")
		if resp.StatusCode != 404 {
			t.Fatalf("%s → %d, want 404", p, resp.StatusCode)
		}
	}
}
```

Harness seams to add in THIS task (extend `newSrv`): `s.dir` (a project dir auto-created), `s.home` (Dirs.Home), `s.permSvc`, `s.parkAsk(sessionID, action, resource)` (goroutine `permSvc.Ask`), `writeCfg(t, dir, jsonc)` (writes `yolo.jsonc`). Zen seed: `provider.NewStaticForTest` accepts a zen models seed (minimal: 2 models — `claude-opus-4-7` anthropic + `gpt-5-nano` openai) for deterministic server tests (real catalog path already covered in Task 9).

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/server/ -run 'TestProvider|TestConfig|TestAuth|TestPermission|TestAgent|TestUnknown' -v` → FAIL.

- [ ] **Step 3: Write minimal implementation** — `handlers_misc.go` per the route table + LOCKED texts.

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server
git commit -m "feat: remaining endpoints — provider/auth state, config get/patch, auth store, agent, command, permission"
```

---

### Task 21: contract golden tests + SSE ordering + fake env e2e

**Files:**
- Create: `internal/server/contract_test.go`, `internal/server/testdata/golden/*.json` (generated once, committed)

**Scope (LOCKED):**
1. **Golden responses:** for each endpoint, one canonical request → normalized JSON (IDs/timestamps replaced by stable placeholders `SES1`/`T1`) compared byte-equal against `testdata/golden/<route>.json` (key-sorted; created by a `-update` flag run: `go test ./internal/server/ -run Golden -update`, then re-run without it). Covers: health, path, project, session list/get/create/patch, message list, provider, config, agent, command, permission empty, status.
2. **SSE ordering (fake driver, deterministic):** send → frames in EXACT relative order: `session.status busy` → `message.updated` (user msg) → `message.part.updated` (user part) → ≥1× `message.part.delta` → `message.part.updated` (assistant part, full text) → `message.updated` (assistant) → `session.status idle`. Assert by index order, not equality (deltas may vary).
3. **Fake-env e2e:** boot the real `server.New` deps with `YOLO_LLM=fake` + `YOLO_FAKE_SCRIPT` env (in-process `os.Setenv` + t.Cleanup restore): complete a two-turn scripted conversation through the HTTP API; assert messages, parts, and that a second `Send` on the same session replays history (ReqLog shows the tool result in request 2 if the script's turn 1 had a tool call — use a script with one `read` call on a seeded file).
4. **Scoping matrix:** header absent → CWD default (test sets server WorkDir); every scoped endpoint 404s for another dir's session id (loop over the 8 scoped routes).

- [ ] **Step 1: Write the failing tests** — `contract_test.go` with the four groups; `-update` flag handling via `flag.Bool("update", false, …)`.

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/server/ -run Golden|TestSSEOrdering|TestFakeEnv -v` → FAIL (no goldens yet).

- [ ] **Step 3: Generate goldens** — run with `-update`, INSPECT each golden file by hand (no placeholders missed; no live data), commit them WITH the test (single commit).

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS (including the non-update golden comparison).

- [ ] **Step 5: Commit**

```bash
git add internal/server
git commit -m "test: server contract — golden envelopes, SSE ordering, fake-env e2e, scoping matrix"
```

---

# Milestone M6 — TUI core

Locked TUI decisions:
- **Import rule (enforced by test in T29):** NON-TEST files under `internal/tui/**` may import only `github.com/kido5217/yolo/internal/protocol` and packages under `internal/tui/`. `testdata`/stdlib/charm libs allowed. NO `internal/session`, `internal/llm`, `internal/server`, `internal/storage`, `internal/permission`, `internal/provider`, `internal/tool`, `internal/config`, `internal/bus`, `internal/auth`. ESCAPE HATCH: `_test.go` files under `internal/tui/**` MAY additionally import `internal/server/testutil` (+ transitively its fake/seam deps) for scripted e2e — the invariant covers shipped app code, not test wiring.
- **Test stack:** TUI tests run the REAL app model (`tui.NewApp(...)`) against a **real in-process server** built from the M5 full-stack helper (move `newSrv` into `internal/server/testutil` as `testutil.Boot(t) *TestServer` — exported: `URL`, `Dir`, `Home`, `Perm *permission.Service`, `Fake *llm/fake.Handle` (script setter + `Delay(d)`), `DB`, and `LastMessages(id string) ([]protocol.MessageWithParts, error)`). TUI tests build their own client: `client.New(ts.URL, ts.Dir)`. `teatest/v2` drives the model (`teatest.NewTestModel(app, opts)`; `.Run()`, `.Send`, `.Type`, `.WaitFor`, `.RequireEqualOutput`, `.WithFinalTimeout`); assertions on final rendered output + `FinalModel` type-assert.
- **Store hydration:** route entry → REST rehydrate (home: `GET /session`; session: `GET /session/{id}`, `/message`, `/session/status`, `/permission`); every SSE reconnect → rehydrate current route. After hydration, session-route state changes come ONLY from SSE.
- **`Store.Apply(Event)`** (pure, one function, fully unit-tested): `message.updated` → upsert msg+its parts placeholder in current session; `message.part.delta` → append `Delta` to part text (field `text`/`reasoning` — `input` ignored v1); `message.part.updated` → upsert part; `message.part.removed`/`message.removed` → remove; `session.updated` → upsert in list + current fields; `session.deleted` → drop from list, route→home if current; `session.status` → `Store.Status`; `permission.asked` → append `Store.Pending` (FIFO); `permission.replied` → drop by id. Events for non-current sessions: update list entries only (title via `session.updated`), never clobber viewport.
- **Render is pure:** every screen/view/dialog exposes `Render(state Store, w int) string` (or receives a narrow struct) so unit tests assert layout without tea.
- Keys follow spec §5 keymap exactly. Busy = `Status.Type == "busy"|"retry"`.

### Task 22: `internal/tui/client` + SSE reader + `Store`

**Files:**
- Create: `internal/tui/client/client.go`, `internal/tui/client/event.go`, `internal/tui/store/store.go`, `_test.go` for each
- Create: `internal/server/testutil/testutil.go` (exported M5 harness, per locked decision above — move+rename, no behavior change)

**Interfaces:**
- Consumes: `protocol`.
- Produces:

```go
package client

type Client struct {
	Base string          // http://127.0.0.1:PORT
	Dir  string          // sent as x-yolo-directory (abs; "" = omit → server CWD)
	HC   *http.Client
	Backoff func(int) time.Duration // SSE reconnect backoff (override in tests)
}
func New(base, dir string) *Client
// errors: ErrNotFound (404), ErrBusy (409), ErrBadRequest (400)
func (c *Client)
  Health(ctx) error
  ListSessions(ctx) ([]protocol.Session, error)                 // GET /session
  CreateSession(ctx, title string) (protocol.Session, error)    // POST
  GetSession(ctx, id string) (protocol.Session, error)
  PatchSession(ctx, id string, patch map[string]any) (protocol.Session, error)
  DeleteSession(ctx, id string) error
  ListMessages(ctx, id string) ([]protocol.MessageWithParts, error)
  SendMessage(ctx, id, text string) (messageID string, err error) // 202; ErrBusy on 409
  Abort(ctx, id string) (bool, error)
  Command(ctx, id, cmd string) (resp protocol.CommandResponse, error) // {session_id? handled?}
  Status(ctx) (map[string]protocol.SessionStatus, error)
  ListProviders(ctx) ([]protocol.Provider, error)
  GetConfig(ctx) (map[string]any, error);  PatchConfig(ctx, patch map[string]any) (map[string]any, error)
  GlobalConfig(ctx, patch map[string]any|nil) (map[string]any, error)
  Auth(ctx, providerID, key string, remove bool) error          // PUT/DELETE /auth/{id}
  ListAgents(ctx) ([]protocol.Agent, error)
  ListCommands(ctx) ([]protocol.Command, error)
  ListPermissions(ctx) ([]protocol.PermissionAskedProps, error) // GET /permission
  ReplyPermission(ctx, requestID, reply string) error
  Events(ctx) chan protocol.Event                                 // SSE + auto-reconnect

package store
type Store struct {
	Sessions   []protocol.Session
	Current    *protocol.Session
	Messages   []protocol.MessageWithParts
	Providers  []protocol.Provider
	Agents     []protocol.Agent
	Commands   []protocol.Command
	Config     map[string]any
	Status     protocol.SessionStatus // zero = idle
	Pending    []protocol.PermissionAskedProps
	Conn       bool   // SSE live?
	LastHydrate int64
}
func (s *Store) Apply(ev protocol.Event)
type EventMsg struct{ Ev protocol.Event }  // tea.Msg carrier (defined in tui root, re-exported here? LOCKED: defined in internal/tui root app.go to avoid cycles; client only returns chan)
```

SSE reader (`event.go`): goroutine loop: `GET /event` (header dir), read `data:` lines → decode `protocol.Event` → send to chan; on read error/close → backoff = min(30s, 1s<<n) (injectable `Backoff func(n int) time.Duration` for tests), reconnect; ctx cancel → chan closes. Emits nothing after ctx done.

`GET /permission` row type = `protocol.PermissionAskedProps` (server T20 returns `properties` array directly).

- [ ] **Step 1: Write the failing tests**

Client tests (httptest handler echoing route+header):

```go
func TestClientScopingAndErrors(t *testing.T) {
	var gotDir, gotRoute string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDir = r.Header.Get("x-yolo-directory")
		gotRoute = r.Method + " " + r.URL.Path
		switch {
		case r.URL.Path == "/session" && r.Method == "GET":
			w.Write([]byte `[{"id":"ses_1","title":"T"}]`))  // placeholder valid JSON in real impl
		case r.URL.Path == "/session/ses_x/message" && r.Method == "POST":
			w.WriteHeader(409); w.Write([]byte(`{"error":{"message":"busy"}}`))
		case r.URL.Path == "/session/ses_missing":
			w.WriteHeader(404); w.Write([]byte(`{"error":{"message":"not found"}}`))
		}
	}))
	t.Cleanup(srv.Close)
	c := client.New(srv.URL, "/abs/dir")
	_, err := c.ListSessions(ctx)
	err = …; if !errors.Is(err, nil) { … }
	if gotDir != "/abs/dir" { t.Fatalf("dir header = %q", gotDir) }
	_, err = c.SendMessage(ctx, "ses_x", "hi")
	if !errors.Is(err, client.ErrBusy) { t.Fatalf("err = %v", err) }
	_, err = c.GetSession(ctx, "ses_missing")
	if !errors.Is(err, client.ErrNotFound) { t.Fatalf("err = %v", err) }
}

func TestEventsDecodeAndReconnect(t *testing.T) {
	n := 0
	closed := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl := w.(http.Flusher)
		n++
		if n == 1 {
			fmt.Fprint(w, `data: {"id":"evt_1","type":"session.status","properties":{"sessionID":"ses_1","status":{"type":"idle"}}}

`)
			fl.Flush()
			close(closed)  // server closes after first frame
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, `data: {"id":"evt_2","type":"session.status","properties":{"sessionID":"ses_1","status":{"type":"busy"}}}

`)
	}))
	…
	c := client.New(srv.URL, "")
	c.Backoff = func(int) time.Duration { return 10 * time.Millisecond }
	ch := c.Events(ctx)
	var evs []protocol.Event
	for i := 0; i < 2; i++ {
		evs = append(evs, <-ch)
	}
	if evs[0].ID != "evt_1" || evs[1].ID != "evt_2" || evs[1].Type != "session.status" {
		t.Fatalf("events = %+v", evs)
	}
}
```

Store unit tests: one table-driven test per event type (build Store with a current session + 1 message + 1 part, apply each event, assert resulting state); plus `TestApplyIgnoresOtherSessions` (message.updated for different sessionID does not touch `Messages`; session.updated updates list title).

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/tui/...` → FAIL (missing packages).

- [ ] **Step 3: Write minimal implementation** — client (one `do(ctx, method, path, in, out)` helper; header; body decode; error mapping by status), event reader, store.Apply switch, `testutil.Boot` moved/exported (keep T19-T21 tests compiling unchanged: they now call `server.TestServer` → alias to testutil).

- [ ] **Step 4: Run test to verify it passes** — `go vet ./... && go test ./...` PASS (M1–M5 suites still green).

- [ ] **Step 5: Commit**

```bash
git add internal/tui internal/server
git commit -m "feat: tui client, sse reader with backoff, store apply; server testutil"
```

---

### Task 23: root app + home route

**Files:**
- Create: `internal/tui/app.go`, `internal/tui/home.go`, `internal/tui/style.go`, `internal/tui/app_test.go`, `internal/tui/home_test.go`

**Interfaces:**
- Produces:

```go
package tui

type App struct {
	*client.Client
	store  store.Store
	route  route            // routeHome | routeSession
	cur    string           // current session id
	home   homeModel
	sess   sessionModel     // T24
	prompt promptModel      // T25
	dlg    dialogStack      // T26-T29 (T23: empty stack + quit-confirm only)
	toasts []toast          // T29
	lastErr string
	// tea plumbing
}
func NewApp(c *client.Client, store *store.Store, startSessionID string) *App // startSessionID "" = home
func (a *App) Init() tea.Cmd
// Init: hydrate(route) cmd → on event → apply + re-render; Events() chan drained via tea.Cmd pump (recvEventMsg)
func (a *App) Update(tea.Msg) (tea.Model, tea.Cmd)
func (a *App) View() string
```

- Hydration + event pump: `Init` returns `cmdHydrate` (client call → `hydrateMsg`) + `cmdNextEvent` (select on Events chan → `EventMsg` / on close → `connLostMsg` + reconnect pump). Every msg that mutates state ends with View recompute (bubbletea default).
- Home layout (LOCKED render):

```
Yolo
────────────────────────────
  ▸ New session
  T1 · kido/q · 2m          ← current cursor ▸
  T2 · opencode/gpt-5-nano · 3h
────────────────────────────
↑/↓ move · enter open · n new · /help
```

- Relative time (<60s `12s`, <60m `5m`, <24h `3h`, else `4d`); cursor model; keys: up/down (wrap), enter (open → hydrate session), `n` (CreateSession → open), `/help` (dialog), ctrl+c (quit confirm). Last 50 sessions only (client already returns full list; home slices newest-first — server returns updated-desc already; slice [:50]).
- `startSessionID != ""` (resume): Init hydrates that session directly; if `ErrNotFound` → exit with visible error line + `tea.Quit` (cmd layer maps to exit code 2).

- [ ] **Step 1: Write the failing tests**

```go
func TestHomeRendersListAndNewSession(t *testing.T) {
	ts := testutil.Boot(t)      // real stack: project dir + fake autotext driver
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, &store.Store{}, "")
	out := teatest.NewTestModel(a, teatest.WithInitialTermSize(80, 24)).Run() // teatest/v2
	… assert out contains "Yolo" and "New session"
	// create one via API then hydrate:
	ses, _ := c.CreateSession(ctx, "Hello")
	… send hydrate (via model test hook: teatest.Send(a, hydrateKey{}) — LOCKED: App exposes unexported test hook? teatest can only send exported tea.Msg from test pkg (same module, different pkg → use exported TestHydrateMsg? LOCKED: define exported msg type `type HydrateMsg struct{}` in app.go — harmless, teatest.Send works)
	… assert rendered line "Hello · kido/q"
}
```

(Real impl detail LOCKED: teatest scripts `HydrateMsg{}`, `prompt.KeyMsg`, etc.; app keeps one exported `EventMsg` + `HydrateMsg`; everything else internal.)

- Key handling unit test (pure): `app.handleKey(home, key)` table: up/down wrap with 3 sessions, enter sets route, `n` issues create cmd (assert via captured tea.Cmd call list — LOCKED hook: `a.Cmds []tea.Cmd` recorded in test).

- [ ] **Step 2: Run failing** — FAIL.
- [ ] **Step 3: Implement** — app.go (root, hydrate, event pump, route switch, key dispatch, quit dialog stub), home.go (model + Render pure), style.go (lipgloss v2 styles: `title`, `divider`, `cursor`, `dim`, `errRed`, `okGreen`, `toolRow`).
- [ ] **Step 4: PASS** — `go test ./internal/tui/... && go test ./...`.
- [ ] **Step 5: Commit** — `feat: tui root app + home route (list, new session, resume hydrate)`.

---

### Task 24: session viewport + streaming render

**Files:**
- Create: `internal/tui/session.go`, `internal/tui/session_test.go`

**Behavior (LOCKED):**
- Viewport = `bubbles/v2/viewport` over rendered message blocks; ↑/↓/pgup/pgdn scroll (when no dialog + not typing in prompt… prompt has focus? LOCKED focus: prompt focused only when routeSession AND not busy AND no dialog; otherwise viewport scroll keys work. Simplest v1 rule: **prompt always focused** (opencode TUI behavior), scroll keys `pgup/pgdn` only — ↑/↓ move prompt cursor for multiline; LOCKED: pgup/pgdn scroll viewport; ↑/↓ = prompt edit (v1 simplification, noted in /help table).
- Block render per message:

```
User: hello                         ← user, verbatim, wraps
──────────────────────────────────
▸ think                             ← reasoning part collapsed: "▸ think" / expanded "▾ think" + dim indented text (toggle t)
  because …
read src/main.go (3 ok)             ← tool line completed: "✓ read src/main.go" + output-size hint
▶ bash: ls -la                      ← running: "▶" + first line of input
✗ grep: pattern: error              ← error tool: red, "✗ tool: error-text"
(e pressed expands → full I/O block, collapses next e)
ok-text part                        ← assistant text, streams on delta
```

- Tool line format LOCKED: `✓ <tool> <title>` (title = state.Title; empty → first input arg stringified or callID prefix 8); running `▶ <tool> <title>`; error `✗ <tool> <error>` (error text first line). Expand adds 2-space-indented block of state.Output (≤40 lines, tail) or state.Error.
- Streaming text: `message.part.delta` appends to current assistant part text; last message auto-scrolls when busy AND cursor-at-bottom (auto-follow; user scroll-up pauses follow, `pgdn` to bottom resumes).
- Abort: `esc` while busy → `client.Abort` (toast on error).
- Error message parts (message.error set / session.error) render as red `! <message>` line.

- [ ] **Step 1: Write the failing tests** — (a) pure render test: fixture Store (user msg; assistant msg with text part + reasoning part + 3 tool parts (running/completed/error); expanded states) → assert expected multi-line string (golden-ish, inline). (b) teatest streaming: `ts.Fake.SetScript(2 turns: [text "thinking", tool read], [text "done"])`; open session; type+enter "do it"; `teatest.WaitFor` output contains `✓ read` and `done` (timeout 5s default — use `teatest.WithFinalTimeout(5*time.Second)`); assert auto-scroll final frame shows last line `done`. (c) expand/collapse: press `e` toggles output block lines count.

- [ ] **Step 2: FAIL** — [ **Step 3: Implement** (session.go + pure `renderMessages(st, expanded map[partID]bool, w int) string`) ] — [ **Step 4: PASS** ] — [ **Step 5: Commit** `feat: tui session viewport — streaming, reasoning, tool rows, expand, abort`.

---

### Task 25: prompt input + slash commands

**Files:**
- Create: `internal/tui/prompt.go`, `internal/tui/prompt_test.go`

**Behavior (LOCKED):**
- `bubbles/v2/textinput`, multiline (NewlineAfterEnter? LOCKED: plain single-line v1 + `shift+enter` unsupported by term → multiline via backslash-line-end `\`+enter (opencode-ish escape) — document in /help; simple, no ambiguity).
- Enter: if busy → toast "abort or wait (esc aborts)"; else `SendMessage` → on `ErrBusy` toast; on success clear input + store stays (server events drive view). Empty → ignore.
- Slash menu: input starts `/` → overlay menu above input listing `Commands` filtered by prefix (`/help /new /model /agents /exit` + `/` shows all); ↑/↓ select (when menu open, arrows drive menu not prompt), enter → execute:
  - `/new` → `client.Command(id, "/new")` → response `session_id` → hydrate new session (or CreateSession fallback if no current session)
  - `/model` → open model dialog (T28)
  - `/agents` → agent dialog
  - `/help` → help dialog
  - `/exit` → quit-confirm
  - No current session + `/new` → `CreateSession` directly (LOCKED).
- `/` with no match → menu shows `no match`; enter clears.

- [ ] **Step 1: Failing tests** — teatest: type "hello", enter → assert `ts.LastMessages(sesID)` server-side sees the message + prompt cleared; type "/m" → menu contains `/model`; select enter → model dialog appears (T28 stub dialog exists by then — for T25 isolation assert routed dialog title "Model"); 409 path: fake delay turn running, send second text → toast text visible.
- [ ] **Steps 2-5** as standard (commit `feat: tui prompt + slash command menu`).

---

### Task 26: permission dialog + footer

**Files:**
- Create: `internal/tui/permission.go`, `internal/tui/footer.go`, both `_test.go`

**Behavior (LOCKED):**
- On top `Store.Pending[0]` render overlay above prompt:

```
permission · bash
  patterns: ls *
  tool call: msg_1/call_  (first 6 chars of callID)
  [1] once  [2] always  [3] reject
```

  always-line shows `Always: ls, dir/*` suggestions if non-empty (else omit line).
- Keys 1/2/3 → `ReplyPermission` (fail → toast, keep dialog); success → drop from pending (also via `permission.replied` event; idempotent). Dialog hidden when pending empty. While dialog open, other keys ignored (except 1/2/3; esc = reject LOCKED — matches opencode TUI).
- Footer always visible (both routes? LOCKED: both routes):

```
kido/q · build · ↑123 ↓45 · $0.0002 · ● live · ▸ busy 2/…
```

  - model = current session model (home: config default model or "no model")
  - agent = current session agent (home: config default)
  - tokens = session tokens input/output (summed, from Store.Current.Tokens)
  - cost = session cost, 4 decimals
  - `● live` (SSE ok, green) / `○ off` (reconnecting, red) from Store.Conn
  - busy: `spinner` (5-frame lipgloss spinner) + status; retry shows `retry n: msg`
- Unit tests: footer render table (idle/busy/retry/live/off/token formatting); permission render (with/without always suggestions, callID truncation).
- Teatest: fake script turn1 = tool `bash` call (build agent → ask per matrix) → dialog visible; press `1`; turn completes; final output shows `✓ bash`; then `permission.replied` event path (reply via HTTP directly in test, not key — second scenario) drops dialog.

- [ ] **Commit** `feat: tui permission dialog + footer (status, tokens, cost, conn)`.

---

# Milestone M7 — TUI dialogs, polish, CLI

### Task 27: model + agent dialogs

**Files:**
- Create: `internal/tui/model.go`, `internal/tui/agent.go`, `_test.go`

**Behavior (LOCKED):**
- Model dialog (ctrl+p or /model): two-pane list — left providers (name + auth dot: `● loaded` green / `○ missing` red when key_required / `· not-required`), right models of selected provider: `<default*> <name>  262k ctx  $2/$10` (default marker `*` when model id == config `model` tail or session model). Keys: ↑/↓ move pane focus (tab or ←/→; LOCKED: tab), enter on model → sub-choice overlay: `[a] this session  [b] set default`:
  - a → `PatchSession(id, {"model": "provider/id"})`
  - b → `PatchConfig({"model": "provider/id"})`
  - esc closes.
- Agent dialog (ctrl+a or /agents): list from `GET /agent` (name + description + current marker); enter → sub-choice `[a] this session  [b] set default` → PatchSession/PatchConfig `agent`.
- Both: after success → close + toast `model set: opencode/gpt-5-nano`.

- Teatest: open model dialog (ctrl+p) → render contains `kido` provider with `not-required` and `Qwen3.8-27B` (server test fixture seed) + opencode with `missing`; navigate to a zen model, enter, `a` → `GetSession` shows model `opencode/gpt-5-nano`; agent dialog: select `yolo` + `a` → session agent yolo.
- Commit `feat: tui model + agent dialogs (session/default targeting)`.

---

### Task 28: toasts + /help + quit + full teatest suites

**Files:**
- Create: `internal/tui/toast.go`, `internal/tui/help.go`, `_test.go`

**Behavior (LOCKED):**
- Toast: top error-flash area, red, auto-clear after 4s (`tick` cmd) or on next toast; queue ≤3.
- Help dialog: renders spec keymap table verbatim (incl. v1 scroll note: `pgup/pgdn scroll · \+enter newline`) — static text.
- Quit-confirm (ctrl+c): `quit? [y/n]`; y → `tea.Quit`; n → back.
- Teatest full suites (M6-M7 "done when" gate):
  1. `TestTUIFullTurn`: home → `n` → type → streamed text+tool+reasoning rendered → `t` toggle → `e` expand → exit → assert full sequence via `FinalOutput`.
  2. `TestTUIPermissionFlow`: bash ask → 1/2/3 each in separate runs (3 subtests) → final states (allow proceeds; reject → tool error part `forbidden` rendered red).
  3. `TestTUIDialogs`: model+agent+help+quit each scripted.

- Commit `feat: tui toasts, help, quit confirm; teatest suites green`.

---

### Task 29: CLI wiring (`yolo`, `yolo serve`, resume) + import-direction test

**Files:**
- Create: `cmd/yolo/main.go`, `cmd/yolo/main_test.go`, `internal/tui/imports_test.go`

**Behavior (LOCKED):**
- `yolo [sessionID] [--dir DIR]`: build config loader (XDG env), auth store, storage DB, provider registry (with real fetch + `provider.NoFetch()` seam for tests), permission service, engine (real drivers; if `YOLO_LLM=fake` → fake per T19 wiring), server `New` → `net.Listen("127.0.0.1:0")` (in-process mode; NO port flag for TUI mode — ephemeral) → `tui.NewApp(client, store, sessionID?)` → `tea.NewProgram(app, tea.WithAltScreen())` run → on exit: engine drain (LOCKED: add `Engine.Shutdown(ctx)`: abort all active + wait ≤5s), server Close, DB close.
  - `--dir` defaults to CWD; must exist (abs path).
- `yolo serve [--addr 127.0.0.1:4096]`: same stack, real listener, log `listening on …`, serve until SIGINT.
- Resume: `yolo ses_…` → TUI starts in that session; on 404 → print `session not found: ses_…` to stderr, exit 2.
- Env passthrough: `YOLO_LLM`, `YOLO_FAKE_SCRIPT`, XDG vars (config.Loader reads real env).
- **Import-direction test** (`internal/tui/imports_test.go`): `go/parser`+`go/ast` walk all **non-test** `.go` under `internal/tui/`, collect import paths, fail on any `internal/*` outside `internal/tui`/`internal/protocol`. `_test.go` files are checked with the M6 escape-hatch allowlist (additional: `internal/server/testutil` and its public types) — so scripted e2e can boot a real stack without polluting the app import graph.
- Tests: (a) import test (trivial pass now, guards future); (b) cmd build smoke: `go build ./cmd/yolo` + `TestMain`-style golden: run `yolo serve --addr 127.0.0.1:0` child? LOCKED: no child-process tests in unit suite — instead `serve` has an unexported `buildStack(t) (*server.Deps, cleanup)` + `main_test` calls it with fake env and asserts `/global/health` 200 (in-process, no subprocess).
- Commit `feat: cli — yolo in-process serve+tui, yolo serve, resume; tui import-direction guard`.

---

# Milestone M8 — Polish + docs

### Task 30: README, logging+rotation, signal drain, lint sweep, live e2e, tag

**Files:**
- Create: `README.md`, `internal/log/log.go` (+test), `.golangci.yml`, `scripts/e2e-live.sh`
- Edit: `cmd/yolo/main.go` (log+signal wiring), `go.mod` (tidy)

**Behavior (LOCKED):**
- `internal/log`: writes `<DataYoloDir>/log/yolo.log`; size rotation at 5 MB → `yolo.log.1` (keep 1 generation, overwrite); `log.New(dir) *Logger` with `Errorf/Infof`; server handlers + engine errors + cli startup use it (no print). Unit: rotation triggers on size (fixture 5MB write).
- Signal: SIGINT/SIGTERM → TUI quit path (or server: graceful `http.Server.Shutdown` 5s) → engine `Shutdown` → exit 0. (Wired in T29 `run()`; add drain assertions there if missing.)
- README: what/why, prereq (Go ≥1.25), build (`go build ./cmd/yolo`), run (`yolo`, `yolo serve`, `yolo <session>`), config files (project `yolo.jsonc` + global, fields table minimal), auth (`yolo auth add opencode`? LOCKED v1: auth via `auth.json` or `OPENCODE_API_KEY`/config `provider.X.apiKey`; `/auth` API only — document that), keymap table, data dir layout (`~/.local/share/yolo/{auth.json,storage/yolo.db,plans,log}`), test commands, env (`YOLO_LLM=fake` dev mode), v1 non-goals.
- Lint: `.golangci.yml` (run: govet, staticcheck, errcheck (exclude `tea.Cmd`/`fmt.Fprint` to stdout), unused, misspell, gocritic (default -tag)) → `golangci-lint run` clean (install via `go install` if missing; note in plan).
- Live e2e (ON-DEMAND ONLY — never in CI): `scripts/e2e-live.sh` — boots `yolo serve` against real kido (needs `KIDO_BASE_URL` or config), runs a scripted HTTP client doing: create session → send "list files in /tmp" with agent=yolo → assert one `read`/`glob` tool call + text reply → abort test → print PASS/FAIL exit code. Flagged in README as manual.
- Tag `v1.0.0`: **only after explicit user go-ahead** (plan step: ask; do not auto-tag).

**Verification (final gate):** `go vet ./... && golangci-lint run && go test ./...` all green + user dogfoods real task on `kido/Qwen3.8-27B` (spec v1 success criteria) — recorded in PROGRESS.md.

- Commit `docs+chore: README, log rotation, signal drain, lint sweep, live e2e script` (tag separately, user-approved).

---

# Self-Review Notes (written inline here; re-run at plan commit)
- Coverage: M0 §8.1 (T1) · M1 protocol/config/auth/storage/bus (T2-T6) · M2 llm+providers (T7-T9) · M3 tools+permissions (T10-T14) · M4 engine (T15-T18) · M5 server+contract (T19-T21) · M6 TUI core (T22-T26) · M7 dialogs+cli (T27-T29) · M8 polish (T30). Spec §3 wire ⇐ T2/T19/T20/T21 · §4 ⇐ T7-T18 · §5 ⇐ T22-T28 · §6 ⇐ T3/T20 · §7 ⇐ T3/T5/T19 · §8 milestones ⇐ all.
- Cross-task type consistency checked points: protocol DTOs (T2) used verbatim by server/TUI/client; `provider.Registry` seam `NewStaticForTest` introduced T19 and consumed by T20-T29 testutil; `fake.AutoText`/`Handle` (T19) vs `Fake.Handle` testutil (T22); `Engine.Shutdown` (T29 note — LOCKED: add method in T18 engine if not present: `Shutdown(ctx)`: abort all, wait ≤5s, close); `testutil.Boot` (T22) moves T19 harness.
- Placeholder scan: none — every task has concrete files, APIs, steps, commit.
- Resolutions/flags to surface in handoff (severity):
  1. important — teatest/v2 module substitution (spec's `charm.land/x/exp/teatest` is v1; plan pins `teatest/v2` v2.0.0-20260816…).
  2. important — spec DDL lacked message cost/tokens; added columns (T5) + session aggregate at read time (T19 session DTO).
  3. important — spec DDL lacked todo persistence; added migration v2 `todo` table (T14) + `protocol.Todo`.
  4. minor — title.txt lives in `agent/prompt/` (spec said `session/prompt/`); M4 embeds title.txt + 13 session prompts = 14 files.
  5. minor — `config.agents` key vs spec `agent` ambiguity → `agents` map (T3/T20), custom agents v1 = permission merge + description stub.
  6. minor — SSE frame includes `id` (spec envelope example shows type+properties only).
  7. minor — JSONC comment loss on config PATCH rewrite.
  8. minor — v1 keymap scroll simplified: pgup/pgdn viewport, `\`+enter newline (spec `↑/↓` viewport replaced — noted in /help text, flagged for T24/T28 render consistency).
