package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

// TestMessageDialogOpen pins the opener (the S7.3 yolo-surface alt+m, the
// Expand/Think precedent): the session route + a message pushes the
// dlgMessage modal with the LAST message snapshot; esc closes it; with no
// message the key is consumed and the stack stays empty; the home route
// never opens it (the session keys are not dispatched there).
func TestMessageDialogOpen(t *testing.T) {
	a := testApp()
	a.route = routeSession
	a.store.Messages = []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1", Role: "user"}},
		{Info: protocol.Message{ID: "m2", Role: "assistant"}},
	}
	a.handleKey(pressAlt('m'))
	d, ok := a.dlg.top()
	if !ok || !d.modal || d.kind != dlgMessage || d.message == nil || d.message.Info.ID != "m2" {
		t.Fatalf("after alt+m: top=%+v, want the dlgMessage modal with the last message", d)
	}
	a.handleKey(press(tea.KeyEscape))
	if d, ok := a.dlg.top(); ok && d.modal {
		t.Fatalf("esc must close the message dialog: top=%+v", d)
	}

	b := testApp()
	b.route = routeSession
	b.handleKey(pressAlt('m'))
	if d, ok := b.dlg.top(); ok && d.modal {
		t.Fatalf("no message: the stack must stay empty, top=%+v", d)
	}

	c := testApp()
	c.store.Messages = []protocol.MessageWithParts{{Info: protocol.Message{ID: "m1"}}}
	c.handleKey(pressAlt('m')) // the home route: the session keys are not dispatched
	if d, ok := c.dlg.top(); ok && d.modal {
		t.Fatalf("home route: the message dialog must not open, top=%+v", d)
	}
}

// TestMessageViewRender pins the full-message render (the stripANSI unit
// idiom): the header row, the meta line (the created time is formatted the
// same way in the expectation — TZ-independent), the per-part headers +
// content, the msgPartMaxLines clamp + the hint (AFTER the head), the
// error line.
func TestMessageViewRender(t *testing.T) {
	a := testApp()
	m := &protocol.MessageWithParts{
		Info: protocol.Message{
			ID:     "m1",
			Role:   "assistant",
			Agent:  "build",
			Time:   protocol.MessageTime{Created: 1_700_000_000_000},
			Tokens: &protocol.Tokens{Input: 123, Output: 45},
			Cost:   0.42,
		},
		Parts: []protocol.Part{
			{Type: "text", Text: "hello world"},
			{Type: "reasoning", Text: "think hard"},
			{Type: "tool", Tool: "bash", State: &protocol.ToolState{
				Title:  "Run command",
				Output: strings.Repeat("out\n", 39) + "out", // 40 lines, no trailing newline
				Error:  "boom",
			}},
		},
	}
	wantTime := time.UnixMilli(1_700_000_000_000).Format("15:04:05")
	got := stripANSI(a.messageView(m, 86, a.theme))
	if !strings.Contains(got, "Message") || !strings.Contains(got, "esc") {
		t.Fatalf("header row missing:\n%s", got)
	}
	if want := "assistant · build · " + wantTime + " · ↑123 ↓45 · $0.42"; !strings.Contains(got, want) {
		t.Fatalf("meta line %q missing:\n%s", want, got)
	}
	if !strings.Contains(got, "Text") || !strings.Contains(got, "hello world") {
		t.Fatalf("the text part is missing:\n%s", got)
	}
	if !strings.Contains(got, "Reasoning") || !strings.Contains(got, "think hard") {
		t.Fatalf("the reasoning part is missing:\n%s", got)
	}
	if !strings.Contains(got, "Tool: bash — Run command") {
		t.Fatalf("the tool header is missing:\n%s", got)
	}
	// the clamp: the 40-line output renders 12 head lines + the hint.
	n := 0
	for _, l := range strings.Split(got, "\n") {
		if l == "out" {
			n++
		}
	}
	if n != 12 {
		t.Fatalf("clamped output lines = %d, want 12 (msgPartMaxLines):\n%s", n, got)
	}
	if !strings.Contains(got, "… (28 more lines)") {
		t.Fatalf("the overflow hint is missing:\n%s", got)
	}
	if !strings.Contains(got, "error: boom") {
		t.Fatalf("the tool error line is missing:\n%s", got)
	}
	// the empty-parts omissions: the meta line without agent / tokens /
	// cost.
	m2 := &protocol.MessageWithParts{
		Info:  protocol.Message{ID: "m2", Role: "user", Time: protocol.MessageTime{Created: 1_700_000_000_000}},
		Parts: []protocol.Part{{Type: "text", Text: "hi"}},
	}
	got2 := stripANSI(a.messageView(m2, 86, a.theme))
	if want := "user · " + wantTime; !strings.Contains(got2, want) {
		t.Fatalf("the user meta line %q missing:\n%s", want, got2)
	}
}
