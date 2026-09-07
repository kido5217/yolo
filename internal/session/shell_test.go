package session_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
)

// waitShellPart polls the DB until the shell's bash part reaches a
// terminal status (completed/error) and returns the decoded part: the
// exec goroutine finalizes out-of-band, so a bounded wait keeps the legs
// deterministic instead of a fixed sleep.
func waitShellPart(t *testing.T, h *harness, partID string, timeout time.Duration) protocol.Part {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		row, err := h.db.GetPart(t.Context(), partID)
		if err == nil {
			if p, perr := storage.PartToProtocol(row); perr == nil && p.State != nil &&
				(p.State.Status == "completed" || p.State.Status == "error") {
				return p
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("part %s did not reach a terminal status within %v", partID, timeout)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// partStatuses returns the State.Status sequence of the part id's
// message.part.updated events (the bus preserves publish order).
func partStatuses(t *testing.T, h *harness, partID string) []string {
	t.Helper()
	h.eventsMu.Lock()
	defer h.eventsMu.Unlock()
	out := []string{}
	for _, e := range h.events {
		if e.Type != protocol.EventTypeMessagePartUpdated {
			continue
		}
		var p protocol.MessagePartUpdatedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			t.Fatal(err)
		}
		if p.Part.ID == partID && p.Part.State != nil {
			out = append(out, p.Part.State.Status)
		}
	}
	return out
}

// lastMessageInfo returns the Info of the session's LAST message.updated
// event for the message id (bus order = publish order).
func lastMessageInfo(t *testing.T, h *harness, msgID string) protocol.Message {
	t.Helper()
	h.eventsMu.Lock()
	defer h.eventsMu.Unlock()
	var info protocol.Message
	found := false
	for _, e := range h.events {
		if e.Type != protocol.EventTypeMessageUpdated {
			continue
		}
		var p protocol.MessageUpdatedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			t.Fatal(err)
		}
		if p.Info.ID == msgID {
			info = p.Info
			found = true
		}
	}
	if !found {
		t.Fatalf("no message.updated event for %s", msgID)
	}
	return info
}

// waitShellTerminal waits for the terminal part.updated event (completed
// or error) of the part id on the bus, so the event-order assertions run
// after the exec goroutine's final publishes were delivered.
func waitShellTerminal(t *testing.T, h *harness, partID string) {
	t.Helper()
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeMessagePartUpdated {
			return false
		}
		var p protocol.MessagePartUpdatedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			return false
		}
		return p.Part.ID == partID && p.Part.State != nil &&
			(p.Part.State.Status == "completed" || p.Part.State.Status == "error")
	})
}

func TestShellHappyPath(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)

	res, err := h.eng.Shell(t.Context(), ses, "echo yolo-shell-out")
	if err != nil {
		t.Fatal(err)
	}
	if res.MessageID == "" || res.PartID == "" {
		t.Fatalf("result ids missing: %+v", res)
	}

	// The user message row (role user, agent from the session row) owns
	// the synthetic text part (verbatim, flagged synthetic).
	userRow, err := h.db.GetMessage(t.Context(), res.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	if userRow.Role != "user" || userRow.Agent != "build" {
		t.Fatalf("user row = role %q agent %q; want user/build", userRow.Role, userRow.Agent)
	}
	ump, err := h.db.ListParts(t.Context(), res.MessageID)
	if err != nil || len(ump) != 1 {
		t.Fatalf("user parts = %d, err %v; want 1", len(ump), err)
	}
	up, err := storage.PartToProtocol(ump[0])
	if err != nil {
		t.Fatal(err)
	}
	if up.Type != "text" || up.Text != "The following tool was executed by the user" {
		t.Fatalf("synthetic part = type %q text %q", up.Type, up.Text)
	}
	if up.IsSynthetic == nil || !*up.IsSynthetic {
		t.Fatalf("synthetic part IsSynthetic = %v; want true", up.IsSynthetic)
	}

	// The bash tool part completes with the command output; the assistant
	// row (role assistant, agent from the session row) owns it.
	part := waitShellPart(t, h, res.PartID, 5*time.Second)
	if part.State.Status != "completed" {
		t.Fatalf("status = %q; want completed", part.State.Status)
	}
	if !strings.Contains(part.State.Output, "yolo-shell-out") {
		t.Fatalf("Output = %q; want it to contain yolo-shell-out", part.State.Output)
	}
	if part.State.Title != "echo yolo-shell-out" {
		t.Fatalf("Title = %q; want the command", part.State.Title)
	}
	if _, ok := part.State.Metadata["exit"]; ok {
		t.Fatalf("Metadata = %v; want no exit key (exit 0)", part.State.Metadata)
	}
	asstRow, err := h.db.GetMessage(t.Context(), part.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	if asstRow.Role != "assistant" || asstRow.Agent != "build" {
		t.Fatalf("assistant row = role %q agent %q; want assistant/build", asstRow.Role, asstRow.Agent)
	}

	// The publishes: running -> completed for the part id, and the
	// assistant message.updated carries Time.Completed after finalize.
	waitShellTerminal(t, h, res.PartID)
	h.waitForEvent(t, func(e protocol.Event) bool {
		if e.Type != protocol.EventTypeMessageUpdated {
			return false
		}
		var p protocol.MessageUpdatedProps
		if err := json.Unmarshal(e.Properties, &p); err != nil {
			return false
		}
		return p.Info.ID == asstRow.ID && p.Info.Time.Completed > 0
	})
	if got := partStatuses(t, h, res.PartID); len(got) != 2 || got[0] != "running" || got[1] != "completed" {
		t.Fatalf("part.updated sequence = %v; want [running completed]", got)
	}
	if info := lastMessageInfo(t, h, res.MessageID); info.Model == nil {
		t.Fatal("user message.updated lost its model")
	}
}

func TestShellExitCode(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)

	res, err := h.eng.Shell(t.Context(), ses, "exit 3")
	if err != nil {
		t.Fatal(err)
	}
	part := waitShellPart(t, h, res.PartID, 5*time.Second)
	if part.State.Status != "completed" {
		t.Fatalf("status = %q; want completed (a non-zero exit is not a tool error)", part.State.Status)
	}
	if v, ok := part.State.Metadata["exit"]; !ok || v != float64(3) {
		t.Fatalf("Metadata[exit] = %v (ok %v); want 3", v, ok)
	}
}

