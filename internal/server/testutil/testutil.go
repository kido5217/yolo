// Package testutil exports the M5 server harness: it boots the full core
// server stack on a scripted fake provider (no network) for black-box tests,
// including TUI tests that drive the wire contract end to end.
package testutil

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/llm"
	fakellm "github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

// TestServer is the full-stack harness: core server plus engine on a fake
// provider, temp data/home dirs.
type TestServer struct {
	*httptest.Server
	DB      *storage.DB
	Bus     *bus.Bus
	Eng     *session.Engine
	Fake    *fakellm.Driver
	PermSvc *permission.Service
	Dir     string
	Home    string
	// LogDir, when set (BootWithDriverLog), is the server's log directory
	// (yolo.log at LogDir/log/yolo.log).
	LogDir string
	// ctx is the harness's test context (t.Context() at Boot); harness-side
	// storage calls use it instead of minting their own.
	ctx context.Context
}

// Boot boots the full stack with the auto-text fake driver and registers
// cleanup on t.
func Boot(t *testing.T) *TestServer {
	t.Helper()
	return bootLog(t, fakellm.New(fakellm.AutoText()), &protocol.Config{}, "")
}

// BootWithDriver boots the full stack with a caller-provided fake driver
// (the env-gate variant, YOLO_LLM=fake).
func BootWithDriver(t *testing.T, drv *fakellm.Driver) *TestServer {
	t.Helper()
	return bootLog(t, drv, &protocol.Config{}, "")
}

// BootWithDriverConfig boots the full stack with a caller-provided fake driver
// and pins the engine's config dependency to cfg (empty per directory by
// default). Tests use it to exercise config-driven behavior such as
// permission rules without a yolo.jsonc file.
func BootWithDriverConfig(t *testing.T, drv *fakellm.Driver, cfg *protocol.Config) *TestServer {
	t.Helper()
	return bootLog(t, drv, cfg, "")
}

// BootWithDriverLog boots the full stack with a caller-provided fake driver,
// writing server logs to logDir (TestServer.LogDir) so tests can read
// <logDir>/log/yolo.log.
func BootWithDriverLog(t *testing.T, drv *fakellm.Driver, logDir string) *TestServer {
	t.Helper()
	return bootLog(t, drv, &protocol.Config{}, logDir)
}

