package server_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/llm"
	fakellm "github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/server/testutil"
)

// updateGolden regenerates testdata/golden/*.json
// (run: go test ./internal/server/ -run Golden -update).
var updateGolden = flag.Bool("update", false, "regenerate server contract golden files")

const goldenDir = "testdata/golden"

// idRe matches the generated ID contract (prefix + '_' + body).
var idRe = regexp.MustCompile(`^(ses|msg|prt|prj|evt|perm|cmd|req|mod)_[0-9A-Za-z]+$`)

// normalizer rewrites a decoded JSON tree so a response can be compared
// byte-for-byte across runs: generated IDs -> <PREFIX><n> (deduped per
// concrete id), the test project dir + its basename -> <DIR>/<DIRNAME>, and
// epoch-millisecond integers (>= 1e11) -> <T>. Maps are re-emitted key-sorted.
type normalizer struct {
	dir  string
	base string
	seen map[string]string
	cnt  map[string]int
}

func newNormalizer(dir string) *normalizer {
	return &normalizer{dir: dir, base: filepath.Base(dir), seen: map[string]string{}, cnt: map[string]int{}}
}

func (n *normalizer) idPlaceholder(s string) (string, bool) {
	m := idRe.FindStringSubmatch(s)
	if m == nil {
		return s, false
	}
	if ph, ok := n.seen[s]; ok {
		return ph, true
	}
	n.cnt[m[1]]++
	ph := strings.ToUpper(m[1]) + strconv.Itoa(n.cnt[m[1]])
	n.seen[s] = ph
	return ph, true
}

func (n *normalizer) str(s string) any {
	if ph, ok := n.idPlaceholder(s); ok {
		return ph
	}
	switch {
	case s == n.dir:
		return "<DIR>"
	case s == n.base:
		return "<DIRNAME>"
	case strings.Contains(s, n.dir):
		return strings.ReplaceAll(s, n.dir, "<DIR>")
	}
	return s
}

func (n *normalizer) num(f float64) any {
	if f >= 1e11 && f == math.Trunc(f) {
		return "<T>"
	}
	return f
}

func (n *normalizer) walk(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		for _, k := range keys {
			nk, _ := n.str(k).(string)
			out[nk] = n.walk(t[k])
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = n.walk(e)
		}
		return out
	case string:
		return n.str(t)
	case float64:
		return n.num(t)
	default:
		return v
	}
}

// golden performs one canonical request, normalizes the JSON body, and compares
// (or, with -update, regenerates) it against testdata/golden/<name>.json.
func golden(t *testing.T, s *testutil.TestServer, name, method, path, dir, body string, want int) {
	t.Helper()
	resp, b := testutil.Req(t, s, method, path, dir, body)
	if resp.StatusCode != want {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, want, b)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s %s: decode: %v", method, path, err)
	}
	normDir := dir
	if normDir == "" {
		normDir = s.Dir
	}
	norm := newNormalizer(normDir).walk(v)
	data, err := json.MarshalIndent(norm, "", "  ")
	if err != nil {
		t.Fatalf("%s %s: encode: %v", method, path, err)
	}
	data = append(data, '\n')
	gp := filepath.Join(goldenDir, name+".json")
	if *updateGolden {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(gp, data, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", gp, err)
		}
		return
	}
	wantData, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("no golden %s (run: go test ./internal/server/ -run Golden -update): %v", gp, err)
	}
	if !bytes.Equal(data, wantData) {
		t.Fatalf("golden %s mismatch:\n--- got ---\n%s\n--- want ---\n%s", gp, data, wantData)
	}
}

