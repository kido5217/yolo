package session_test

import (
	"context"
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

// TestShutdownAbortsActiveAndWaits: two sessions with active turns (the
// slowTurn seam holds each stream open 500 ms); Shutdown aborts all of
// them and returns once every turn has released — well under the 500 ms
// hold, i.e. it aborts rather than waits.
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

	start := time.Now()
	h.eng.Shutdown(context.Background())
	elapsed := time.Since(start)

	if got := h.eng.Status(sesA); got != protocol.StatusIdle {
		t.Fatalf("status(%s) = %s, want idle", sesA, got)
	}
	if got := h.eng.Status(sesB); got != protocol.StatusIdle {
		t.Fatalf("status(%s) = %s, want idle", sesB, got)
	}
	if elapsed > 400*time.Millisecond {
		t.Fatalf("Shutdown took %s; slowTurn holds each stream 500 ms, so it should abort, not wait", elapsed)
	}
}
