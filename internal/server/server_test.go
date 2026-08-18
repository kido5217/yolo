package server_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

type srv struct {
	*httptest.Server
	db      *storage.DB
	eng     *session.Engine
	fake    *fakellm.Driver
	permSvc *permission.Service
	dir     string
	home    string
}

// fakeDelay makes subsequent fake turns hold open for d (slow-turn tests).
func (s *srv) fakeDelay(d time.Duration) { s.fake.SetDelay(d) }

// newSrv boots the FULL stack on a fake provider set (no network): kido static model, no fetch
func newSrv(t *testing.T) *srv {
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
	fake := fakellm.New(fakellm.AutoText())
	prov := provider.NewStaticForTest()
	permSvc := permission.New(db, b)
	eng := session.New(session.Deps{
		DB: db, Bus: b,
		Prov:  prov,
		Perm:  permSvc,
		Tools: tool.Registry(),
		DataDir: dataDir,
		Cfg:   func(string) (*protocol.Config, error) { return &protocol.Config{}, nil },
		Drivers: map[string]llm.Driver{"kido": fake},
		Clock:   func() int64 { return time.Now().UnixMilli() },
	})
	dir := t.TempDir()
	home := filepath.Join(root, "home")
	h := server.New(server.Deps{
		DB: db, Bus: b, Engine: eng, Prov: prov, Perm: permSvc,
		Config:  config.Loader{Env: map[string]string{}},
		WorkDir: dir,
		Dirs:    config.Dirs{Home: home, Data: dataDir, Cache: filepath.Join(root, "cache")},
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return &srv{Server: ts, db: db, eng: eng, fake: fake, permSvc: permSvc, dir: dir, home: home}
}

// parkAsk parks a pending permission ask in a goroutine and blocks until it
// is visible on GET /permission (so pinned tests never race the park).
func (s *srv) parkAsk(sessionID, action, resource string) {
	req := permission.Request{
		RequestID:  protocol.NewID("perm"),
		SessionID:  sessionID,
		Agent:      "build",
		Permission: action,
		Resources:  []string{resource},
	}
	go s.permSvc.Ask(context.Background(), req)
	row, err := s.db.GetSession(sessionID)
	if err != nil {
		panic(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := http.NewRequest(http.MethodGet, s.URL+"/permission", nil)
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

// writeCfg writes a project yolo.jsonc.
func writeCfg(t *testing.T, dir, jsonc string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "yolo.jsonc"), []byte(jsonc), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

// sseFrame is one decoded `data:` frame.
type sseFrame struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
}

// String returns a property as a string; object properties resolve to their
// "type" field (session.status: status {type:"idle"|...}).
func (f sseFrame) String(key string) string {
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

// sseReader keeps ONE scanner over the open SSE body (a fresh scanner per
// read would drop buffered bytes).
type sseReader struct{ sc *bufio.Scanner }

// sseConnect opens the /event stream and asserts the 200 handshake.
func sseConnect(t *testing.T, s *srv, dir string) *sseReader {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, s.URL+"/event", nil)
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
	return &sseReader{sc: bufio.NewScanner(resp.Body)}
}

// Frame decodes the next `data:` frame.
func (r *sseReader) Frame(t *testing.T) sseFrame {
	t.Helper()
	for r.sc.Scan() {
		line := r.sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var f sseFrame
		if err := json.Unmarshal([]byte(line[len("data: "):]), &f); err != nil {
			t.Fatalf("bad frame %q: %v", line, err)
		}
		return f
	}
	t.Fatalf("sse stream closed: %v", r.sc.Err())
	return sseFrame{}
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

func TestScopedHeaderURLDecoded(t *testing.T) {
	s := newSrv(t)
	odd := filepath.Join(t.TempDir(), "odd dir")
	if err := os.MkdirAll(odd, 0o755); err != nil {
		t.Fatal(err)
	}
	// header value is URL-encoded; the server must PathUnescape it
	resp, b := req(t, s, "GET", "/path", url.PathEscape(odd), "")
	var p map[string]string
	_ = json.Unmarshal(b, &p)
	if resp.StatusCode != 200 || p["directory"] != odd {
		t.Fatalf("path: %d %s", resp.StatusCode, b)
	}
	// invalid escape → 400
	resp, _ = req(t, s, "GET", "/path", "/bad/%zz", "")
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

func TestMessagesEndpoint(t *testing.T) {
	s := newSrv(t)
	d := t.TempDir()
	_, b := req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	json.Unmarshal(b, &ses)
	resp, _ := req(t, s, "POST", "/session/"+ses.ID+"/message", d, `{"text":"hello"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send: %d", resp.StatusCode)
	}
	// wait for the turn to settle
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, b = req(t, s, "GET", "/session/status", d, "")
		var st struct {
			Sessions map[string]string `json:"sessions"`
		}
		json.Unmarshal(b, &st)
		if st.Sessions[ses.ID] == "idle" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	resp, b = req(t, s, "GET", "/session/"+ses.ID+"/message", d, "")
	if resp.StatusCode != 200 {
		t.Fatalf("messages: %d %s", resp.StatusCode, b)
	}
	var msgs []protocol.MessageWithParts
	if err := json.Unmarshal(b, &msgs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var user, asst *protocol.MessageWithParts
	for i := range msgs {
		switch msgs[i].Info.Role {
		case "user":
			user = &msgs[i]
		case "assistant":
			asst = &msgs[i]
		}
	}
	if user == nil || asst == nil {
		t.Fatalf("want user+assistant messages, got %d: %s", len(msgs), b)
	}
	var seenHello bool
	for _, p := range user.Parts {
		if p.Type == "text" && p.Text == "hello" {
			seenHello = true
		}
	}
	if !seenHello {
		t.Fatalf("user parts = %+v", user.Parts)
	}
	var seenOK bool
	for _, p := range asst.Parts {
		if p.Type == "text" && strings.HasPrefix(p.Text, "ok-") {
			seenOK = true
		}
	}
	if !seenOK {
		t.Fatalf("assistant parts = %+v", asst.Parts)
	}
}

func TestSendMessage409AndEvents(t *testing.T) {
	s := newSrv(t)
	d := t.TempDir()
	_, b := req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	json.Unmarshal(b, &ses)
	id := ses.ID

	// subscribe SSE BEFORE sending (no pre-read: nothing is published yet)
	res := sseConnect(t, s, d)
	resp, b := req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"hello"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send: %d %s", resp.StatusCode, b)
	}
	// collect events until session.status idle
	seen := map[string]int{}
	var types []string
	for i := 0; i < 50; i++ {
		ev := res.Frame(t)
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
	json.Unmarshal(b2, &body)
	if resp.StatusCode != 200 || !body.Aborted {
		t.Fatalf("abort: %d %s", resp.StatusCode, b2)
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

func TestFakeFromEnv(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		drv, err := server.FakeFromEnv(map[string]string{})
		if err != nil || drv != nil {
			t.Fatalf("drv=%v err=%v", drv, err)
		}
	})
	t.Run("fake without script", func(t *testing.T) {
		if _, err := server.FakeFromEnv(map[string]string{"YOLO_LLM": "fake"}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("bad value", func(t *testing.T) {
		if _, err := server.FakeFromEnv(map[string]string{"YOLO_LLM": "openai"}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("script", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "script.json")
		script := `[{"parts":[{"kind":"text","text":"hi","finish":"stop","usage":{"input":1,"output":1}}],"delay_ms":0}]`
		if err := os.WriteFile(p, []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
		drv, err := server.FakeFromEnv(map[string]string{"YOLO_LLM": "fake", "YOLO_FAKE_SCRIPT": p})
		if err != nil {
			t.Fatal(err)
		}
		req := llm.Request{Model: "q", Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}}
		st, err := drv.Stream(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		p1, err := st.Next(context.Background())
		if err != nil || p1.Text != "hi" || p1.Finish != "stop" {
			t.Fatalf("part=%+v err=%v", p1, err)
		}
	})
}