// mkSession creates a session in dir ("") and returns its id.
func mkSession(t *testing.T, s *testutil.TestServer, dir, title string) string {
	t.Helper()
	resp, b := testutil.Req(t, s, "POST", "/session", dir, `{"title":"`+title+`"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("create session: %d %s", resp.StatusCode, b)
	}
	var ses struct{ ID string }
	_ = json.Unmarshal(b, &ses)
	if ses.ID == "" {
		t.Fatalf("create session: empty id: %s", b)
	}
	return ses.ID
}

// waitIdle polls /session/status until the session reports idle.
func waitIdle(t *testing.T, s *testutil.TestServer, dir, id string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, b := testutil.Req(t, s, "GET", "/session/status", dir, "")
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

func TestGoldenResponses(t *testing.T) {
	t.Run("health", func(t *testing.T) {
		s := testutil.Boot(t)
		golden(t, s, "health", "GET", "/global/health", "", "", 200)
	})
	t.Run("path", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "path", "GET", "/path", d, "", 200)
	})
	t.Run("project", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "project", "GET", "/project/current", d, "", 200)
	})
	t.Run("session_list", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		mkSession(t, s, d, "Golden")
		golden(t, s, "session_list", "GET", "/session", d, "", 200)
	})
	t.Run("session_create", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "session_create", "POST", "/session", d, `{"title":"Golden"}`, 201)
	})
	t.Run("session_get", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		id := mkSession(t, s, d, "Golden")
		golden(t, s, "session_get", "GET", "/session/"+id, d, "", 200)
	})
	t.Run("session_patch", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		id := mkSession(t, s, d, "Golden")
		golden(t, s, "session_patch", "PATCH", "/session/"+id, d,
			`{"title":"Patched","agent":"yolo","model":"opencode/gpt-5-nano"}`, 200)
	})
	t.Run("message_list", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		id := mkSession(t, s, d, "Golden")
		resp, b := testutil.Req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"hello"}`)
		if resp.StatusCode != 202 {
			t.Fatalf("send: %d %s", resp.StatusCode, b)
		}
		waitIdle(t, s, d, id)
		golden(t, s, "message_list", "GET", "/session/"+id+"/message", d, "", 200)
	})
	t.Run("provider", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "provider", "GET", "/provider", d, "", 200)
	})
	t.Run("config", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		testutil.WriteCfg(t, d, `{"model":"kido/q","permission":{"edit":"ask"}}`)
		golden(t, s, "config", "GET", "/config", d, "", 200)
	})
	t.Run("agent", func(t *testing.T) {
		s := testutil.Boot(t)
		golden(t, s, "agent", "GET", "/agent", "", "", 200)
	})
	t.Run("command", func(t *testing.T) {
		s := testutil.Boot(t)
		golden(t, s, "command", "GET", "/command", "", "", 200)
	})
	t.Run("permission_empty", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		golden(t, s, "permission_empty", "GET", "/permission", d, "", 200)
	})
	t.Run("status", func(t *testing.T) {
		s := testutil.Boot(t)
		d := t.TempDir()
		mkSession(t, s, d, "Golden")
		golden(t, s, "status", "GET", "/session/status", d, "", 200)
	})
}

// sseMsgField pulls a string field out of a message.updated frame's info object.
func sseMsgField(t *testing.T, f testutil.SSEFrame, field string) string {
	t.Helper()
	info, ok := f.Properties["info"].(map[string]any)
	if !ok {
		return ""
	}
	s, _ := info[field].(string)
	return s
}

// ssePart returns the part object of a message.part.updated frame.
func ssePart(t *testing.T, f testutil.SSEFrame) map[string]any {
	t.Helper()
	p, _ := f.Properties["part"].(map[string]any)
	return p
}

func sseTypes(frames []testutil.SSEFrame) []string {
	out := make([]string, 0, len(frames))
	for _, f := range frames {
		out = append(out, f.Type)
	}
	return out
}

