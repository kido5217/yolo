package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

// countErrorEvents counts the bus message.updated events carrying a
// non-nil Info.Error.
func (h *harness) countErrorEvents() int {
	h.eventsMu.Lock()
	defer h.eventsMu.Unlock()
	n := 0
	for _, e := range h.events {
		if e.Type != protocol.EventTypeMessageUpdated {
			continue
		}
		var p protocol.MessageUpdatedProps
		if json.Unmarshal(e.Properties, &p) != nil {
			continue
		}
		if p.Info.Error != nil {
			n++
		}
	}
	return n
}

// assertNoErrorSurface pins a non-failure turn: no message.updated with
// Info.Error on the bus and no row.Error in storage.
func assertNoErrorSurface(t *testing.T, h *harness, ses string) {
	t.Helper()
	h.waitForEvent(t, func(e protocol.Event) bool {
		return e.Type == protocol.EventTypeSessionStatus && statusType(t, e).Type == protocol.SessionStatusIdle
	})
	if n := h.countErrorEvents(); n != 0 {
		t.Fatalf("error events = %d, want 0", n)
	}
	msgs, err := h.db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range msgs {
		if m.Error != nil {
			t.Fatalf("row %s Error = %+v, want nil", m.ID, m.Error)
		}
	}
}

// TestTurnFailureSurfacesMessageError: a failed turn (pre-stream driver
// error, non-transient so no retry) persists the MessageError on the
// round's assistant message and re-publishes the FULL message as
// message.updated with Info.Error BEFORE the idle status — the ordering the
// TUI's one-bell contract needs (the errored flag suppresses the done-bell).
func TestTurnFailureSurfacesMessageError(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fake.Turn{{Err: errors.New("boom")}}
	done := make(chan error, 1)
	if _, err := h.eng.Send(context.Background(), ses, "hi", func(e error) { done <- e }); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("turn ended without error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onDone not called")
	}
	msgs, err := h.db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[1].Role != "assistant" {
		t.Fatalf("messages = %d", len(msgs))
	}
	if msgs[1].Error == nil || msgs[1].Error.Type != "unknown" || msgs[1].Error.Message != "boom" {
		t.Fatalf("row Error = %+v, want unknown/boom", msgs[1].Error)
	}
	// The terminal idle event is buffered, so every earlier event is too
	// (bus delivery is channel-based and order-preserving).
	h.waitForEvent(t, func(e protocol.Event) bool {
		return e.Type == protocol.EventTypeSessionStatus && statusType(t, e).Type == protocol.SessionStatusIdle
	})
	h.eventsMu.Lock()
	errIdx, idleIdx := -1, -1
	for i, e := range h.events {
		switch e.Type {
		case protocol.EventTypeMessageUpdated:
			var p protocol.MessageUpdatedProps
			if json.Unmarshal(e.Properties, &p) != nil {
				continue
			}
			if p.Info.Error != nil && errIdx == -1 {
				errIdx = i
			}
		case protocol.EventTypeSessionStatus:
			var p protocol.SessionStatusProps
			if json.Unmarshal(e.Properties, &p) != nil {
				continue
			}
			if p.Status.Type == protocol.SessionStatusIdle {
				idleIdx = i
			}
		}
	}
	var evErr error
	if errIdx != -1 {
		evErr = json.Unmarshal(h.events[errIdx].Properties, &struct {
			Info protocol.Message `json:"info"`
		}{})
	}
	h.eventsMu.Unlock()
	if errIdx == -1 {
		t.Fatal("no message.updated with Info.Error on the bus")
	}
	if idleIdx == -1 || idleIdx < errIdx {
		t.Fatalf("order = error@%d idle@%d, want the error before the idle", errIdx, idleIdx)
	}
	if evErr != nil {
		t.Fatal(evErr)
	}
	// the re-publish is the FULL message (the TUI upsert replaces Info)
	h.eventsMu.Lock()
	var p protocol.MessageUpdatedProps
	_ = json.Unmarshal(h.events[errIdx].Properties, &p)
	h.eventsMu.Unlock()
	if p.SessionID != ses || p.Info.ID != msgs[1].ID || p.Info.Role != "assistant" ||
		p.Info.Agent != "build" || p.Info.Time.Created == 0 ||
		p.Info.Error == nil || p.Info.Error.Type != "unknown" || p.Info.Error.Message != "boom" {
		t.Fatalf("error publish = %+v", p)
	}
}

