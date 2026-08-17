package session_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/llm"
	fakellm "github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

type harness struct {
	db  *storage.DB
	bus *bus.Bus
	eng *session.Engine
	drv *fakellm.Driver

	eventsMu sync.Mutex
	events   []protocol.Event
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	b := bus.New()
	ch, unsub := b.Subscribe()
	h := &harness{db: db, bus: b}
	t.Cleanup(func() {
		unsub()
		db.Close()
	})
	go func() {
		for e := range ch {
			h.eventsMu.Lock()
			h.events = append(h.events, e)
			h.eventsMu.Unlock()
		}
	}()
	return h
}

func (h *harness) build(t *testing.T) {
	t.Helper()
	drv := fakellm.New()
	reg, err := provider.NewWithSeams(t.Context(), t.TempDir(), func(providerID string) (provider.Info, provider.Model, error) {
		return provider.Info{ID: "kido", Name: "kido", BaseURL: "http://fake", KeyRequired: false},
			provider.Model{ID: "q", Name: "q", Adapter: "openai", Context: 100000, ToolCall: true}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	h.drv = drv
	h.eng = session.New(session.Deps{
		DB:      h.db,
		Bus:     h.bus,
		Prov:    reg,
		Perm:    permission.New(h.db, h.bus),
		Tools:   tool.Registry(),
		DataDir: t.TempDir(),
		Cfg:     func(string) (*protocol.Config, error) { return &protocol.Config{}, nil },
		Drivers: map[string]llm.Driver{"kido": drv},
		Clock:   func() int64 { return time.Now().UnixMilli() },
	})
}

func (h *harness) startSession(t *testing.T, dir string) string {
	t.Helper()
	id := protocol.NewID("ses")
	now := time.Now().UnixMilli()
	err := h.db.CreateSession(storage.SessionRow{
		ID: id, ProjectDir: dir, Model: "kido/q", Agent: "build",
		Title: "New session", TimeCreated: now, TimeUpdated: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (h *harness) eventCount(cond func(protocol.Event) bool) int {
	h.eventsMu.Lock()
	defer h.eventsMu.Unlock()
	n := 0
	for _, e := range h.events {
		if cond(e) {
			n++
		}
	}
	return n
}

func (h *harness) waitForEvent(t *testing.T, cond func(protocol.Event) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.eventsMu.Lock()
		found := false
		for _, e := range h.events {
			if cond(e) {
				found = true
				break
			}
		}
		h.eventsMu.Unlock()
		if found {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for event")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func statusType(t *testing.T, e protocol.Event) protocol.SessionStatus {
	t.Helper()
	if e.Type != protocol.EventTypeSessionStatus {
		t.Fatalf("event type %q", e.Type)
	}
	var p protocol.SessionStatusProps
	if err := json.Unmarshal(e.Properties, &p); err != nil {
		t.Fatal(err)
	}
	return p.Status
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

func isTitleReq(r llm.Request) bool {
	return len(r.Messages) > 0 && r.Messages[0].Role == llm.RoleSystem &&
		strings.HasPrefix(r.Messages[0].Content, "You are a title generator")
}

func nonTitle(log []llm.Request) []llm.Request {
	out := []llm.Request{}
	for _, r := range log {
		if isTitleReq(r) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSingleTextTurnEndToEnd(t *testing.T) {
	h := newHarness(t)
	h.build(t)

	h.drv.Turns = []fakellm.Turn{{Parts: []llm.Part{
		{Kind: "text", Text: "Hel"},
		{Kind: "text", Text: "lo"},
		{Kind: "text", Text: "", Finish: "stop", Usage: &llm.Usage{Input: 42, Output: 7}},
	}}}

	ses := h.startSession(t, t.TempDir())
	done := make(chan struct{})
	var errMsg error
	waitIdle(t, h, ses, func() {
		res, err := h.eng.Send(t.Context(), ses, "say hi", func(err error) {
			errMsg = err
			close(done)
		})
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if res.MessageID == "" || res.PartID == "" {
			t.Fatalf("SendResult: %+v", res)
		}
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onDone not called")
	}
	if h.eng.Status(ses) != protocol.StatusIdle {
		t.Fatal("status not idle")
	}
	if errMsg != nil {
		t.Fatalf("turn error: %v", errMsg)
	}

	msgs, err := h.db.ListMessages(ses)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("roles %q %q", msgs[0].Role, msgs[1].Role)
	}

	ump, err := h.db.ListParts(msgs[0].ID)
	if err != nil || len(ump) != 1 {
		t.Fatalf("user parts: %v, err %v", ump, err)
	}
	up, err := storage.PartToProtocol(ump[0])
	if err != nil {
		t.Fatal(err)
	}
	if up.Type != "text" || up.Text != "say hi" {
		t.Fatalf("user part = %+v", up)
	}

	amp, err := h.db.ListParts(msgs[1].ID)
	if err != nil || len(amp) != 1 {
		t.Fatalf("assistant parts: %v, err %v", amp, err)
	}
	ap, err := storage.PartToProtocol(amp[0])
	if err != nil {
		t.Fatal(err)
	}
	if ap.Type != "text" || ap.Text != "Hello" {
		t.Fatalf("assistant part = %+v", ap)
	}

	if msgs[1].Tokens.Input != 42 || msgs[1].Tokens.Output != 7 {
		t.Fatalf("tokens = %+v", msgs[1].Tokens)
	}

	reqs := nonTitle(h.drv.Requests())
	if len(reqs) != 1 {
		t.Fatalf("model rounds = %d, want 1", len(reqs))
	}
	req := reqs[0]
	if req.Model != "q" {
		t.Fatalf("model = %q", req.Model)
	}
	if req.Messages[0].Role != llm.RoleSystem {
		t.Fatalf("first message role = %q", req.Messages[0].Role)
	}
	if req.Messages[len(req.Messages)-1].Role != llm.RoleUser {
		t.Fatalf("last message role = %q", req.Messages[len(req.Messages)-1].Role)
	}
	if len(req.Tools) != 7 {
		t.Fatalf("tools = %d, want 7", len(req.Tools))
	}

	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeSessionStatus {
			return false
		}
		return statusType(t, e).Type == protocol.StatusBusy
	})
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeMessagePartDelta {
			return false
		}
		var p protocol.MessagePartDeltaProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			t.Fatal(err)
		}
		return p.Field == "text" && p.Delta == "Hel"
	})
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeMessagePartUpdated {
			return false
		}
		var p protocol.MessagePartUpdatedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			t.Fatal(err)
		}
		return p.Part.Text == "Hello"
	})
	h.waitForEvent(t, func(e protocol.Event) bool {
		return e.Type == protocol.EventTypeSessionStatus && statusType(t, e).Type == protocol.StatusIdle
	})
	if n := h.eventCount(func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeMessagePartUpdated {
			return false
		}
		var p protocol.MessagePartUpdatedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			return false
		}
		return p.Part.Text == "Hello"
	}); n != 1 {
		t.Fatalf("final part.updated count = %d, want 1", n)
	}
}