// TestSSEOrdering asserts the EXACT faithful frame order for one text turn
// (fake driver). The user message/part are published synchronously in Send
// BEFORE the turn goroutine emits busy — matching upstream v1.18.18, and
// deviating from the plan's pinned "busy first" order (see PROGRESS.md).
func TestSSEOrdering(t *testing.T) {
	s := testutil.Boot(t)
	d := t.TempDir()
	id := mkSession(t, s, d, "Sse") // explicit title: no title-generation side request

	res := testutil.SSEConnect(t, s, d)
	s.WaitSubscribe(t, 1) // subscription live before we publish the turn
	resp, b := testutil.Req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"hello"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send: %d %s", resp.StatusCode, b)
	}
	var out struct {
		MessageID string `json:"message_id"`
	}
	_ = json.Unmarshal(b, &out)
	userMsgID := out.MessageID

	var frames []testutil.SSEFrame
	for i := 0; i < 200; i++ {
		f := res.Frame(t)
		frames = append(frames, f)
		if f.Type == "session.status" && f.String("status") == "idle" {
			break
		}
	}
	if len(frames) == 0 {
		t.Fatal("no SSE frames received")
	}
	types := sseTypes(frames)

	// frames 0..2 are fixed by index:
	f0 := frames[0]
	if f0.Type != "message.updated" || sseMsgField(t, f0, "role") != "user" || sseMsgField(t, f0, "id") != userMsgID {
		t.Fatalf("frame 0 = type %s info %v, want user message %s (got %v)", f0.Type, f0.Properties["info"], userMsgID, types)
	}
	f1 := frames[1]
	if f1.Type != "message.part.updated" {
		t.Fatalf("frame 1 = %s, want message.part.updated (got %v)", f1.Type, types)
	}
	if p := ssePart(t, f1); p["messageID"] != userMsgID || p["type"] != "text" {
		t.Fatalf("frame 1 part = %v, want user text part %s", p, userMsgID)
	}
	f2 := frames[2]
	if f2.Type != "session.status" || f2.String("status") != "busy" {
		t.Fatalf("frame 2 = %s %v, want session.status busy (got %v)", f2.Type, f2.Properties["status"], types)
	}

	// The rest, by relative order:
	//   assistant message (round start) < first assistant part < last delta
	//   < final assistant part < last assistant message < idle (last frame)
	var ai, pi, di, mi, lastPart, lastDelta int
	ai, pi, di, mi, lastPart = -1, -1, -1, -1, -1
	for i, f := range frames {
		switch f.Type {
		case "message.updated":
			if sseMsgField(t, f, "role") == "assistant" {
				if ai == -1 {
					ai = i
				}
				mi = i // last assistant message.updated
			}
		case "message.part.updated":
			if p := ssePart(t, f); p["messageID"] != userMsgID {
				if pi == -1 {
					pi = i
				}
				lastPart = i // last assistant part.updated (the final frame)
			}
		case "message.part.delta":
			di = i
			lastDelta = i
		}
	}
	for name, idx := range map[string]int{"assistantMsg": ai, "assistantPart": pi, "delta": di, "assistantFinal": mi, "finalPart": lastPart} {
		if idx < 0 {
			t.Fatalf("missing frame %s in %v", name, types)
		}
	}
	if ai <= 2 || pi <= ai || di <= pi || lastPart <= di || mi <= lastPart {
		t.Fatalf("ordering violation: busy(2) < assistantMsg(%d) < assistantPart(%d) < lastDelta(%d) < finalPart(%d) < assistantFinal(%d); got %v",
			ai, pi, di, lastPart, mi, types)
	}
	// the final assistant part carries the full text with an end timestamp
	lp := ssePart(t, frames[lastPart])
	if lp["type"] != "text" || strings.TrimSpace(lpText(lp)) == "" {
		t.Fatalf("final assistant part = %v, want non-empty text", lp)
	}
	if tm, ok := lp["time"].(map[string]any); !ok || tm["end"] == nil {
		t.Fatalf("final assistant part = %v, want time.end set", lp)
	}
	// idle is the last frame
	last := frames[len(frames)-1]
	if last.Type != "session.status" || last.String("status") != "idle" {
		t.Fatalf("last frame = %s %v, want session.status idle", last.Type, last.Properties["status"])
	}
	_ = lastDelta
}