// bootLog boots the FULL stack on the given kido driver (no network); with
// logDir set, server logs go to <logDir>/log/yolo.log.
func bootLog(t *testing.T, drv *fakellm.Driver, cfg *protocol.Config, logDir string) *TestServer {
	t.Helper()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "storage"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := storage.Open(filepath.Join(dataDir, "storage", "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	b := bus.New()
	prov := provider.NewStaticForTest()
	permSvc := permission.New(db, b, nil, dataDir)
	eng, err := session.New(session.Deps{
		DB:      db,
		Bus:     b,
		Prov:    prov,
		Perm:    permSvc,
		Tools:   tool.Registry(),
		DataDir: dataDir,
		Cfg:     func(string) (*protocol.Config, error) { return cfg, nil },
		Drivers: map[string]llm.Driver{"kido": drv},
		Clock:   func() int64 { return time.Now().UnixMilli() },
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	home := filepath.Join(root, "home")
	var logger *log.Logger
	if logDir != "" {
		logger = log.New(logDir)
		t.Cleanup(logger.Close)
	}
	h := server.NewHandler(server.Deps{
		DB:      db,
		Bus:     b,
		Engine:  eng,
		Prov:    prov,
		Perm:    permSvc,
		Config:  config.Loader{Env: map[string]string{}},
		WorkDir: dir,
		Dirs:    config.Dirs{Home: home, Data: dataDir, Cache: filepath.Join(root, "cache")},
		Log:     logger,
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return &TestServer{Server: ts, DB: db, Bus: b, Eng: eng, Fake: drv, PermSvc: permSvc, Dir: dir, Home: home, LogDir: logDir, ctx: t.Context()}
}

// WaitSubscribe blocks until the bus has at least n live subscribers (an SSE
// reader registered), so subsequent publishes are not dropped by the
// subscribe/handshake window. Fails the test on a 2s deadline.
func (ts *TestServer) WaitSubscribe(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ts.Bus.SubscriberCount() >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d bus subscriber(s); have %d", n, ts.Bus.SubscriberCount())
}

// WaitIdle polls /session/status until the session reports idle. Fails on a
// 10s deadline.
func (ts *TestServer) WaitIdle(t *testing.T, dir, id string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, b := Req(t, ts, "GET", "/session/status", dir, "")
		var st struct {
			Sessions map[string]string `json:"sessions"`
		}
		_ = json.Unmarshal(b, &st)
		if st.Sessions[id] == "idle" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("session %s never went idle", id)
}

// WaitBusy polls /session/status until the session reports busy (deterministic
// busy window for 409 tests instead of a fixed sleep). Fails on a 5s deadline.
func (ts *TestServer) WaitBusy(t *testing.T, dir, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, b := Req(t, ts, "GET", "/session/status", dir, "")
		var st struct {
			Sessions map[string]string `json:"sessions"`
		}
		_ = json.Unmarshal(b, &st)
		if st.Sessions[id] == "busy" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("session %s never became busy", id)
}

// FakeDelay makes subsequent fake turns hold open for d (slow-turn tests).
func (ts *TestServer) FakeDelay(d time.Duration) { ts.Fake.SetDelay(d) }

// LastMessages fetches the persisted messages of a session (GET
// /session/{id}/message) and decodes them. Test-side read path against the
// in-process test server.
func (ts *TestServer) LastMessages(t *testing.T, sessionID string) []protocol.MessageWithParts {
	t.Helper()
	resp, body := Req(t, ts, "GET", "/session/"+sessionID+"/message", ts.Dir, "")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /session/%s/message = %d %s", sessionID, resp.StatusCode, string(body))
	}
	var out []protocol.MessageWithParts
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode LastMessages: %v", err)
	}
	return out
}

// ParkAsk parks a pending permission ask in a goroutine and blocks until it
// is visible on GET /permission (so pinned tests never race the park). The
// ask's context is cancelled at test end, so an un-replied park cannot leak
// its goroutine past the test.
func (ts *TestServer) ParkAsk(t *testing.T, sessionID, action, resource string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req := permission.Request{
		RequestID:  protocol.NewID("perm"),
		SessionID:  sessionID,
		Agent:      "build",
		Permission: action,
		Resources:  []string{resource},
	}
	go func() {
		_, _ = ts.PermSvc.Ask(ctx, req)
	}()
	row, err := ts.DB.GetSession(ts.ctx, sessionID)
	if err != nil {
		t.Fatalf("park ask: get session %s: %v", sessionID, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := http.NewRequest(http.MethodGet, ts.URL+"/permission", nil)
		r.Header.Set("x-yolo-directory", row.ProjectDir)
		if resp, err := http.DefaultClient.Do(r); err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			var pend []protocol.PermissionAskedProps
			if json.Unmarshal(b, &pend) == nil {
				for _, p := range pend {
					if p.ID == req.RequestID {
						return
					}
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("parked ask %s did not become visible on GET /permission within 2s", req.RequestID)
}

// WriteCfg writes a project yolo.jsonc.
func WriteCfg(t *testing.T, dir, jsonc string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "yolo.jsonc"), []byte(jsonc), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Req issues one request against the harness; dir "" omits the scope header
// (server work dir fallback).
func Req(t *testing.T, ts *TestServer, method, path, dir, body string) (*http.Response, []byte) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r, err := http.NewRequest(method, ts.URL+path, rd)
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

// SSEFrame is one decoded `data:` frame.
type SSEFrame struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
}

// String returns a property as a string; object properties resolve to their
// "type" field (session.status: status {type:"idle"|...}).
func (f SSEFrame) String(key string) string {
	v, ok := f.Properties[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if s, ok := t["type"].(string); ok {
			return s
		}
	}
	return ""
}

// SSEReader keeps ONE scanner over the open SSE body (a fresh scanner per
// read would drop buffered bytes).
type SSEReader struct {
	sc *bufio.Scanner
}

// SSEConnect opens the /event stream and asserts the 200 handshake.
func SSEConnect(t *testing.T, ts *TestServer, dir string) *SSEReader {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, ts.URL+"/event", nil)
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
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("sse connect: %d %s", resp.StatusCode, b)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return &SSEReader{sc: bufio.NewScanner(resp.Body)}
}

// Frame decodes the next `data:` frame. The single scan runs in a helper
// goroutine so the wait is bounded: a frame that never arrives fails the
// test on the deadline instead of hanging to the go test timeout.
func (r *SSEReader) Frame(t *testing.T) SSEFrame {
	t.Helper()
	type lineResult struct {
		line string
		ok   bool
		err  error
	}
	ch := make(chan lineResult, 1)
	deadline := time.After(5 * time.Second)
	for {
		go func() {
			if !r.sc.Scan() {
				ch <- lineResult{ok: false, err: r.sc.Err()}
				return
			}
			ch <- lineResult{line: r.sc.Text(), ok: true}
		}()
		select {
		case res := <-ch:
			if !res.ok {
				t.Fatalf("sse stream closed: %v", res.err)
			}
			if !strings.HasPrefix(res.line, "data: ") {
				continue
			}
			var f SSEFrame
			if err := json.Unmarshal([]byte(res.line[len("data: "):]), &f); err != nil {
				t.Fatalf("bad frame %q: %v", res.line, err)
			}
			return f
		case <-deadline:
			t.Fatalf("timed out after 5s waiting for an SSE data frame")
		}
	}
}
