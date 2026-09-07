package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
)

// mkEv builds a typed event for the renderer tests.
func mkEv(t *testing.T, typ string, props any) protocol.Event {
	t.Helper()
	ev, err := protocol.MakeEvent(typ, props)
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

// TestRunDefaultRenderer pins leg (c): the default-format stdout bytes
// (deltas verbatim + the finalize-newline rule) and the stderr lines
// (header-once, tool lines incl. the error append, the permission-free
// error line, reasoning gated on --thinking), plus the foreign-session
// filter.
func TestRunDefaultRenderer(t *testing.T) {
	var out, errw bytes.Buffer
	r := newRenderer(formatDefault, true, "ses_1", "> build · kido/q", func() int64 { return 1 }, &out, &errw)
	r.apply(mkEv(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "ses_1", Info: protocol.Message{ID: "msg_u", Role: "user"},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "ses_1", Info: protocol.Message{ID: "msg_1", Role: "assistant"},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "ses_1", Info: protocol.Message{ID: "msg_1", Role: "assistant", Finish: "stop", Tokens: &protocol.Tokens{Input: 1, Output: 1}, Cost: 0.1},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_1", Part: protocol.Part{ID: "prt_r", Type: protocol.PartTypeReasoning, Text: "Let me check.", Time: protocol.PartTime{End: 1}},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_1", Part: protocol.Part{ID: "prt_t", Type: protocol.PartTypeTool, Tool: "bash", State: &protocol.ToolState{Status: "running"}},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_1", Part: protocol.Part{ID: "prt_t", Type: protocol.PartTypeTool, Tool: "bash", State: &protocol.ToolState{Status: "completed", Title: "ls", Output: "ok"}},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{SessionID: "ses_1", PartID: "prt_2", Field: "text", Delta: "Hel"}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{SessionID: "ses_1", PartID: "prt_2", Field: "text", Delta: "lo"}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_1", Part: protocol.Part{ID: "prt_2", Type: protocol.PartTypeText, Text: "Hello", Time: protocol.PartTime{End: 1}},
	}))
	// finalized without deltas, text already ends with a newline -> no
	// extra newline
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_1", Part: protocol.Part{ID: "prt_3", Type: protocol.PartTypeText, Text: "tail\n", Time: protocol.PartTime{End: 1}},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_1", Part: protocol.Part{ID: "prt_t2", Type: protocol.PartTypeTool, Tool: "read", State: &protocol.ToolState{Status: "error", Title: "notes.txt", Error: "command aborted"}},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "ses_1", Info: protocol.Message{ID: "msg_2", Role: "assistant", Error: &protocol.MessageError{Type: "unknown", Message: "model provider unavailable"}},
	}))
	// a foreign session's events are ignored
	r.apply(mkEv(t, protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{SessionID: "ses_other", PartID: "prt_x", Field: "text", Delta: "zzz"}))

	if want := "Hello\n"; out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
	wantErr := []string{
		"> build · kido/q",
		"thinking: Let me check.",
		"tool: bash ls",
		"tool: read notes.txt (error: command aborted)",
		"error: model provider unavailable",
	}
	gotErr := strings.Split(strings.TrimRight(errw.String(), "\n"), "\n")
	if len(gotErr) != len(wantErr) {
		t.Fatalf("stderr lines = %d (%q), want %d", len(gotErr), errw.String(), len(wantErr))
	}
	for i := range wantErr {
		if gotErr[i] != wantErr[i] {
			t.Fatalf("stderr[%d] = %q, want %q", i, gotErr[i], wantErr[i])
		}
	}
}

// TestRunDefaultRendererThinkingOff pins the --thinking gate (off: no
// reasoning line on stderr, stdout untouched).
func TestRunDefaultRendererThinkingOff(t *testing.T) {
	var out, errw bytes.Buffer
	r := newRenderer(formatDefault, false, "ses_1", "> build · kido/q", func() int64 { return 1 }, &out, &errw)
	r.apply(mkEv(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "ses_1", Info: protocol.Message{ID: "msg_1", Role: "assistant"},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_1", Part: protocol.Part{ID: "prt_r", Type: protocol.PartTypeReasoning, Text: "hidden", Time: protocol.PartTime{End: 1}},
	}))
	if strings.Contains(errw.String(), "thinking:") {
		t.Fatalf("thinking off: stderr = %q", errw.String())
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if r.turnErr {
		t.Fatal("turnErr set without an error event")
	}
}

