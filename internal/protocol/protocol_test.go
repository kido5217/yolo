package protocol_test

import (
	"encoding/json"
	"regexp"
	"strings"
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
	a := p.NewID("msg")
	if a == p.NewID("msg") {
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
	want := `{"id":"ses_test1234567890123456","projectID":"prj_123","directory":"/w","title":"t","model":{"id":"Qwen3.8-27B","providerID":"kido"},"cost":0.5,"tokens":{"input":10,"output":20,"reasoning":0,"cache":{"read":1,"write":2}},"version":"1","time":{"created":1,"updated":2}}`
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
	a := p.Message{ID: "msg_2", SessionID: "ses_1", Role: "assistant",
		Time: p.MessageTime{Created: 1, Completed: 2}, ParentID: "msg_1",
		ModelID: "Qwen3.8-27B", ProviderID: "kido", Mode: "primary", Agent: "build",
		Path: &p.MessagePath{Cwd: "/w", Root: "/w"}, Cost: 0.1,
		Tokens: &p.Tokens{Input: 3, Output: 4}}
	ba, _ := json.Marshal(a)
	for _, want := range []string{`"parentID":"msg_1"`, `"modelID":"Qwen3.8-27B"`, `"providerID":"kido"`, `"path":{"cwd":"/w","root":"/w"}`, `"cost":0.1`, `"tokens":{"input":3,"output":4,"reasoning":0,"cache":{"read":0,"write":0}}`} {
		if !strings.Contains(string(ba), want) {
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
		if !strings.Contains(string(bd), want) {
			t.Fatalf("tool part missing %s:\n%s", want, bd)
		}
	}
}

func TestMakeEvent(t *testing.T) {
	e, err := p.MakeEvent(p.EventTypePermissionAsked, p.PermissionAskedProps{
		ID: "per_1", SessionID: "ses_1", Permission: "bash",
		Patterns: []string{"ls"}, Metadata: map[string]any{"tool": "bash"},
		Always: []string{"ls"},
		Tool:   &p.PermissionToolRef{MessageID: "msg_2", CallID: "call_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !evtRe.MatchString(e.ID) || e.Type != p.EventTypePermissionAsked {
		t.Fatalf("envelope bad: %+v", e)
	}
	b, _ := json.Marshal(e)
	for _, want := range []string{`"type":"permission.asked"`, `"permission":"bash"`, `"patterns":["ls"]`, `"always":["ls"]`, `"tool":{"messageID":"msg_2","callID":"call_1"}`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("event missing %s:\n%s", want, b)
		}
	}
}

func TestParsePerms(t *testing.T) {
	rules, err := p.ParsePerms(map[string]any{
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
		if rules, err := p.ParsePerms(m); err == nil {
			t.Fatalf("ParsePerms(%v) = %+v, want error", m, rules)
		}
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
