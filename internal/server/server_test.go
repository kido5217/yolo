package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

func TestHealthAndPathAndProject(t *testing.T) {
	s := testutil.Boot(t)
	resp, b := testutil.Req(t, s, "GET", "/global/health", "", "")
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"ok"`) {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	d := t.TempDir()
	resp, b = testutil.Req(t, s, "GET", "/path", d, "")
	var p map[string]string
	_ = json.Unmarshal(b, &p)
	if resp.StatusCode != 200 || p["directory"] != d {
		t.Fatalf("path: %d %s", resp.StatusCode, b)
	}
	_, b = testutil.Req(t, s, "GET", "/project/current", d, "")
	var pr struct {
		ID, Name, Directory string
	}
	if err := json.Unmarshal(b, &pr); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if pr.Directory != d || strings.Count(pr.ID, "prj_") != 1 || !strings.HasPrefix(pr.ID, "prj_") {
		t.Fatalf("project: %s %s", pr.ID, pr.Directory)
	}
	// bad dir → 400
	resp, _ = testutil.Req(t, s, "GET", "/path", "/no/such/dir/xyz", "")
	if resp.StatusCode != 400 {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestScopedHeaderURLDecoded(t *testing.T) {
	s := testutil.Boot(t)
	odd := filepath.Join(t.TempDir(), "odd dir")
	if err := os.MkdirAll(odd, 0o755); err != nil {
		t.Fatal(err)
	}
	// header value is URL-encoded; the server must PathUnescape it
	resp, b := testutil.Req(t, s, "GET", "/path", url.PathEscape(odd), "")
	var p map[string]string
	_ = json.Unmarshal(b, &p)
	if resp.StatusCode != 200 || p["directory"] != odd {
		t.Fatalf("path: %d %s", resp.StatusCode, b)
	}
	// invalid escape → 400
	resp, _ = testutil.Req(t, s, "GET", "/path", "/bad/%zz", "")
	if resp.StatusCode != 400 {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestSessionLifecycleAndScoping(t *testing.T) {
	s := testutil.Boot(t)
	d := t.TempDir()
	other := t.TempDir()
	resp, b := testutil.Req(t, s, "POST", "/session", d, `{"title":"T1"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var ses struct {
		ID         string
		Title      string
		Agent      string
		Model      struct{ ID, ProviderID string }
		ProjectDir string
		Cost       float64
		Tokens     struct{ Input, Output int }
	}
	if err := json.Unmarshal(b, &ses); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if ses.Title != "T1" || ses.Agent != "build" || ses.Model.ID != "q" || ses.Model.ProviderID != "kido" {
		t.Fatalf("session = %+v", ses)
	}
	id := ses.ID

	// list scoped
	_, b = testutil.Req(t, s, "GET", "/session", d, "")
	var list []map[string]any
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d", len(list))
	}
	// other dir sees nothing
	_, b = testutil.Req(t, s, "GET", "/session", other, "")
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if len(list) != 0 {
		t.Fatalf("cross-dir leak: %d", len(list))
	}
	// get by id from other dir → 404
	resp, _ = testutil.Req(t, s, "GET", "/session/"+id, other, "")
	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
	// patch model+agent+title
	resp, b = testutil.Req(t, s, "PATCH", "/session/"+id, d, `{"title":"T2","agent":"yolo","model":"opencode/gpt-5-nano"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("patch: %d %s", resp.StatusCode, b)
	}
	var got struct {
		Title string
		Agent string
		Model struct{ ID, ProviderID string }
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if got.Title != "T2" || got.Agent != "yolo" || got.Model.ProviderID != "opencode" {
		t.Fatalf("patched = %+v", got)
	}
	// delete → gone
	resp, _ = testutil.Req(t, s, "DELETE", "/session/"+id, d, "")
	if resp.StatusCode != 204 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, _ = testutil.Req(t, s, "GET", "/session/"+id, d, "")
	if resp.StatusCode != 404 {
		t.Fatalf("after delete: %d", resp.StatusCode)
	}
}

func TestMessagesEndpoint(t *testing.T) {
	s := testutil.Boot(t)
	d := t.TempDir()
	_, b := testutil.Req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	if err := json.Unmarshal(b, &ses); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	resp, _ := testutil.Req(t, s, "POST", "/session/"+ses.ID+"/message", d, `{"text":"hello"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send: %d", resp.StatusCode)
	}
	// wait for the turn to settle
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, b = testutil.Req(t, s, "GET", "/session/status", d, "")
		var st struct {
			Sessions map[string]string `json:"sessions"`
		}
		if err := json.Unmarshal(b, &st); err != nil {
			t.Fatalf("unmarshal: %v (%s)", err, b)
		}
		if st.Sessions[ses.ID] == "idle" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	resp, b = testutil.Req(t, s, "GET", "/session/"+ses.ID+"/message", d, "")
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
	s := testutil.Boot(t)
	d := t.TempDir()
	_, b := testutil.Req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	if err := json.Unmarshal(b, &ses); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	id := ses.ID

	// subscribe SSE BEFORE sending (no pre-read: nothing is published yet)
	res := testutil.SSEConnect(t, s, d)
	resp, b := testutil.Req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"hello"}`)
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
	s.FakeDelay(200 * time.Millisecond)
	resp2, _ := testutil.Req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"again"}`)
	if resp2.StatusCode != 202 {
		t.Fatalf("second send: %d", resp2.StatusCode)
	}
	time.Sleep(50 * time.Millisecond)
	resp3, b3 := testutil.Req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"thrice"}`)
	if resp3.StatusCode != 409 {
		t.Fatalf("want 409 during busy, got %d %s", resp3.StatusCode, b3)
	}
	// envelope shape
	var env struct {
		Error struct{ Message string } `json:"error"`
	}
	if err := json.Unmarshal(b3, &env); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b3)
	}
	if env.Error.Message == "" {
		t.Fatalf("envelope = %s", b3)
	}
}

func TestAbortEndpoint(t *testing.T) {
	s := testutil.Boot(t)
	d := t.TempDir()
	_, b := testutil.Req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	if err := json.Unmarshal(b, &ses); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	s.FakeDelay(300 * time.Millisecond)
	_, _ = testutil.Req(t, s, "POST", "/session/"+ses.ID+"/message", d, `{"text":"slow"}`)
	time.Sleep(30 * time.Millisecond)
	resp, b2 := testutil.Req(t, s, "POST", "/session/"+ses.ID+"/abort", d, `{}`)
	var body struct {
		Aborted bool
	}
	if err := json.Unmarshal(b2, &body); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b2)
	}
	if resp.StatusCode != 200 || !body.Aborted {
		t.Fatalf("abort: %d %s", resp.StatusCode, b2)
	}
	// status now idle
	_, b3 := testutil.Req(t, s, "GET", "/session/status", d, "")
	var st struct {
		Sessions map[string]string `json:"sessions"`
	}
	if err := json.Unmarshal(b3, &st); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b3)
	}
	if st.Sessions[ses.ID] != "idle" {
		t.Fatalf("status = %v", st.Sessions)
	}
	// abort idle → aborted:false
	_, b4 := testutil.Req(t, s, "POST", "/session/"+ses.ID+"/abort", d, `{}`)
	var b5 struct{ Aborted bool }
	if err := json.Unmarshal(b4, &b5); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b4)
	}
	if b5.Aborted {
		t.Fatal("abort on idle must be false")
	}
}

func TestCommandEndpoint(t *testing.T) {
	s := testutil.Boot(t)
	d := t.TempDir()
	_, b := testutil.Req(t, s, "POST", "/session", d, `{}`)
	var ses struct{ ID string }
	if err := json.Unmarshal(b, &ses); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	// /new → new session id
	resp, b := testutil.Req(t, s, "POST", "/session/"+ses.ID+"/command", d, `{"command":"/new"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var out struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if out.SessionID == "" || out.SessionID == ses.ID {
		t.Fatalf("/new = %s", out.SessionID)
	}
	resp, b = testutil.Req(t, s, "POST", "/session/"+ses.ID+"/command", d, `{"command":"/model"}`)
	var client struct {
		Handled string `json:"handled"`
	}
	if err := json.Unmarshal(b, &client); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	if resp.StatusCode != 200 || client.Handled != "client" {
		t.Fatalf("/model = %d %s", resp.StatusCode, b)
	}
	// /quit is canonical; /exit is accepted as its alias (both client-handled)
	for _, c := range []string{"/quit", "/exit"} {
		resp, b = testutil.Req(t, s, "POST", "/session/"+ses.ID+"/command", d, `{"command":"`+c+`"}`)
		client = struct {
			Handled string `json:"handled"`
		}{}
		if err := json.Unmarshal(b, &client); err != nil {
			t.Fatalf("unmarshal %s: %v (%s)", c, err, b)
		}
		if resp.StatusCode != 200 || client.Handled != "client" {
			t.Fatalf("%s = %d %s", c, resp.StatusCode, b)
		}
	}
	resp, _ = testutil.Req(t, s, "POST", "/session/"+ses.ID+"/command", d, `{"command":"/bogus"}`)
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

// TestSendLogsFailedTurn: a failed model turn must not vanish silently —
// the send handler logs the turn's final error to yolo.log (upstream
// promptAsync parity), so the "invisible failure" has a diagnostic home.
func TestSendLogsFailedTurn(t *testing.T) {
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
	drv := fakellm.New(fakellm.Turn{Err: errors.New("boom")})
	eng := session.New(session.Deps{
		DB:      db,
		Bus:     b,
		Prov:    prov,
		Perm:    permSvc,
		Tools:   tool.Registry(),
		DataDir: dataDir,
		Cfg:     func(string) (*protocol.Config, error) { return &protocol.Config{}, nil },
		Drivers: map[string]llm.Driver{"kido": drv},
		Backoff: func(int) time.Duration { return time.Millisecond },
		Clock:   func() int64 { return time.Now().UnixMilli() },
	})
	logDir := t.TempDir()
	lob := log.New(logDir)
	t.Cleanup(lob.Close)
	dir := t.TempDir()
	h := server.New(server.Deps{
		DB:      db,
		Bus:     b,
		Engine:  eng,
		Prov:    prov,
		Perm:    permSvc,
		Config:  config.Loader{Env: map[string]string{}},
		WorkDir: dir,
		Dirs:    config.Dirs{Home: filepath.Join(root, "home"), Data: dataDir, Cache: filepath.Join(root, "cache")},
		Log:     lob,
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	do := func(path, payload string) (int, []byte) {
		req, e2 := http.NewRequest("POST", ts.URL+path, strings.NewReader(payload))
		if e2 != nil {
			t.Fatal(e2)
		}
		req.Header.Set("x-yolo-directory", dir)
		req.Header.Set("Content-Type", "application/json")
		resp, e2 := http.DefaultClient.Do(req)
		if e2 != nil {
			t.Fatal(e2)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, resp.Body)
		return resp.StatusCode, buf.Bytes()
	}

	code, body := do("/session", `{}`)
	if code/100 != 2 {
		t.Fatalf("create session: %d %s", code, body)
	}
	var ses struct {
		ID string
	}
	if err := json.Unmarshal(body, &ses); err != nil {
		t.Fatalf("unmarshal session: %v (%s)", err, body)
	}
	code, body = do("/session/"+ses.ID+"/message", `{"text":"hi"}`)
	if code != 202 {
		t.Fatalf("send: %d %s", code, body)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if s := eng.Status(ses.ID); s == protocol.StatusIdle {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("turn did not settle")
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, err := os.ReadFile(filepath.Join(logDir, "log", "yolo.log"))
	if err != nil {
		t.Fatalf("read yolo.log: %v", err)
	}
	if !strings.Contains(string(data), "boom") {
		t.Fatalf("failed turn not logged to yolo.log:\n%s", data)
	}
}