// TestRunPermissionNote pins the §6.1-3 note bytes (both pattern cases).
func TestRunPermissionNote(t *testing.T) {
	var b bytes.Buffer
	permissionNote(&b, &protocol.PermissionAskedProps{Permission: "edit", Patterns: []string{"notes.txt"}})
	if got := b.String(); got != "permission requested: edit (notes.txt); auto-rejecting\n" {
		t.Fatalf("note = %q", got)
	}
	b.Reset()
	permissionNote(&b, &protocol.PermissionAskedProps{Permission: "bash"})
	if got := b.String(); got != "permission requested: bash; auto-rejecting\n" {
		t.Fatalf("note = %q", got)
	}
}

// TestRunSettleTurnError pins the settle read (spec §6.3): a persisted
// assistant error sets the flag even when the event was lost.
func TestRunSettleTurnError(t *testing.T) {
	msgs := []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1", Role: "user"}},
		{Info: protocol.Message{ID: "m2", Role: "assistant"}},
		{Info: protocol.Message{ID: "m3", Role: "assistant", Error: &protocol.MessageError{Type: "unknown", Message: "boom"}}},
	}
	if !settleTurnError(msgs, false) {
		t.Fatal("persisted assistant error must set the flag")
	}
	if settleTurnError(msgs[:2], false) {
		t.Fatal("no assistant error: flag stays false")
	}
	if !settleTurnError(nil, true) {
		t.Fatal("the event-observed flag survives")
	}
}

// TestRunNDJSONBasics pins the json-mode envelope + the lines this task's
// scripted sequence produces (step_start on a NEW assistant id, tool_use,
// text, the yolo-shaped error line, and the step-finish omission rule on
// a failed turn); stderr carries ONLY the error line (the header and
// tool/thinking lines are suppressed in json mode). The full §6.2 byte
// pins (reasoning, the exact example) are Task 10.
func TestRunNDJSONBasics(t *testing.T) {
	var out, errw bytes.Buffer
	r := newRenderer(formatJSON, true, "ses_abc", "", func() int64 { return 1725676800000 }, &out, &errw)
	r.apply(mkEv(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "ses_abc", Info: protocol.Message{ID: "msg_1", Role: "assistant"},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_abc", Part: protocol.Part{ID: "prt_1", SessionID: "ses_abc", MessageID: "msg_1", Type: protocol.PartTypeTool, CallID: "prt_1", Tool: "bash", State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": "ls"}, Title: "ls", Output: "notes.txt"}, Time: protocol.PartTime{Start: 0}},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{SessionID: "ses_abc", PartID: "prt_2", Field: "text", Delta: "Hello"}))
	r.apply(mkEv(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: "ses_abc", Part: protocol.Part{ID: "prt_2", SessionID: "ses_abc", MessageID: "msg_1", Type: protocol.PartTypeText, Text: "Hello", Time: protocol.PartTime{Start: 1, End: 2}},
	}))
	r.apply(mkEv(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "ses_abc", Info: protocol.Message{ID: "msg_9", Role: "assistant", Error: &protocol.MessageError{Type: "unknown", Message: "model provider unavailable"}},
	}))
	r.finish()

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	// NOTE (principle 5): the plan's expectation omitted two faithful bytes —
	// the tool_use state carries its own `time` (ToolState.Time has no
	// omitempty), and msg_9 (a NEW assistant id) emits its own step_start
	// per the "step_start on a new assistant id" rule. Both are pinned here.
	want := []string{
		`{"type":"step_start","timestamp":1725676800000,"sessionID":"ses_abc","part":{"type":"step-start","messageID":"msg_1"}}`,
		`{"type":"tool_use","timestamp":1725676800000,"sessionID":"ses_abc","part":{"id":"prt_1","sessionID":"ses_abc","messageID":"msg_1","type":"tool","callID":"prt_1","tool":"bash","state":{"status":"completed","input":{"command":"ls"},"title":"ls","output":"notes.txt","time":{"start":0}},"time":{"start":0}}}`,
		`{"type":"text","timestamp":1725676800000,"sessionID":"ses_abc","part":{"id":"prt_2","sessionID":"ses_abc","messageID":"msg_1","type":"text","text":"Hello","time":{"start":1,"end":2}}}`,
		`{"type":"step_start","timestamp":1725676800000,"sessionID":"ses_abc","part":{"type":"step-start","messageID":"msg_9"}}`,
		`{"type":"error","timestamp":1725676800000,"sessionID":"ses_abc","error":{"type":"unknown","message":"model provider unavailable"}}`,
		`{"type":"step_finish","timestamp":1725676800000,"sessionID":"ses_abc","part":{"type":"step-finish"}}`,
	}
	if len(lines) != len(want) {
		t.Fatalf("ndjson lines = %d:\n%s", len(lines), out.String())
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line[%d] =\n%s\nwant\n%s", i, lines[i], want[i])
		}
	}
	if want := "error: model provider unavailable\n"; errw.String() != want {
		t.Fatalf("json-mode stderr = %q, want %q (header/tool lines suppressed)", errw.String(), want)
	}
}
