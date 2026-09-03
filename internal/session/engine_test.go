package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

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

func TestSingleTextTurnEndToEnd(t *testing.T) {
	h := newHarness(t)
	h.build(t)

	h.drv.Turns = []fake.Turn{{Parts: []llm.Part{
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
	if h.eng.Status(ses) != protocol.SessionStatusIdle {
		t.Fatal("status not idle")
	}
	if errMsg != nil {
		t.Fatalf("turn error: %v", errMsg)
	}

	assertTurnState(t, h, ses)

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

	assertTurnEvents(t, h)
}

func TestHistoryReplayIncludesToolResults(t *testing.T) {
	h := newHarness(t)
	h.build(t)

	d := t.TempDir()
	fp := filepath.Join(d, "f.txt")
	writeFile(t, fp, "content")

	h.drv.Turns = []fake.Turn{
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
	msgs, err := h.db.ListMessages(t.Context(), ses)
	if err != nil || len(msgs) < 3 {
		t.Fatalf("messages: %v, %v", msgs, err)
	}
	ump, err := h.db.ListParts(t.Context(), msgs[1].ID)
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
	if got := h.eng.Status(sesA); got != protocol.SessionStatusBusy {
		t.Fatalf("status(%s) = %s, want busy", sesA, got)
	}
	if got := h.eng.Status(sesB); got != protocol.SessionStatusBusy {
		t.Fatalf("status(%s) = %s, want busy", sesB, got)
	}

	h.eng.Shutdown(context.Background())

	if got := h.eng.Status(sesA); got != protocol.SessionStatusIdle {
		t.Fatalf("status(%s) = %s, want idle", sesA, got)
	}
	if got := h.eng.Status(sesB); got != protocol.SessionStatusIdle {
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
	h.drv.Turns = []fake.Turn{{Parts: []llm.Part{
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
	msgs, _ := h.db.ListMessages(t.Context(), ses)
	for _, m := range msgs {
		parts, _ := h.db.ListParts(t.Context(), m.ID)
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
	h.drv.Turns = []fake.Turn{
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
	row := storage.SessionRow{Title: "titled", TimeUpdated: time.Now().UnixMilli()}
	if err := h.db.UpdateSession(t.Context(), ses, row); err != nil {
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
	if got := h.eng.Status(ses); got != protocol.SessionStatusIdle {
		t.Fatalf("status after recovered panic = %s, want %s", got, protocol.SessionStatusIdle)
	}
	// The idle event is the last publish in the turn's defer (same closure
	// that fires onDone), so wait for the collector goroutine to fold it
	// before counting — counting right after onDone races the collector.
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeSessionStatus {
			return false
		}
		return statusType(t, e).Type == protocol.SessionStatusIdle
	})
	idles := h.eventCount(func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeSessionStatus {
			return false
		}
		return statusType(t, e).Type == protocol.SessionStatusIdle
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
	if err := h.db.DeleteSession(t.Context(), ses); err != nil {
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
	inner     *fake.Driver
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
	hd := holdTitleDriver{inner: fake.New(fake.Turn{Parts: []llm.Part{
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
	inner     *fake.Driver
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
	hd := &supersededTitleDriver{inner: fake.New(fake.AutoText()), release1: make(chan struct{})}
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
	msgs, err := h.db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range msgs {
		if m.Role == "assistant" {
			if err := h.db.DeleteMessage(t.Context(), m.ID); err != nil {
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
		return p.SessionID == ses && p.Status.Type == protocol.SessionStatusBusy
	})
	h.eng.Close(ses)
	deadline := time.Now().Add(5 * time.Second)
	for h.eng.Status(ses) == protocol.SessionStatusBusy {
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
	for h.eng.Status(ses) == protocol.SessionStatusBusy {
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

// TestWaitIdleUnknownSession: WaitIdle returns nil immediately for a
// session with no in-flight turn (unknown id or an idle session) — it
// observes the done event, never polls Status.
func TestWaitIdleUnknownSession(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	ses := h.startSession(t, t.TempDir())
	if err := h.eng.WaitIdle(t.Context(), protocol.NewID("ses")); err != nil {
		t.Fatalf("WaitIdle(unknown session) = %v, want nil", err)
	}
	if err := h.eng.WaitIdle(t.Context(), ses); err != nil {
		t.Fatalf("WaitIdle(idle session) = %v, want nil", err)
	}
}

// TestWaitIdleSettlesOnTurnEnd: WaitIdle blocks while a turn is active and
// returns nil once the turn releases the busy flag (event-driven settle
// instead of a Status poll; the slow turn holds the busy window open).
func TestWaitIdleSettlesOnTurnEnd(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	h.slowTurn = true // the slow stream holds the busy window open ~500 ms
	ses := h.startSession(t, t.TempDir())
	if _, err := h.eng.Send(t.Context(), ses, "hi", nil); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitBusy(t, h, ses)
	if err := h.eng.WaitIdle(t.Context(), ses); err != nil {
		t.Fatalf("WaitIdle = %v, want nil", err)
	}
	if st := h.eng.Status(ses); st != protocol.SessionStatusIdle {
		t.Fatalf("status after WaitIdle = %q, want idle", st)
	}
}

// TestWaitIdleContextCancel: WaitIdle returns the caller's context error
// when the context cancels before the turn settles (the slow turn holds
// the busy window open well past the 50 ms deadline).
func TestWaitIdleContextCancel(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	h.slowTurn = true // the slow stream holds the busy window open ~500 ms
	ses := h.startSession(t, t.TempDir())
	if _, err := h.eng.Send(t.Context(), ses, "hi", nil); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitBusy(t, h, ses)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond) // well inside the hold
	defer cancel()
	if err := h.eng.WaitIdle(ctx, ses); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitIdle = %v, want context.DeadlineExceeded", err)
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
	h.drv.Turns = []fake.Turn{
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
		rows, err := h.db.ListParts(t.Context(), m.ID)
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
	rows, err := db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
