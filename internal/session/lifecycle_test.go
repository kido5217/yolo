package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
)

func TestTransientRetrySucceeds(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fake.Turn{
		{Err: &llm.TransientError{Status: 429, Err: errors.New("slow down")}},
		{Err: &llm.TransientError{Status: 503, Err: errors.New("unavailable")}},
		{Parts: []llm.Part{{Kind: "text", Text: "ok", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	h.fastBackoff = true // harness seam: Deps.Backoff func(attempt int) time.Duration → 1ms (test only)
	if _, err := h.eng.Send(context.Background(), ses, "hi", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	// Two retry status events. Wait for the idle status event first: bus
	// delivery is channel-based and order-preserving, so once the turn's
	// final idle event is buffered, every earlier event is too.
	h.waitForEvent(t, func(e protocol.Event) bool {
		return e.Type == protocol.EventTypeSessionStatus && statusType(t, e).Type == protocol.SessionStatusIdle
	})
	retries := h.eventCount(func(e protocol.Event) bool {
		return e.Type == protocol.EventTypeSessionStatus && statusType(t, e).Type == protocol.SessionStatusRetry
	})
	if retries != 2 {
		t.Fatalf("retry events = %d", retries)
	}
	msgs, _ := h.db.ListMessages(t.Context(), ses)
	if len(msgs) != 2 || msgs[1].Role != "assistant" {
		t.Fatalf("turn lost data: %d", len(msgs))
	}
	if got := len(nonTitle(h.drv.Requests())); got != 3 {
		t.Fatalf("attempts = %d", got)
	}
}

func TestTransientGivesUpAfter4(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	for i := 0; i < 4; i++ {
		h.drv.Turns = append(h.drv.Turns, fake.Turn{Err: &llm.TransientError{Status: 500, Err: errors.New("boom")}})
	}
	h.fastBackoff = true
	done := make(chan struct{})
	var doneErr error
	if _, err := h.eng.Send(context.Background(), ses, "hi", func(e error) {
		doneErr = e
		close(done)
	}); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onDone not called")
	}
	if got := len(nonTitle(h.drv.Requests())); got != 4 {
		t.Fatalf("attempts = %d, want 4", got)
	}
	// turn ended idle; assistant message exists (may be empty)
	msgs, _ := h.db.ListMessages(t.Context(), ses)
	if len(msgs) < 2 {
		t.Fatalf("messages = %d", len(msgs))
	}
	// give up surfaces the last transient error to the caller
	if doneErr == nil {
		t.Fatal("expected onDone(err) after retry exhaustion")
	}
}

func TestMidStreamErrorNoRetry(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{
			{Kind: "text", Text: "partial"},
			{Kind: "text", Finish: "error", Err: errors.New("connection reset")},
		}},
	}
	done := make(chan struct{})
	var doneErr error
	if _, err := h.eng.Send(context.Background(), ses, "hi", func(e error) {
		doneErr = e
		close(done)
	}); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onDone not called")
	}
	// mid-stream failure fails the turn (no retry)
	if doneErr == nil {
		t.Fatal("expected onDone(err) on mid-stream error")
	}
	if got := len(nonTitle(h.drv.Requests())); got != 1 {
		t.Fatalf("retried mid-stream: %d", got)
	}
}

func TestAbortMidTurn(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{
			{Kind: "text", Text: "working"},
			{Kind: "tool", Name: "bash", CallID: "t1", Text: `{"command":"sleep 10"}`, Finish: "tool_calls"},
		}},
	}
	if _, err := h.eng.Send(context.Background(), ses, "slow", nil); err != nil {
		t.Fatal(err)
	}
	// wait for the tool part to go running, then abort
	waitPart(t, h, ses, "tool", "running", 3*time.Second)
	if !h.eng.Abort(ses) {
		t.Fatal("abort rejected")
	}
	waitIdle(t, h, ses, func() {})
	msgs, _ := h.db.ListMessages(t.Context(), ses)
	var state *protocol.ToolState
	for _, m := range msgs {
		parts, _ := h.db.ListToolParts(t.Context(), m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if pt.Tool != "" {
				state = pt.State
			}
		}
	}
	if state == nil || state.Status != "error" || !strings.Contains(state.Error, "aborted") {
		t.Fatalf("state = %+v", state)
	}
}

