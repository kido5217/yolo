package session_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	t   *testing.T
	db  *storage.DB
	bus *bus.Bus
	eng *session.Engine
	drv *fakellm.Driver
	svc *permission.Service

	// dataDir is shared by the engine Deps and the permission service (the
	// engine used to push it per turn; the service now carries it at build).
	dataDir string

	// cfgPermission feeds the Cfg seam (read per turn, so it may be set
	// after build, before Send).
	cfgPermission []protocol.Rule

	// fastBackoff makes the engine's retry backoff 1ms (read lazily per
	// retry, so it may be set after build, before Send).
	fastBackoff bool
	// slowTurn holds each scripted stream open for 500ms (fake driver
	// Delay), so a concurrent Send hits the busy flag.
	slowTurn bool
	// overrideDriver, when set before build, replaces the driver wired for
	// the test provider (bespoke stream behavior, e.g. stream-leak probes).
	overrideDriver llm.Driver

	sessions []string // created via startSession; shells closed on cleanup

	eventsMu sync.Mutex
	events   []protocol.Event

	// slowCancel counts held slow-driver streams that observed ctx
	// cancellation (asserted after Shutdown, so "abort, not wait" is
	// deterministic instead of a wall-clock margin).
	slowCancel atomic.Int32

	replies chan string
	done    chan struct{} // closed on cleanup; lets the watcher exit a pending wait
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	b := bus.New()
	ch, unsub := b.Subscribe()
	dataDir := t.TempDir()
	h := &harness{
		t: t, db: db, bus: b,
		svc:     permission.New(db, b, nil, dataDir),
		dataDir: dataDir,
		replies: make(chan string, 32),
		done:    make(chan struct{}),
	}
	askCh, unsubAsked := b.Subscribe()
	t.Cleanup(func() {
		close(h.done) // let the watcher exit a pending reply wait before it can fire its timer
		unsubAsked()
		unsub()
		if h.eng != nil {
			for _, s := range h.sessions {
				h.eng.Close(s) // release the session's bash shell
			}
		}
		db.Close()
	})
	go func() {
		for e := range ch {
			h.eventsMu.Lock()
			h.events = append(h.events, e)
			h.eventsMu.Unlock()
		}
	}()
	go h.replyWatcher(askCh)
	return h
}

// replyWatcher answers permission.asked events with the test-queued replies
// (FIFO). An ask that finds no queued reply fails the test after 3s — a
// prompt the test did not expect.
func (h *harness) replyWatcher(ch <-chan protocol.Event) {
	for e := range ch {
		if e.Type != protocol.EventTypePermissionAsked {
			continue
		}
		var p protocol.PermissionAskedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			h.t.Errorf("decode permission.asked: %v", err)
			continue
		}
		select {
		case resp := <-h.replies:
			if err := h.svc.Reply(p.ID, resp); err != nil {
				h.t.Errorf("reply %q to %s: %v", resp, p.ID, err)
			}
		case <-h.done:
			return
		case <-time.After(3 * time.Second):
			h.t.Errorf("permission.asked %s (permission %q) has no queued reply", p.ID, p.Permission)
		}
	}
}

