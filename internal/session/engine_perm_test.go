package session_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

type toolPart struct {
	// partID is the model call id (the engine stores the call id as the part
	// id; CallID itself is not persisted — see PROGRESS deviations).
	partID string
	state  protocol.ToolState
}

// toolParts lists the session's tool parts (all messages), oldest first.
func toolParts(t *testing.T, h *harness, ses string) []toolPart {
	t.Helper()
	msgs, err := h.db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	var out []toolPart
	for _, m := range msgs {
		if m.Role != "assistant" {
			continue
		}
		parts, err := h.db.ListParts(t.Context(), m.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range parts {
			pt, err := storage.PartToProtocol(p)
			if err != nil {
				t.Fatal(err)
			}
			if pt.Type == "tool" && pt.State != nil {
				out = append(out, toolPart{partID: pt.ID, state: *pt.State})
			}
		}
	}
	return out
}

// askedPermissions lists the permission actions of all permission.asked
// events observed, in bus order.
func askedPermissions(t *testing.T, h *harness) []string {
	t.Helper()
	h.eventsMu.Lock()
	defer h.eventsMu.Unlock()
	var out []string
	for _, e := range h.events {
		if e.Type != protocol.EventTypePermissionAsked {
			continue
		}
		var p protocol.PermissionAskedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p.Permission)
	}
	return out
}

// eventIndex is the first bus-order index of an event matching cond.
func eventIndex(t *testing.T, h *harness, cond func(protocol.Event) bool) int {
	t.Helper()
	h.eventsMu.Lock()
	defer h.eventsMu.Unlock()
	for i, e := range h.events {
		if cond(e) {
			return i
		}
	}
	return -1
}

func TestPermissionDenyStopsToolButNotTurn(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	// build agent, config deny rule: bash "cat *" is denied.
	h.cfgPermission = []protocol.Rule{{Permission: "bash", Pattern: "cat *", Action: "deny"}}
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{
			{Kind: "tool", Name: "bash", CallID: "c1", Text: `{"command":"cat secret.txt"}`, Finish: "tool_calls"},
		}},
		{Parts: []llm.Part{{Kind: "text", Text: "ok", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "sneak", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	parts := toolParts(t, h, ses)
	if len(parts) != 1 {
		t.Fatalf("tool parts = %+v", parts)
	}
	st := parts[0].state
	if st.Status != "error" {
		t.Fatalf("tool state = %+v", st)
	}
	if !strings.Contains(st.Error, "permission rejected") {
		t.Fatalf("error = %q", st.Error)
	}
	// the turn continued to the model's next round.
	msgs, err := h.db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) < 3 || msgs[len(msgs)-1].Role != "assistant" {
		t.Fatalf("turn did not continue: %d messages, last role = %q", len(msgs), msgs[len(msgs)-1].Role)
	}
}

func TestPermissionAlwaysPersistsAndSkipsNext(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)
	// build agent: read "*.env" ASKS (base matrix).
	fp := filepath.Join(d, ".env")
	writeFile(t, fp, "SECRET=1")
	readCall := func(callID string) llm.Part {
		return llm.Part{Kind: "tool", Name: "read", CallID: callID, Text: fmt.Sprintf(`{"filePath":%q}`, fp), Finish: "tool_calls"}
	}
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{readCall("c1")}},
		{Parts: []llm.Part{readCall("c2")}},
		{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	h.queueReplies("always")
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "read env", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	parts := toolParts(t, h, ses)
	byCall := map[string]toolPart{}
	for _, p := range parts {
		byCall[p.partID] = p
	}
	if st := byCall["c1"].state; st.Status != "completed" {
		t.Fatalf("c1 = %+v", st)
	}
	if st := byCall["c2"].state; st.Status != "completed" {
		t.Fatalf("c2 (should skip ask via always) = %+v", st)
	}
	perms := askedPermissions(t, h)
	if len(perms) != 1 || perms[0] != "read" {
		t.Fatalf("asked permissions = %v, want [read]", perms)
	}
	rules, err := h.db.AlwaysRules(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rules {
		if r.Permission == "read" && r.Pattern == "*" && r.Action == "allow" {
			found = true
		}
	}
	if !found {
		t.Fatalf("always rules = %+v, want [{read * allow}]", rules)
	}
}

func TestHiddenToolNotSentToModel(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	// wildcard-deny as the LAST edit rule hides both edit and write.
	h.cfgPermission = []protocol.Rule{{Permission: "edit", Pattern: "*", Action: "deny"}}
	h.drv.Turns = []fake.Turn{{Parts: []llm.Part{{Kind: "text", Text: "x", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}}}
	d := t.TempDir()
	ses := h.startSession(t, d)
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "hi", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	reqs := nonTitle(h.drv.Requests())
	if len(reqs) != 1 {
		t.Fatalf("model rounds = %d, want 1", len(reqs))
	}
	names := map[string]bool{}
	for _, td := range reqs[0].Tools {
		names[td.Name] = true
	}
	if names["edit"] || names["write"] {
		t.Fatalf("hidden tools leaked: %v", names)
	}
	if !names["read"] || !names["bash"] {
		t.Fatalf("visible tools missing: %v", names)
	}
}

func TestDoomLoopThirdIdenticalAsks(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d) // agent build: doom_loop ASKS (base matrix)
	globCall := func(callID string) llm.Part {
		return llm.Part{Kind: "tool", Name: "glob", CallID: callID, Text: `{"pattern":"x*"}`, Finish: "tool_calls"}
	}
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{globCall("a"), globCall("b"), globCall("c"), globCall("d")}},
		{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	h.queueReplies("once", "once")
	waitIdle(t, h, ses, func() {
		if _, err := h.eng.Send(t.Context(), ses, "loop", nil); err != nil {
			t.Fatalf("Send: %v", err)
		}
	})

	parts := toolParts(t, h, ses)
	if len(parts) != 4 {
		t.Fatalf("tool parts = %+v", parts)
	}
	for _, p := range parts {
		if p.state.Status != "completed" {
			t.Fatalf("call %s = %+v", p.partID, p.state)
		}
	}
	perms := askedPermissions(t, h)
	if len(perms) != 2 || perms[0] != "doom_loop" || perms[1] != "doom_loop" {
		t.Fatalf("doom asks = %v, want [doom_loop doom_loop] (c and d)", perms)
	}
	// LOCKED ordering: c's doom ask fires BEFORE c runs.
	askIdx := eventIndex(t, h, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypePermissionAsked {
			return false
		}
		var p protocol.PermissionAskedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			return false
		}
		return p.Permission == "doom_loop"
	})
	cRunningIdx := eventIndex(t, h, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeMessagePartUpdated {
			return false
		}
		var p protocol.MessagePartUpdatedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			return false
		}
		return p.Part.Type == "tool" && p.Part.CallID == "c" && p.Part.State != nil && p.Part.State.Status == "running"
	})
	if askIdx == -1 || cRunningIdx == -1 || askIdx >= cRunningIdx {
		t.Fatalf("doom ask index %d, c running index %d: ask must precede run", askIdx, cRunningIdx)
	}
}