func TestMaxToolStepsHalts(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	// 51 tool calls in one stream, then a final text — engine must stop at 50.
	// Patterns are unique per call: identical args would trip the doom-loop
	// gate (Task 17) on the 3rd call and park an ask nobody answers.
	parts := make([]llm.Part, 0, 52)
	for i := 0; i < 51; i++ {
		parts = append(parts, llm.Part{Kind: "tool", Name: "glob", CallID: string(rune('a'+i%26)) + strconv.Itoa(i), Text: fmt.Sprintf(`{"pattern":"p%d*"}`, i), Finish: "tool_calls"})
	}
	parts = append(parts, llm.Part{Kind: "text", Text: "end", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}})
	h.drv.Turns = []fake.Turn{{Parts: parts}, {Parts: []llm.Part{{Kind: "text", Text: "x", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}}}
	if _, err := h.eng.Send(context.Background(), ses, "spin", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	var toolParts int
	msgs, _ := h.db.ListMessages(t.Context(), ses)
	for _, m := range msgs {
		partsDB, _ := h.db.ListToolParts(t.Context(), m.ID)
		toolParts += len(partsDB)
	}
	if toolParts != 50 {
		t.Fatalf("tool parts = %d, want 50", toolParts)
	}
}

// leakDriver mimics the real drivers' stream goroutine (openai.go /
// anthropic.go): parts are pushed into a 64-slot channel and the feeder
// blocks on a full channel until ctx is cancelled.
type leakDriver struct {
	feederDone chan struct{} // closed when the feeder goroutine exits
}

func (d *leakDriver) Stream(ctx context.Context, req llm.Request) (llm.PartStream, error) {
	ch := make(chan llm.Part, 64)
	go func() {
		defer close(d.feederDone)
		defer close(ch)
		// 200 tool parts: the engine reads 51 (budget drop on the 51st),
		// and 51 + the 64-slot buffer still leaves 85 sends with no room —
		// the feeder blocks on send unless the ctx is cancelled.
		for i := 0; i < 200; i++ {
			select {
			case ch <- llm.Part{
				Kind:   "tool",
				Name:   "glob",
				CallID: "lk" + strconv.Itoa(i),
				Args:   json.RawMessage(fmt.Sprintf(`{"pattern":"lk%d*"}`, i)),
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return llm.PartStream{Parts: ch}, nil
}

// TestBudgetDropCancelsStreamContext pins the per-round context: when the
// tool-step budget drop abandons a live stream, the round ctx must be
// cancelled so the driver's feeder (blocked pushing undelivered parts)
// unblocks instead of leaking a goroutine plus connection until shutdown.
func TestBudgetDropCancelsStreamContext(t *testing.T) {
	h := newHarness(t)
	done := make(chan struct{})
	h.overrideDriver = &leakDriver{feederDone: done}
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	// Suppress the title side-call so exactly one Stream happens.
	if err := h.db.UpdateSession(t.Context(), ses, storage.SessionRow{Title: "leak"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.eng.Send(context.Background(), ses, "spin", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream feeder still blocked after the budget drop: per-round context not cancelled")
	}
}

func TestOverflowHardStop(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	// model Context is 100000 (harness seam) — make usage.Input 100001
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{{Kind: "text", Text: "big", Finish: "stop", Usage: &llm.Usage{Input: 100001, Output: 5}}}},
	}
	if _, err := h.eng.Send(context.Background(), ses, "big", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	msgs, _ := h.db.ListMessages(t.Context(), ses)
	// second turn attempt is NOT made (only 1 request logged)
	if got := len(nonTitle(h.drv.Requests())); got != 1 {
		t.Fatalf("requests = %d", got)
	}
	// synthetic overflow part present on the assistant message
	var found bool
	for _, m := range msgs {
		parts, _ := h.db.ListParts(t.Context(), m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if pt.Text != "" && strings.Contains(pt.Text, "context overflow") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("overflow part missing")
	}
}

func TestConcurrentSend409(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fake.Turn{{Parts: []llm.Part{{Kind: "text", Text: "slow", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}}}
	h.slowTurn = true // harness seam: hold the turn 500ms via fake driver delay
	_, err := h.eng.Send(context.Background(), ses, "one", nil)
	if err != nil {
		t.Fatal(err)
	}
	waitBusy(t, h, ses)
	_, err2 := h.eng.Send(context.Background(), ses, "two", nil)
	if !errors.Is(err2, session.ErrSessionBusy) {
		t.Fatalf("want ErrSessionBusy, got %v", err2)
	}
	waitIdle(t, h, ses, func() {})
	if _, err3 := h.eng.Send(context.Background(), ses, "three", nil); err3 != nil {
		t.Fatalf("after idle send failed: %v", err3)
	}
	waitIdle(t, h, ses, func() {})
}

// TestOverflow400FromDriverEndsIdleWithNote pins the revived graceful path:
// a provider 400 whose DECODED message matches the classifier ends the turn
// idle with the synthetic overflow note (no turn error).
func TestOverflow400FromDriverEndsIdleWithNote(t *testing.T) {
	h := newHarness(t)
	// overrideDriver must be set BEFORE build (the harness captures the
	// driver at build time).
	h.overrideDriver = errDriver{err: errors.New("upstream error (http 400): prompt is too long: 120000 > 100000")}
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	if err := h.db.UpdateSession(t.Context(), ses, storage.SessionRow{Title: "no-title"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.eng.Send(context.Background(), ses, "big", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	var found bool
	msgs, _ := h.db.ListMessages(t.Context(), ses)
	for _, m := range msgs {
		parts, _ := h.db.ListParts(t.Context(), m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if strings.Contains(pt.Text, "context overflow") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("synthetic overflow note missing")
	}
}

// TestNonOverflowAPIErrorFailsTurn pins that 401/403/429-class errors surface
// as REAL errors: the turn ends errored (not idle-with-note), the decoded
// provider text is on the synthetic note, and no overflow note exists.
func TestNonOverflowAPIErrorFailsTurn(t *testing.T) {
	h := newHarness(t)
	// overrideDriver must be set BEFORE build (the harness captures the
	// driver at build time).
	h.overrideDriver = errDriver{err: errors.New("upstream error (http 401): Invalid API key provided")}
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	if err := h.db.UpdateSession(t.Context(), ses, storage.SessionRow{Title: "no-title"}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var turnErr error
	if _, err := h.eng.Send(context.Background(), ses, "hi", func(e error) {
		turnErr = e
		close(done)
	}); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onDone not called")
	}
	if turnErr == nil {
		t.Fatal("turn ended without error")
	}
	msgs, _ := h.db.ListMessages(t.Context(), ses)
	overflow, realNote := false, false
	for _, m := range msgs {
		parts, _ := h.db.ListParts(t.Context(), m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if strings.Contains(pt.Text, "context overflow") {
				overflow = true
			}
			if strings.Contains(pt.Text, "Invalid API key provided") {
				realNote = true
			}
		}
	}
	if overflow {
		t.Fatal("401 misclassified as overflow")
	}
	if !realNote {
		t.Fatal("decoded provider message missing from the note")
	}
}

// errDriver fails every Stream call with a fixed error (pre-stream path).
type errDriver struct{ err error }

func (e errDriver) Stream(context.Context, llm.Request) (llm.PartStream, error) {
	return llm.PartStream{}, e.err
}