func TestShellNoOutput(t *testing.T) {
	h := newHarness(t)
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)

	res, err := h.eng.Shell(t.Context(), ses, "true")
	if err != nil {
		t.Fatal(err)
	}
	part := waitShellPart(t, h, res.PartID, 5*time.Second)
	if part.State.Status != "completed" {
		t.Fatalf("status = %q; want completed", part.State.Status)
	}
	if part.State.Output != "(no output)" {
		t.Fatalf("Output = %q; want (no output)", part.State.Output)
	}
}

func TestShellTimeout(t *testing.T) {
	h := newHarness(t)
	h.shellTimeout = 300 * time.Millisecond // before build: the Deps seam
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)

	res, err := h.eng.Shell(t.Context(), ses, "sleep 10")
	if err != nil {
		t.Fatal(err)
	}
	part := waitShellPart(t, h, res.PartID, 5*time.Second)
	if part.State.Status != "error" {
		t.Fatalf("status = %q; want error", part.State.Status)
	}
	const prefix = "shell tool terminated command after exceeding timeout 300 ms"
	if !strings.HasPrefix(part.State.Error, prefix) {
		t.Fatalf("Error = %q; want prefix %q", part.State.Error, prefix)
	}
}

func TestShellDeletedSession(t *testing.T) {
	t.Run("engine-deleted", func(t *testing.T) {
		h := newHarness(t)
		h.build(t)
		d := t.TempDir()
		ses := h.startSession(t, d)
		// The engine's delete path (the deleted flag + the shell close)
		// with the row still present: shellFor returns nil.
		h.eng.Close(ses)
		_, err := h.eng.Shell(t.Context(), ses, "echo hi")
		if !errors.Is(err, session.ErrShellClosed) {
			t.Fatalf("err = %v; want ErrShellClosed", err)
		}
	})
	t.Run("row-deleted", func(t *testing.T) {
		h := newHarness(t)
		h.build(t)
		d := t.TempDir()
		ses := h.startSession(t, d)
		if err := h.db.DeleteSession(t.Context(), ses); err != nil {
			t.Fatal(err)
		}
		h.eng.Close(ses)
		_, err := h.eng.Shell(t.Context(), ses, "echo hi")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("err = %v; want storage.ErrNotFound", err)
		}
	})
}

// TestShellCloseDuringRun pins the Step-4 delete-during-run contract:
// deleting the session while a shell command runs does NOT produce a
// "command aborted" terminal state — the exec's ctx is context.Background
// (errShellAborted needs a ctx cancel) and Shell.Close blocks on the
// shell mutex the running Exec holds, so the proc-group kill lands only
// AFTER the exec returns on its own (here the per-command timer). The
// observable contract: Close returns (bounded by the command's own
// timeout) and the terminal part STILL LANDS (finalize-must-land), its
// terminal publishes suppressed for the deleted session.
func TestShellCloseDuringRun(t *testing.T) {
	h := newHarness(t)
	h.shellTimeout = 500 * time.Millisecond
	h.build(t)
	d := t.TempDir()
	ses := h.startSession(t, d)

	res, err := h.eng.Shell(t.Context(), ses, "sleep 10")
	if err != nil {
		t.Fatal(err)
	}
	// Delete the session while the command runs; Close returns once the
	// running exec settles (the 500 ms timer fires first).
	h.eng.Close(ses)

	part := waitShellPart(t, h, res.PartID, 5*time.Second)
	if part.State.Status != "error" {
		t.Fatalf("status = %q; want error", part.State.Status)
	}
	if !strings.Contains(part.State.Error, "exceeding timeout 500 ms") {
		t.Fatalf("Error = %q; want the timeout message", part.State.Error)
	}
	// The terminal publish is suppressed (deleted session): the part saw
	// exactly one part.updated (the running frame) on the bus.
	if got := partStatuses(t, h, res.PartID); len(got) != 1 || got[0] != "running" {
		t.Fatalf("part.updated sequence = %v; want [running] (terminal suppressed)", got)
	}
}