func TestPermissionAbortDuringAskAbortsTool(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d) // agent build: read "*.env" ASKS (base matrix)
	fp := filepath.Join(d, ".env")
	writeFile(t, fp, "SECRET=1")
	h.drv.Turns = []fake.Turn{
		{Parts: []llm.Part{
			{Kind: "tool", Name: "read", CallID: "c1", Text: fmt.Sprintf(`{"filePath":%q}`, fp), Finish: "tool_calls"},
		}},
		{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop", Usage: &llm.Usage{Input: 1, Output: 1}}}},
	}
	// no queued reply: the turn parks at the ask until it is aborted.

	var sendErr error
	finished := make(chan struct{})
	go func() {
		if _, err := h.eng.Send(t.Context(), ses, "read env", nil); err != nil {
			sendErr = err
		}
		close(finished)
	}()

	waitBusy(t, h, ses)
	// wait for the ask to park, then cancel the turn mid-prompt.
	h.waitForEvent(t, func(e protocol.Event) bool {
		return e.Type == protocol.EventTypePermissionAsked
	})
	if !h.eng.Abort(ses) {
		t.Fatal("Abort returned false (no active turn)")
	}

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("Send did not return after Abort")
	}
	if sendErr != nil {
		t.Fatalf("Send: %v", sendErr)
	}
	deadline := time.Now().Add(5 * time.Second)
	for h.eng.Status(ses) == protocol.SessionStatusBusy {
		if time.Now().After(deadline) {
			t.Fatal("engine did not go idle after abort")
		}
		time.Sleep(10 * time.Millisecond)
	}

	parts := toolParts(t, h, ses)
	var st *protocol.ToolState
	for _, p := range parts {
		if p.partID == "c1" {
			st = &p.state
			break
		}
	}
	if st == nil {
		t.Fatalf("no tool part c1: %+v", parts)
	}
	if st.Status != "error" || !strings.Contains(st.Error, "aborted") {
		t.Fatalf("c1 = %+v, want error containing aborted", st)
	}
}