func lpText(p map[string]any) string {
	s, _ := p["text"].(string)
	return s
}

// osEnvMap snapshots the process environment as a map (how production feeds
// the YOLO_LLM gate).
func osEnvMap() map[string]string {
	m := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

// newSrvFakeEnv boots the full stack but wires the kido driver from the
// YOLO_LLM/YOLO_FAKE_SCRIPT environment (the M5 env gate), so the e2e runs the
// same path as `yolo serve` with YOLO_LLM=fake.
func newSrvFakeEnv(t *testing.T, script string) (*testutil.TestServer, *fakellm.Driver) {
	t.Helper()
	t.Setenv("YOLO_LLM", "fake")
	t.Setenv("YOLO_FAKE_SCRIPT", script)
	drv, err := server.FakeFromEnv(osEnvMap())
	if err != nil {
		t.Fatalf("FakeFromEnv: %v", err)
	}
	if drv == nil {
		t.Fatal("FakeFromEnv returned nil driver with YOLO_LLM=fake set")
	}
	return testutil.BootWithDriver(t, drv), drv
}

// TestFakeEnvE2E drives a two-send conversation through the HTTP API with the
// env-gated fake driver; verifies messages/parts persist and that a later model
// request replays history including a tool result.
func TestFakeEnvE2E(t *testing.T) {
	script := `[
		{"parts":[{"kind":"tool","name":"read","call_id":"call_1","args":{"filePath":"note.txt"},"finish":"tool_calls","usage":{"input":1,"output":1}}],"delay_ms":0},
		{"parts":[{"kind":"text","text":"Done reading.","finish":"stop","usage":{"input":2,"output":2}}],"delay_ms":0}
	]`
	sp := filepath.Join(t.TempDir(), "script.json")
	if err := os.WriteFile(sp, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	s, drv := newSrvFakeEnv(t, sp)
	// the session (and the read tool's relative path) resolves against the
	// server work dir, so the note must live there
	if err := os.WriteFile(filepath.Join(s.Dir, "note.txt"), []byte("hello from note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	id := mkSession(t, s, s.Dir, "E2E")

	resp, b := testutil.Req(t, s, "POST", "/session/"+id+"/message", s.Dir, `{"text":"read note please"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send 1: %d %s", resp.StatusCode, b)
	}
	waitIdle(t, s, s.Dir, id)
	resp, b = testutil.Req(t, s, "POST", "/session/"+id+"/message", s.Dir, `{"text":"again"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send 2: %d %s", resp.StatusCode, b)
	}
	waitIdle(t, s, s.Dir, id)

	// messages + parts persisted: user x2, an assistant with a completed
	// `read` tool part (output = the note) and the closing text part.
	resp, b = testutil.Req(t, s, "GET", "/session/"+id+"/message", s.Dir, "")
	if resp.StatusCode != 200 {
		t.Fatalf("messages: %d %s", resp.StatusCode, b)
	}
	var msgs []protocol.MessageWithParts
	if err := json.Unmarshal(b, &msgs); err != nil {
		t.Fatalf("decode messages: %v", err)
	}
	var userTexts []string
	var sawReadTool, sawDoneText bool
	for _, m := range msgs {
		if m.Info.Role == "user" {
			for _, p := range m.Parts {
				if p.Type == "text" {
					userTexts = append(userTexts, p.Text)
				}
			}
			continue
		}
		for _, p := range m.Parts {
			if p.Type == "tool" && p.Tool == "read" && p.State != nil && p.State.Status == "completed" {
				sawReadTool = true
				if !strings.Contains(p.State.Output, "hello from note") {
					t.Fatalf("read tool output missing note: %q", p.State.Output)
				}
			}
			if p.Type == "text" && p.Text == "Done reading." {
				sawDoneText = true
			}
		}
	}
	if len(userTexts) != 2 || userTexts[0] != "read note please" || userTexts[1] != "again" {
		t.Fatalf("user texts = %v, want [read note please again]", userTexts)
	}
	if !sawReadTool {
		t.Fatalf("no completed read tool part in %d messages", len(msgs))
	}
	if !sawDoneText {
		t.Fatalf("no closing text part in %d messages", len(msgs))
	}

	// history replay: later model requests carry the tool result. Send 1's
	// follow-up (request index 1) and send 2 (the last request) must each
	// contain a RoleTool message with the note content.
	reqs := drv.Requests()
	if len(reqs) < 3 {
		t.Fatalf("want >=3 model requests (2 for send 1's tool round + 1 for send 2), got %d", len(reqs))
	}
	assertReplayed := func(idx int, label string) {
		var toolContent string
		for _, m := range reqs[idx].Messages {
			if m.Role == llm.RoleTool {
				toolContent = m.Content
			}
		}
		if !strings.Contains(toolContent, "hello from note") {
			t.Fatalf("%s: request %d did not replay the tool result (tool content %q)", label, idx, toolContent)
		}
	}
	assertReplayed(1, "send-1 follow-up")

	last := reqs[len(reqs)-1]
	var users []string
	for _, m := range last.Messages {
		if m.Role == llm.RoleUser {
			users = append(users, m.Content)
		}
	}
	if len(users) < 2 || users[0] != "read note please" || users[len(users)-1] != "again" {
		t.Fatalf("send-2 history user messages = %v, want start 'read note please' end 'again'", users)
	}
	assertReplayed(len(reqs)-1, "send-2")
}

// TestScopeMatrix verifies directory scoping: absent header falls back to the
// server work dir, and every id-scoped route 404s for a session id belonging to
// a different directory.
func TestScopeMatrix(t *testing.T) {
	s := testutil.Boot(t)
	wd := s.Dir

	t.Run("no_header_uses_workdir", func(t *testing.T) {
		resp, b := testutil.Req(t, s, "GET", "/path", "", "")
		var p map[string]string
		_ = json.Unmarshal(b, &p)
		if resp.StatusCode != 200 || p["directory"] != wd {
			t.Fatalf("no-header /path = %d %s, want dir %s", resp.StatusCode, b, wd)
		}
		mkSession(t, s, "", "Cwd") // created without a header -> work dir
		_, b = testutil.Req(t, s, "GET", "/session", "", "")
		var list []map[string]any
		_ = json.Unmarshal(b, &list)
		if len(list) != 1 {
			t.Fatalf("workdir session list = %d, want 1: %s", len(list), b)
		}
	})

	// id-scoped routes: each must 404 for a session id scoped to another dir.
	idScoped := []struct {
		method, path string
		body         string
	}{
		{"GET", "/session/%s", ""},
		{"PATCH", "/session/%s", `{"title":"x"}`},
		{"DELETE", "/session/%s", ""},
		{"GET", "/session/%s/message", ""},
		{"POST", "/session/%s/message", `{"text":"hi"}`},
		{"POST", "/session/%s/abort", ""},
		{"POST", "/session/%s/command", `{"command":"/new"}`},
	}
	dA := t.TempDir()
	dB := t.TempDir()
	id := mkSession(t, s, dA, "A")
	for _, r := range idScoped {
		resp, b := testutil.Req(t, s, r.method, fmt.Sprintf(r.path, id), dB, r.body)
		if resp.StatusCode != 404 {
			t.Fatalf("%s %s (other dir) = %d, want 404: %s", r.method, r.path, resp.StatusCode, b)
		}
	}
	// sanity: the owning dir resolves the id
	resp, b := testutil.Req(t, s, "GET", "/session/"+id, dA, "")
	if resp.StatusCode != 200 {
		t.Fatalf("GET /session/%s (owning dir) = %d, want 200: %s", id, resp.StatusCode, b)
	}
}