func TestHistoryReplayIncludesToolResults(t *testing.T) {
	h := newHarness(t)
	h.build(t)

	d := t.TempDir()
	fp := filepath.Join(d, "f.txt")
	writeFile(t, fp, "content")

	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{
			{Kind: "text", Text: "checking"},
			{Kind: "tool", Name: "read", CallID: "call_1", Text: fmt.Sprintf(`{"filePath":%q}`, fp)},
			{Kind: "text", Text: "", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 2}},
		}},
		{Parts: []llm.Part{
			{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 3, Output: 4}},
		}},
	}

	ses := h.startSession(t, d)
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "read f.txt", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	reqs := nonTitle(h.drv.Requests())
	if len(reqs) != 2 {
		t.Fatalf("model rounds = %d, want 2", len(reqs))
	}
	req := reqs[1]

	roles := make([]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		roles = append(roles, string(m.Role))
	}
	if len(roles) < 4 {
		t.Fatalf("roles = %v", roles)
	}
	tail := roles[len(roles)-4:]
	want := []string{string(llm.RoleUser), string(llm.RoleAssistant), string(llm.RoleTool), string(llm.RoleUser)}
	for i := range want {
		if tail[i] != want[i] {
			t.Fatalf("role tail = %v, want %v", tail, want)
		}
	}

	var asst *llm.Message
	for i := range req.Messages {
		if req.Messages[i].Role == llm.RoleAssistant {
			asst = &req.Messages[i]
			break
		}
	}
	if asst == nil {
		t.Fatal("no assistant message in round 2 request")
	}
	if len(asst.ToolCalls) != 1 || asst.ToolCalls[0].Name != "read" {
		t.Fatalf("tool calls = %+v", asst.ToolCalls)
	}
	var toolMsgs []llm.Message
	for _, m := range req.Messages {
		if m.Role == llm.RoleTool {
			toolMsgs = append(toolMsgs, m)
		}
	}
	if len(toolMsgs) != 1 {
		t.Fatalf("tool result messages = %d, want 1", len(toolMsgs))
	}
	if toolMsgs[0].ToolCallID != "call_1" {
		t.Fatalf("tool call id = %q", toolMsgs[0].ToolCallID)
	}
	if !strings.Contains(toolMsgs[0].Content, "content") {
		t.Fatalf("tool result = %q", toolMsgs[0].Content)
	}

	// the read tool executed in the session's project dir
	ump, err := h.db.ListParts(func() string {
		msgs, err := h.db.ListMessages(ses)
		if err != nil || len(msgs) < 3 {
			t.Fatalf("messages: %v, %v", msgs, err)
		}
		return msgs[1].ID
	}())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range ump {
		p, err := storage.PartToProtocol(row)
		if err != nil {
			t.Fatal(err)
		}
		if p.Type == "tool" && p.Tool == "read" && p.State != nil &&
			p.State.Status == "completed" && strings.Contains(p.State.Output, "content") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no completed read tool part: %+v", ump)
	}
}
