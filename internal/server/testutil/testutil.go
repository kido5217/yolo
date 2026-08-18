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
}

// Boot boots the full stack with the auto-text fake driver and registers
// cleanup on t.
func Boot(t *testing.T) *TestServer {
	t.Helper()
	return boot(t, fakellm.New(fakellm.AutoText()))
}

// BootWithDriver boots the full stack with a caller-provided fake driver
// (the env-gate variant, YOLO_LLM=fake).
func BootWithDriver(t *testing.T, drv *fakellm.Driver) *TestServer {
	t.Helper()
	return boot(t, drv)
}

// boot boots the FULL stack on the given kido driver (no network).
func boot(t *testing.T, drv *fakellm.Driver) *TestServer {
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
	permSvc := permission.New(db, b)
	eng := session.New(session.Deps{
		DB:      db,
		Bus:     b,
		Prov:    prov,
		Perm:    permSvc,
		Tools:   tool.Registry(),
		DataDir: dataDir,
		Cfg:     func(string) (*protocol.Config, error) { return &protocol.Config{}, nil },
		Drivers: map[string]llm.Driver{"kido": drv},
		Clock:   func() int64 { return time.Now().UnixMilli() },
	})
	dir := t.TempDir()
	home := filepath.Join(root, "home")
	h := server.New(server.Deps{
		DB:      db,
		Bus:     b,
		Engine:  eng,
		Prov:    prov,
		Perm:    permSvc,
		Config:  config.Loader{Env: map[string]string{}},
		WorkDir: dir,
		Dirs:    config.Dirs{Home: home, Data: dataDir, Cache: filepath.Join(root, "cache")},
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return &TestServer{Server: ts, DB: db, Bus: b, Eng: eng, Fake: drv, PermSvc: permSvc, Dir: dir, Home: home}
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

// FakeDelay makes subsequent fake turns hold open for d (slow-turn tests).
func (ts *TestServer) FakeDelay(d time.Duration) { ts.Fake.SetDelay(d) }

// ParkAsk parks a pending permission ask in a goroutine and blocks until it
// is visible on GET /permission (so pinned tests never race the park).
func (ts *TestServer) ParkAsk(sessionID, action, resource string) {
	req := permission.Request{
		RequestID:  protocol.NewID("perm"),
		SessionID:  sessionID,
		Agent:      "build",
		Permission: action,
		Resources:  []string{resource},
	}
	go ts.PermSvc.Ask(context.Background(), req)
	row, err := ts.DB.GetSession(sessionID)
	if err != nil {
		panic(err)
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

// Frame decodes the next `data:` frame.
func (r *SSEReader) Frame(t *testing.T) SSEFrame {
	t.Helper()
	for r.sc.Scan() {
		line := r.sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var f SSEFrame
		if err := json.Unmarshal([]byte(line[len("data: "):]), &f); err != nil {
			t.Fatalf("bad frame %q: %v", line, err)
		}
		return f
	}
	t.Fatalf("sse stream closed: %v", r.sc.Err())
	return SSEFrame{}
}