// TestAbortTurnSurfacesAbortedError: a user abort mid-turn surfaces the
// error as Type "aborted" (the upstream MessageAbortedError class) on the
// round's assistant message and on the wire.
func TestAbortTurnSurfacesAbortedError(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{
			{Kind: "text", Text: "working"},
			{Kind: "tool", Name: "glob", CallID: "t1", Text: `{"pattern":"x*"}`, Finish: "tool_calls"},
		}},
	}
	h.slowTurn = true // hold the stream open so the abort lands mid-turn
	if _, err := h.eng.Send(context.Background(), ses, "slow", nil); err != nil {
		t.Fatal(err)
	}
	waitBusy(t, h, ses)
	// The round's assistant row exists before the first stream attempt —
	// sync on it so the abort lands mid-turn (the slow window), not in the
	// pre-round window where there is no message to attach the error to.
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeMessageUpdated {
			return false
		}
		var p protocol.MessageUpdatedProps
		if json.Unmarshal(e.Properties, &p) != nil {
			return false
		}
		return p.Info.Role == "assistant"
	})
	if !h.eng.Abort(ses) {
		t.Fatal("abort rejected")
	}
	waitIdle(t, h, ses, func() {})
	msgs, err := h.db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range msgs {
		if m.Role != "assistant" || m.Error == nil {
			continue
		}
		found = true
		if m.Error.Type != "aborted" || m.Error.Message != "aborted by the user" {
			t.Fatalf("aborted row Error = %+v", m.Error)
		}
	}
	if !found {
		t.Fatalf("no assistant row with an aborted error: %+v", msgs)
	}
	// the wire event carries the aborted error too
	if n := h.countErrorEvents(); n != 1 {
		t.Fatalf("error events = %d, want 1", n)
	}
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeMessageUpdated {
			return false
		}
		var p protocol.MessageUpdatedProps
		if json.Unmarshal(e.Properties, &p) != nil {
			return false
		}
		return p.Info.Error != nil && p.Info.Error.Type == "aborted"
	})
}

// TestSuccessfulTurnHasNoErrorSurface: a clean turn ends idle with no wire
// error and no row.Error anywhere.
func TestSuccessfulTurnHasNoErrorSurface(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{{Kind: "text", Text: "hello", Finish: "stop", Usage: &llm.Usage{Input: 42, Output: 7}}}},
	}
	done := make(chan error, 1)
	if _, err := h.eng.Send(context.Background(), ses, "hi", func(e error) { done <- e }); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("turn ended with error %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onDone not called")
	}
	assertNoErrorSurface(t, h, ses)
}

// TestMaxToolRoundsEndsIdleWithoutError: exhausting the tool round budget
// ends the turn idle WITHOUT a wire error (a non-failure in the yolo model:
// the turn's error is the onDone log site only).
func TestMaxToolRoundsEndsIdleWithoutError(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	// 50 tool rounds (one per budget slot) + the final text the engine never
	// requests. Patterns are unique per call (identical args would trip the
	// doom-loop gate and park an ask nobody answers).
	turns := make([]fake.Turn, 0, 51)
	for i := 0; i < 50; i++ {
		turns = append(turns, fake.Turn{Parts: []llm.Part{{
			Kind: "tool", Name: "glob", CallID: fmt.Sprintf("c%d", i),
			Text: fmt.Sprintf(`{"pattern":"p%d*"}`, i), Finish: "tool_calls",
		}}})
	}
	endPart := llm.Part{Kind: "text", Text: "end", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}
	turns = append(turns, fake.Turn{Parts: []llm.Part{endPart}})
	h.drv.Turns = turns
	done := make(chan error, 1)
	if _, err := h.eng.Send(context.Background(), ses, "spin", func(e error) { done <- e }); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("max tool rounds must still surface the error to onDone (the log site)")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("onDone not called")
	}
	assertNoErrorSurface(t, h, ses)
}

// TestOverflowEndsIdleWithoutError: a context overflow ends the turn idle
// with the synthetic note and NO wire error (the note + idle are the
// surface; the yolo model treats it as a non-failure).
func TestOverflowEndsIdleWithoutError(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	// model Context is 100000 (harness seam) — usage.Input 100001 overflows.
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{{Kind: "text", Text: "big", Finish: "stop", Usage: &llm.Usage{Input: 100001, Output: 5}}}},
	}
	if _, err := h.eng.Send(context.Background(), ses, "big", nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, h, ses, func() {})
	assertNoErrorSurface(t, h, ses)
	msgs, err := h.db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, m := range msgs {
		parts, _ := h.db.ListParts(t.Context(), m.ID)
		for _, p := range parts {
			pt, _ := storage.PartToProtocol(p)
			if pt.Type == "text" && strings.Contains(pt.Text, "context overflow") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("synthetic overflow note missing")
	}
}
