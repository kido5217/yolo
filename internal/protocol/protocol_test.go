package protocol_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
)

var sesRe = regexp.MustCompile(`^ses_[2-9A-HJK-NP-Zb-hj-np-z]{20}$`)
var evtRe = regexp.MustCompile(`^evt_[2-9A-HJK-NP-Zb-hj-np-z]{20}$`)

func TestNewIDFormats(t *testing.T) {
	if !sesRe.MatchString(protocol.NewID("ses")) {
		t.Fatalf("bad session id format: %q", protocol.NewID("ses"))
	}
	if got := protocol.NewID("ses")[:4]; got != "ses_" {
		t.Fatalf("prefix = %q", got)
	}
	if !evtRe.MatchString(protocol.NewEventID()) {
		t.Fatalf("bad event id: %q", protocol.NewEventID())
	}
	a := protocol.NewID("msg")
	if a == protocol.NewID("msg") {
		t.Fatal("ids are not random")
	}
}

func TestSessionWireShape(t *testing.T) {
	s := protocol.Session{
		ID: "ses_test1234567890123456", ProjectID: "prj_123", Directory: "/w",
		Title: "t", Cost: 0.5, Version: "1",
		Model: &protocol.ModelRef{ID: "Qwen3.8-27B", ProviderID: "kido"},
		Time:  protocol.SessionTime{Created: 1, Updated: 2},
		Tokens: protocol.Tokens{Input: 10, Output: 20, Reasoning: 0,
			Cache: protocol.CacheTokens{Read: 1, Write: 2}},
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"ses_test1234567890123456","projectID":"prj_123","directory":"/w","title":"t","model":{"id":"Qwen3.8-27B","providerID":"kido"},"cost":0.5,"tokens":{"input":10,"output":20,"reasoning":0,"cache":{"read":1,"write":2}},"version":"1","time":{"created":1,"updated":2}}`
	if string(b) != want {
		t.Fatalf("\ngot  %s\nwant %s", b, want)
	}
	var back protocol.Session
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Model.ProviderID != "kido" || back.Tokens.Cache.Write != 2 {
		t.Fatal("round-trip mismatch")
	}
}

func TestMessageRoles(t *testing.T) {
	u := protocol.Message{ID: "msg_1", SessionID: "ses_1", Role: "user",
		Time: protocol.MessageTime{Created: 1}, Agent: "build",
		Model: &protocol.MessageModel{ProviderID: "kido", ModelID: "Qwen3.8-27B"}}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	// user message must not carry any top-level assistant-only fields
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"parentID", "modelID", "providerID", "path", "cost", "tokens", "finish"} {
		if _, ok := top[banned]; ok {
			t.Fatalf("user msg carries top-level %s: %s", banned, b)
		}
	}
	a := protocol.Message{ID: "msg_2", SessionID: "ses_1", Role: "assistant",
		Time: protocol.MessageTime{Created: 1, Completed: 2}, ParentID: "msg_1",
		ModelID: "Qwen3.8-27B", ProviderID: "kido", Mode: "primary", Agent: "build",
		Path: &protocol.MessagePath{Cwd: "/w", Root: "/w"}, Cost: 0.1,
		Tokens: &protocol.Tokens{Input: 3, Output: 4}}
	ba, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"parentID":"msg_1"`, `"modelID":"Qwen3.8-27B"`, `"providerID":"kido"`, `"path":{"cwd":"/w","root":"/w"}`, `"cost":0.1`, `"tokens":{"input":3,"output":4,"reasoning":0,"cache":{"read":0,"write":0}}`} {
		if !strings.Contains(string(ba), want) {
			t.Fatalf("assistant msg missing %s:\n%s", want, ba)
		}
	}
}

func TestPartAndToolStateShapes(t *testing.T) {
	text := protocol.Part{ID: "prt_1", SessionID: "ses_1", MessageID: "msg_2", Type: "text", Text: "hi", Time: protocol.PartTime{Start: 1}}
	b, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"id":"prt_1","sessionID":"ses_1","messageID":"msg_2","type":"text","text":"hi","time":{"start":1}}`; string(b) != want {
		t.Fatalf("text part:\n%s\nwant\n%s", b, want)
	}
	done := protocol.Part{ID: "prt_2", SessionID: "ses_1", MessageID: "msg_2", Type: "tool", CallID: "call_1", Tool: "bash",
		State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": "ls"}, Output: "ok", Title: "ls", Time: protocol.PartTime{Start: 1, End: 2}}}
	bd, err := json.Marshal(done)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type":"tool"`, `"callID":"call_1"`, `"tool":"bash"`, `"status":"completed"`, `"output":"ok"`, `"end":2`} {
		if !strings.Contains(string(bd), want) {
			t.Fatalf("tool part missing %s:\n%s", want, bd)
		}
	}
}

func TestMakeEvent(t *testing.T) {
	e, err := protocol.MakeEvent(protocol.EventTypePermissionAsked, protocol.PermissionAskedProps{
		ID: "per_1", SessionID: "ses_1", Permission: "bash",
		Patterns: []string{"ls"}, Metadata: map[string]any{"tool": "bash"},
		Always: []string{"ls"},
		Tool:   &protocol.PermissionToolRef{MessageID: "msg_2", CallID: "call_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !evtRe.MatchString(e.ID) || e.Type != protocol.EventTypePermissionAsked {
		t.Fatalf("envelope bad: %+v", e)
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type":"permission.asked"`, `"permission":"bash"`, `"patterns":["ls"]`, `"always":["ls"]`, `"tool":{"messageID":"msg_2","callID":"call_1"}`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("event missing %s:\n%s", want, b)
		}
	}
}

func TestParsePerms(t *testing.T) {
	rules, err := protocol.ParsePerms(map[string]any{
		"bash": "ask",
		"edit": "allow",
		"read": map[string]any{"*.env": "ask"},
	})
	if err != nil {
		t.Fatal(err)
	}
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

func TestParsePermsRejectsNonStringValues(t *testing.T) {
	for _, m := range []map[string]any{
		{"bash": map[string]any{"echo": 123}},         // pattern value not a string
		{"bash": 5},                                   // top-level value not a string/object
		{"*": map[string]any{"rm": map[string]any{}}}, // nested map instead of action
	} {
		if rules, err := protocol.ParsePerms(m); err == nil {
			t.Fatalf("ParsePerms(%v) = %+v, want error", m, rules)
		}
	}
}

func TestSessionStatusWire(t *testing.T) {
	b, err := json.Marshal(protocol.SessionStatus{Type: protocol.StatusRetry, Attempt: 2, Message: "429", Next: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"type":"retry","attempt":2,"message":"429","next":2000}`; string(b) != want {
		t.Fatalf("status shape: %s", b)
	}
	bi, err := json.Marshal(protocol.SessionStatus{Type: protocol.StatusIdle})
	if err != nil {
		t.Fatal(err)
	}
	if string(bi) != `{"type":"idle"}` {
		t.Fatalf("idle shape: %s", bi)
	}
}
