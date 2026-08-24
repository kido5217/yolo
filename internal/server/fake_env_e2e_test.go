package server_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server"
	"github.com/kido5217/yolo/internal/server/testutil"
)

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
func newSrvFakeEnv(t *testing.T, script string) (*testutil.TestServer, *fake.Driver) {
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

// TestFakeEnvConversation drives a two-send conversation through the HTTP API
// with the env-gated fake driver; verifies messages/parts persist and that a
// later model request replays history including a tool result. Fully offline —
// the name deliberately avoids the "e2e" of scripts/e2e-live.sh (network,
// user-run).
func TestFakeEnvConversation(t *testing.T) {
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
	s.WaitIdle(t, s.Dir, id)
	resp, b = testutil.Req(t, s, "POST", "/session/"+id+"/message", s.Dir, `{"text":"again"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send 2: %d %s", resp.StatusCode, b)
	}
	s.WaitIdle(t, s.Dir, id)

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