// queueReplies queues permission replies (FIFO) consumed by replyWatcher.
func (h *harness) queueReplies(responses ...string) {
	for _, r := range responses {
		h.replies <- r
	}
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
	eng, err := session.New(session.Deps{
		DB:      h.db,
		Bus:     h.bus,
		Prov:    reg,
		Perm:    h.svc,
		Tools:   tool.Registry(),
		DataDir: h.dataDir,
		Cfg:     h.cfgLoader(),
		// slowDriver reads h.slowTurn per Stream call (the test sets it
		// after build) and slows the call by sleeping in the wrapper before
		// forwarding — not via a fake field, because the title side-call and
		// the turn call Stream concurrently and a shared field would race.
		Drivers: map[string]llm.Driver{"kido": h.wiredDriver(drv)},
		Clock:   func() int64 { return time.Now().UnixMilli() },
		Backoff: func(attempt int) time.Duration {
			if h.fastBackoff {
				return time.Millisecond
			}
			return time.Second << uint(attempt-1)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.eng = eng
}

// wiredDriver picks the driver for the test provider: the slowDriver
// wrapper, or h.overrideDriver when a test set it before build (bespoke
// stream behavior). The title side-call shares the same driver, so
// overrides must be concurrency-safe.
func (h *harness) wiredDriver(drv *fakellm.Driver) llm.Driver {
	if h.overrideDriver != nil {
		return h.overrideDriver
	}
	return slowDriver{h: h, inner: drv}
}

// slowDriver slows each Stream call (ctx-aware sleep) while h.slowTurn is
// set, so a concurrent Send observes the busy flag (the seam lags behind
// build by design). The sleep stays in the wrapper: the title side-call and
// the turn call Stream concurrently, and a shared fake field would race.
type slowDriver struct {
	h     *harness
	inner llm.Driver
}

func (s slowDriver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	if s.h.slowTurn {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			s.h.slowCancel.Add(1)
			return llm.PartStream{}, ctx.Err()
		}
	}
	return s.h.drv.Stream(ctx, req)
}

// TestNewValidatesRequiredDeps: a miswired dep is a construction error, not
// a nil panic deep in an un-recovered turn goroutine (single-binary crash).
func TestNewValidatesRequiredDeps(t *testing.T) {
	valid := func() session.Deps {
		return session.Deps{
			DB:    nil, // filled per case
			Bus:   bus.New(),
			Prov:  provider.NewStaticForTest(),
			Perm:  nil, // filled per case
			Tools: tool.Registry(),
		}
	}
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	perm := permission.New(db, bus.New(), nil, "")

	cases := []struct {
		name string
		mut  func(d *session.Deps)
	}{
		{"nil DB", func(d *session.Deps) { d.DB = nil; d.Perm = perm }},
		{"nil Bus", func(d *session.Deps) { d.DB = db; d.Bus = nil; d.Perm = perm }},
		{"nil Prov", func(d *session.Deps) { d.DB = db; d.Prov = nil; d.Perm = perm }},
		{"nil Perm", func(d *session.Deps) { d.DB = db; d.Perm = nil }},
		{"nil Tools", func(d *session.Deps) { d.DB = db; d.Perm = perm; d.Tools = nil }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := valid()
			c.mut(&d)
			if _, err := session.New(d); err == nil {
				t.Fatal("New accepted a required dep as nil")
			}
		})
	}
	t.Run("valid deps construct", func(t *testing.T) {
		d := valid()
		d.DB = db
		d.Perm = perm
		if _, err := session.New(d); err != nil {
			t.Fatalf("New(valid) = %v", err)
		}
	})
}

// cfgLoader mirrors h.cfgPermission into the config permission map. The
// closure is called per turn by the engine, so rules set after build apply.
func (h *harness) cfgLoader() func(string) (*protocol.Config, error) {
	return func(string) (*protocol.Config, error) {
		cfg := &protocol.Config{}
		if len(h.cfgPermission) > 0 {
			m := map[string]any{}
			for _, r := range h.cfgPermission {
				inner, ok := m[r.Permission].(map[string]any)
				if !ok {
					inner = map[string]any{}
					m[r.Permission] = inner
				}
				inner[r.Pattern] = r.Action
			}
			cfg.Permission = m
		}
		return cfg, nil
	}
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
	h.sessions = append(h.sessions, id)
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

// waitBusy polls Status until the session reports busy, so 409/abort tests
// act inside a deterministic busy window instead of a fixed sleep.
func waitBusy(t *testing.T, h *harness, ses string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for h.eng.Status(ses) != protocol.StatusBusy {
		if time.Now().After(deadline) {
			t.Fatal("turn did not become busy")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitPart polls the DB until a part of kind (tool parts: in state status)
// exists in the session; timeout -> fail.
func waitPart(t *testing.T, h *harness, ses, kind, status string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		found := false
		msgs, err := h.db.ListMessages(ses)
		if err == nil {
			for _, m := range msgs {
				var rows []storage.PartRow
				if kind == "tool" {
					rows, _ = h.db.ListToolParts(m.ID)
				} else {
					rows, _ = h.db.ListParts(m.ID)
				}
				for _, r := range rows {
					p, err := storage.PartToProtocol(r)
					if err != nil {
						continue
					}
					if p.Type != kind {
						continue
					}
					if (kind == "tool" && p.State != nil && p.State.Status == status) ||
						(kind != "tool" && status == "") {
						found = true
					}
				}
			}
		}
		if found {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s part with status %q within %v", kind, status, timeout)
		}
		time.Sleep(5 * time.Millisecond)
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

func countRole(msgs []llm.Message, role llm.Role) int {
	n := 0
	for _, m := range msgs {
		if m.Role == role {
			n++
		}
	}
	return n
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
	if len(roles) < 3 {
		t.Fatalf("roles = %v", roles)
	}
	// Upstream-faithful (message-v2.toModelMessagesEffect): the request
	// mirrors the persisted history 1:1 — in a tool round it ends with the
	// TOOL result, never with a re-appended copy of the user message
	// (deviation 77: the re-append made the model see its instruction
	// re-issued every round and re-run tools in a loop).
	tail := roles[len(roles)-3:]
	want := []string{string(llm.RoleUser), string(llm.RoleAssistant), string(llm.RoleTool)}
	for i := range want {
		if tail[i] != want[i] {
			t.Fatalf("role tail = %v, want %v", tail, want)
		}
	}
	if n := countRole(req.Messages, llm.RoleUser); n != 1 {
		t.Fatalf("user messages in round-2 request = %d, want 1 (no re-append)", n)
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
	msgs, err := h.db.ListMessages(ses)
	if err != nil || len(msgs) < 3 {
		t.Fatalf("messages: %v, %v", msgs, err)
	}
	ump, err := h.db.ListParts(msgs[1].ID)
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

// TestShutdownAbortsActiveAndWaits: two sessions with active turns (the
// slowTurn seam holds each stream open 500 ms); Shutdown aborts all of
// them — each held stream observes its context cancelled, so "abort, not
// wait" is asserted deterministically, not on a wall-clock margin — and
// returns once every turn has released.
func TestShutdownAbortsActiveAndWaits(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	sesA := h.startSession(t, t.TempDir())
	sesB := h.startSession(t, t.TempDir())
	h.slowTurn = true
	if _, err := h.eng.Send(context.Background(), sesA, "hi", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.eng.Send(context.Background(), sesB, "hi", nil); err != nil {
		t.Fatal(err)
	}
	if got := h.eng.Status(sesA); got != protocol.StatusBusy {
		t.Fatalf("status(%s) = %s, want busy", sesA, got)
	}
	if got := h.eng.Status(sesB); got != protocol.StatusBusy {
		t.Fatalf("status(%s) = %s, want busy", sesB, got)
	}

	h.eng.Shutdown(context.Background())

	if got := h.eng.Status(sesA); got != protocol.StatusIdle {
		t.Fatalf("status(%s) = %s, want idle", sesA, got)
	}
	if got := h.eng.Status(sesB); got != protocol.StatusIdle {
		t.Fatalf("status(%s) = %s, want idle", sesB, got)
	}
	// a stream only takes the ctx.Done() branch when its context is
	// cancelled; if Shutdown had waited out the 500 ms holds, no held
	// stream would have observed a cancellation.
	if got := h.slowCancel.Load(); got < 2 {
		t.Fatalf("held streams observing ctx cancellation = %d, want >= 2 (one per session)", got)
	}
}

// TestTextDeltasEmitSSEAndPersistAtFinalize pins ⑩'s contract: a multi-delta
// text stream emits one message.part.delta per delta (wire unchanged) and the
// persisted part carries the full accumulated text. Per-delta DB writes are
// removed; only finalization persists (verified by the wave-13 benchmark).
func TestTextDeltasEmitSSEAndPersistAtFinalize(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	h.drv.Turns = []fakellm.Turn{{Parts: []llm.Part{
		{Kind: "text", Text: "He"},
		{Kind: "text", Text: "llo"},
		{Kind: "text", Text: " world", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 3}},
	}}}
	ses := h.startSession(t, t.TempDir())
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "hi", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})
	if n := h.eventCount(func(e protocol.Event) bool { return e.Type == protocol.EventTypeMessagePartDelta }); n != 3 {
		t.Fatalf("message.part.delta events = %d, want 3", n)
	}
	found := false
	msgs, _ := h.db.ListMessages(ses)
	for _, m := range msgs {
		parts, _ := h.db.ListParts(m.ID)
		for _, pr := range parts {
			p, _ := storage.PartToProtocol(pr)
			if p.Type == "text" && p.Text == "Hello world" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("persisted text part \"Hello world\" not found (finalization must persist the full text)")
	}
}

// TestHistorySnapshotAccumulatesAcrossRounds pins ⑪: over a multi-round tool
// turn, each round's model request carries the full accumulated history from
// the turn's in-memory snapshot — identical to a per-round DB replay (the
// history->llm mapping is untouched).
func TestHistorySnapshotAccumulatesAcrossRounds(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	ses := h.startSession(t, t.TempDir())
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{{Kind: "tool", Name: "glob", CallID: "g1", Text: `{"pattern":"x*"}`, Finish: "tool_calls"}}},
		{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "find x", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})
	reqs := nonTitle(h.drv.Requests())
	if len(reqs) != 2 {
		t.Fatalf("model rounds = %d, want 2", len(reqs))
	}
	if n := countRole(reqs[0].Messages, llm.RoleAssistant); n != 0 {
		t.Fatalf("round1 assistant count = %d, want 0", n)
	}
	r2 := reqs[1].Messages
	if n := countRole(r2, llm.RoleAssistant); n != 1 {
		t.Fatalf("round2 assistant count = %d, want 1", n)
	}
	if n := countRole(r2, llm.RoleTool); n != 1 {
		t.Fatalf("round2 tool count = %d, want 1 (round1 result must be in history)", n)
	}
	var callID string
	for _, m := range r2 {
		if m.Role == llm.RoleAssistant && len(m.ToolCalls) > 0 {
			callID = m.ToolCalls[0].ID
		}
	}
	if callID != "g1" {
		t.Fatalf("round2 assistant tool call id = %q, want g1", callID)
	}
}

// panicDriver panics on its first non-title Stream call (the tool/driver
// panic probe). Title requests get a short plain stream — the test
// pre-titles the session so no title side-call actually races the probe.
type panicDriver struct{ fired atomic.Bool }

func (p *panicDriver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	if isTitleReq(req) {
		ch := make(chan llm.Part, 1)
		ch <- llm.Part{Kind: "text", Text: "t", Finish: "stop"}
		close(ch)
		return llm.PartStream{Parts: ch}, nil
	}
	p.fired.Store(true)
	panic("engine panic probe")
}

// TestRunTurnRecoversPanic: a tool/driver panic escapes no goroutine — the
// turn ends failed (idle status + onDone(err)), the process lives. Without
// the recover this test's binary crashes (unrecovered panic in the turn
// goroutine).
func TestRunTurnRecoversPanic(t *testing.T) {
	h := newHarness(t)
	pd := panicDriver{}
	h.overrideDriver = &pd
	h.build(t)
	ses := h.startSession(t, t.TempDir())
	if err := h.db.UpdateSession(ses, storage.SessionRow{Title: "titled", TimeUpdated: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	var doneErr error
	done := make(chan struct{})
	if _, err := h.eng.Send(t.Context(), ses, "hi", func(err error) {
		doneErr = err
		close(done)
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("onDone never fired after a turn panic")
	}
	if !pd.fired.Load() {
		t.Fatal("probe driver was never called; test is vacuous")
	}
	if doneErr == nil {
		t.Fatal("turn panic reported success (onDone(nil))")
	}
	if got := h.eng.Status(ses); got != protocol.StatusIdle {
		t.Fatalf("status after recovered panic = %s, want %s", got, protocol.StatusIdle)
	}
	// The idle event is the last publish in the turn's defer (same closure
	// that fires onDone), so wait for the collector goroutine to fold it
	// before counting — counting right after onDone races the collector.
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeSessionStatus {
			return false
		}
		return statusType(t, e).Type == protocol.StatusIdle
	})
	idles := h.eventCount(func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeSessionStatus {
			return false
		}
		return statusType(t, e).Type == protocol.StatusIdle
	})
	if idles != 1 {
		t.Fatalf("idle session.status events = %d, want 1", idles)
	}
}

// TestSendEarlyFailPublishesNoStatus: a Send failure before the turn
// goroutine starts publishes NO session.status at all (spec §3.1 B: skip
// both) — no lone idle for a busy no client ever observed. The error is the
// return value; onDone never fires (the turn goroutine never ran).
func TestSendEarlyFailPublishesNoStatus(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	ses := h.startSession(t, t.TempDir())
	if err := h.db.DeleteSession(ses); err != nil {
		t.Fatal(err)
	}
	onDone := make(chan struct{})
	if _, err := h.eng.Send(t.Context(), ses, "hi", func(error) { close(onDone) }); err == nil {
		t.Fatal("Send on a deleted session succeeded")
	}
	select {
	case <-onDone:
		t.Fatal("onDone fired although the turn goroutine never started")
	case <-time.After(100 * time.Millisecond):
	}
	if n := h.eventCount(func(e protocol.Event) bool { return e.Type == protocol.EventTypeSessionStatus }); n != 0 {
		t.Fatalf("early-fail Send published %d session.status events, want 0 (no lone idle)", n)
	}
}

// holdTitleDriver holds title streams until the request ctx is cancelled
// (counted) and forwards turn requests to a plain fake — the title
// goroutine is in flight for the whole test window.
type holdTitleDriver struct {
	cancelled atomic.Bool
	inner     *fakellm.Driver
}

func (d *holdTitleDriver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	if isTitleReq(req) {
		<-ctx.Done()
		d.cancelled.Store(true)
		return llm.PartStream{}, ctx.Err()
	}
	return d.inner.Stream(ctx, req)
}

func titleProbeHarness(t *testing.T) (*harness, *holdTitleDriver) {
	h := newHarness(t)
	hd := holdTitleDriver{inner: fakellm.New(fakellm.Turn{Parts: []llm.Part{
		{Kind: "text", Text: "hi", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}},
	}})}
	h.overrideDriver = &hd
	h.build(t)
	return h, &hd
}

// TestAbortCancelsTitleGoroutine: Abort cancels the in-flight title side-call
// (30 s background ctx is gone — the goroutine ends on the abort, not the
// timeout).
func TestAbortCancelsTitleGoroutine(t *testing.T) {
	h, hd := titleProbeHarness(t)
	ses := h.startSession(t, t.TempDir())
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "hi", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})
	h.eng.Abort(ses)
	deadline := time.Now().Add(2 * time.Second)
	for !hd.cancelled.Load() {
		if time.Now().After(deadline) {
			t.Fatal("Abort did not cancel the in-flight title goroutine")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestShutdownCancelsAndWaitsTitle: Shutdown cancels an in-flight title
// goroutine and waits for it (bounded) before returning — no UpdateSession /
// session.updated after the store closes (design-3 fold-in).
func TestShutdownCancelsAndWaitsTitle(t *testing.T) {
	h, hd := titleProbeHarness(t)
	ses := h.startSession(t, t.TempDir())
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "hi", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})
	done := make(chan struct{})
	go func() {
		h.eng.Shutdown(t.Context())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return while a title goroutine was held")
	}
	deadline := time.Now().Add(2 * time.Second)
	for !hd.cancelled.Load() {
		if time.Now().After(deadline) {
			t.Fatal("Shutdown did not cancel the in-flight title goroutine")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// supersededTitleDriver: the first title stream is held until released,
// then completes with a text part (the resulting title write is the
// deterministic signal that the first title goroutine's body finished);
// the second title stream is held until its ctx is cancelled (counted).
// Turn requests forward to an auto-text fake.
type supersededTitleDriver struct {
	inner     *fakellm.Driver
	titleSeq  atomic.Int32
	release1  chan struct{}
	cancelled atomic.Bool
}

func (d *supersededTitleDriver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	if !isTitleReq(req) {
		return d.inner.Stream(ctx, req)
	}
	switch d.titleSeq.Add(1) {
	case 1:
		<-d.release1
		ch := make(chan llm.Part, 1)
		ch <- llm.Part{Kind: "text", Text: "t1-done", Finish: "stop"}
		close(ch)
		return llm.PartStream{Parts: ch}, nil
	default:
		<-ctx.Done()
		d.cancelled.Store(true)
		return llm.PartStream{}, ctx.Err()
	}
}

// TestSupersededTitleDropKeepsNewerCancel: a retry that schedules a second
// title while the first is still in flight replaces the tracked cancel;
// when the first title then exits, its dropTitleCtx must NOT drop the
// newer cancel — Abort must still cancel the second title (regression pin
// for the unconditional-drop review finding).
func TestSupersededTitleDropKeepsNewerCancel(t *testing.T) {
	h := newHarness(t)
	hd := &supersededTitleDriver{inner: fakellm.New(fakellm.AutoText()), release1: make(chan struct{})}
	h.overrideDriver = hd
	h.build(t)
	ses := h.startSession(t, t.TempDir())

	// Turn A schedules title T1 (held by the driver) and completes.
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "hi", nil); err != nil {
			t.Fatalf("Send A: %v", err)
		}
	})

	// Construct the defect precondition for Send B: the turn ended with no
	// assistant message (the round's row is deleted), the title is still
	// the default — so maybeScheduleTitle fires a SECOND title (T2) whose
	// cancel replaces T1's in the tracked map.
	msgs, err := h.db.ListMessages(ses)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range msgs {
		if m.Role == "assistant" {
			if err := h.db.DeleteMessage(m.ID); err != nil {
				t.Fatal(err)
			}
		}
	}

	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "hi", nil); err != nil {
			t.Fatalf("Send B: %v", err)
		}
	})

	// Release T1: its stream completes, the title is written (the
	// session.updated event below), and the goroutine exits — its
	// dropTitleCtx defer runs while T2's cancel is the tracked entry.
	close(hd.release1)
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeSessionUpdated {
			return false
		}
		var p protocol.SessionUpdatedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			t.Fatal(err)
		}
		return p.SessionID == ses && p.Info.Title == "t1-done"
	})
	// The session.updated publish is the LAST body statement of T1's
	// generateTitle; the dropTitleCtx defer runs immediately after the
	// return. Let the defer land so the Abort below observes the
	// post-drop state (the unconditional-drop bug only manifests then).
	time.Sleep(50 * time.Millisecond)

	h.eng.Abort(ses)
	deadline := time.Now().Add(2 * time.Second)
	for !hd.cancelled.Load() {
		if time.Now().After(deadline) {
			t.Fatal("Abort did not cancel the second (newer) title after the first title's exit dropped the tracked cancel")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestCloseWhileBusyAbortsAndSuppresses: Close on a session with an in-flight
// turn aborts the turn (the held slow stream observes the ctx cancel),
// suppresses the post-Delete event stream (busy-only statuses survive), and
// releases the session. The slowTurn hold makes the busy window
// deterministic.
func TestCloseWhileBusyAbortsAndSuppresses(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	h.slowTurn = true
	ses := h.startSession(t, t.TempDir())
	if _, err := h.eng.Send(t.Context(), ses, "hi", func(error) {}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitBusy(t, h, ses)
	// The busy event (not just the flag) must be published before Close:
	// the flag is set synchronously in Send, but runTurn publishes the busy
	// status from its goroutine — without this, the suppressed-count
	// assertion below races the publish (could read 0).
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeSessionStatus {
			return false
		}
		var p protocol.SessionStatusProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			t.Fatal(err)
		}
		return p.SessionID == ses && p.Status.Type == protocol.StatusBusy
	})
	h.eng.Close(ses)
	deadline := time.Now().Add(5 * time.Second)
	for h.eng.Status(ses) == protocol.StatusBusy {
		if time.Now().After(deadline) {
			t.Fatal("turn still busy after Close (abort not applied)")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if h.slowCancel.Load() == 0 {
		t.Fatal("Close did not abort the in-flight turn (held stream never saw ctx cancel)")
	}
	// Exactly the busy status survives; the post-Delete idle is suppressed.
	deadline = time.Now().Add(2 * time.Second)
	for {
		n := h.eventCount(func(e protocol.Event) bool {
			if e.Type != protocol.EventTypeSessionStatus {
				return false
			}
			var p protocol.SessionStatusProps
			if json.Unmarshal(e.Properties, &p) != nil || p.SessionID != ses {
				return false
			}
			return true
		})
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("session.status events for closed session = %d, want 1 (busy only; the post-Delete idle is suppressed)", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestAbortThenNewTurnCompletes: an Abort followed by a fresh Send must not
// cancel the fresh turn (the TOCTOU: turn 1's stale cancel invoked after
// turn 2 took the busy slot). slowTurn holds turn 1 so the abort lands
// inside a deterministic busy window.
func TestAbortThenNewTurnCompletes(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	h.slowTurn = true
	ses := h.startSession(t, t.TempDir())
	if _, err := h.eng.Send(t.Context(), ses, "hi", func(error) {}); err != nil {
		t.Fatalf("Send 1: %v", err)
	}
	waitBusy(t, h, ses)
	if !h.eng.Abort(ses) {
		t.Fatal("Abort reported no active turn")
	}
	deadline := time.Now().Add(5 * time.Second)
	for h.eng.Status(ses) == protocol.StatusBusy {
		if time.Now().After(deadline) {
			t.Fatal("turn 1 did not settle after Abort")
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.slowTurn = false
	var turn2Err error
	done := make(chan struct{})
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "again", func(err error) {
			turn2Err = err
			close(done)
		}); err != nil {
			t.Fatalf("Send 2: %v", err)
		}
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("turn 2 onDone never fired")
	}
	if turn2Err != nil {
		t.Fatalf("turn 2 failed after a prior Abort (stale cancel): %v", turn2Err)
	}
}

// TestToolRoundMintsFreshTextPart: a tool round whose stream continues with
// text after the tool call starts a NEW text part (fresh id, upstream
// parity) instead of appending to the finalized pre-tool part — and each
// part is finalized exactly once (no re-finalization frames). The engine
// mints a fresh assistant message per round and the tool round continues to
// a synthesized round-2 message, so the persisted assertion collects across
// all assistant messages (the "before"/"after" parts are round 1's).
func TestToolRoundMintsFreshTextPart(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	fp := filepath.Join(d, "f.txt")
	writeFile(t, fp, "content")
	h.drv.Turns = []fakellm.Turn{
		{Parts: []llm.Part{
			{Kind: "text", Text: "before"},
			{Kind: "tool", Name: "read", CallID: "call_1", Text: fmt.Sprintf(`{"filePath":%q}`, fp)},
			{Kind: "text", Text: "after", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 2}},
		}},
	}
	ses := h.startSession(t, d)
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "read f.txt", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	// Persisted: the session's assistant messages carry text parts "before"
	// and "after" with distinct ids (pre-fix: one merged "beforeafter"
	// part).
	byText := map[string]string{}
	for _, m := range mustListMessages(t, h.db, ses) {
		if m.Role != "assistant" {
			continue
		}
		rows, err := h.db.ListParts(m.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			p, err := storage.PartToProtocol(r)
			if err != nil {
				t.Fatalf("PartToProtocol: %v", err)
			}
			if p.Type == "text" {
				byText[p.Text] = p.ID
			}
		}
	}
	for _, want := range []string{"before", "after"} {
		if byText[want] == "" {
			t.Fatalf("text part %q not persisted (got %v)", want, byText)
		}
	}
	if byText["before"] == byText["after"] {
		t.Fatal("pre-tool and post-tool text parts share an id")
	}

	// Wire: no part id gets more than start + final part.updated frames
	// (pre-fix the pre-tool id gets a third, re-finalization frame).
	frames := map[string]int{}
	h.eventsMu.Lock()
	for _, e := range h.events {
		if e.Type != protocol.EventTypeMessagePartUpdated {
			continue
		}
		var p protocol.MessagePartUpdatedProps
		if json.Unmarshal(e.Properties, &p) != nil || p.SessionID != ses {
			continue
		}
		frames[p.Part.ID]++
	}
	h.eventsMu.Unlock()
	for id, n := range frames {
		if n > 2 {
			t.Fatalf("part %s published %d part.updated frames, want ≤ 2 (no re-finalization)", id, n)
		}
	}
}

// mustListMessages is the test-local ListMessages wrapper (fatal on error).
func mustListMessages(t *testing.T, db *storage.DB, ses string) []storage.MessageRow {
	t.Helper()
	rows, err := db.ListMessages(ses)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
