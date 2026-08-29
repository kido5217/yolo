package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

// yolo-ukc: below-viewport surfaces (toasts, permission, slash menu, model /
// agent dialogs, home rows, the error line) must word-wrap at the terminal
// width instead of being clipped — the viewport only guards the transcript.
// The footer and the locked quit/help dialogs stay single-line by design.

// fitsWidth reports whether every line of s is at most w display columns
// (the fixtures here are plain ASCII, so rune count is the width).
func fitsWidth(t *testing.T, s string, w int) {
	t.Helper()
	for _, l := range strings.Split(s, "\n") {
		if n := len([]rune(l)); n > w {
			t.Fatalf("line wider than %d (got %d): %q", w, n, l)
		}
	}
}

// rejoined flattens the wrap newlines so a full-text containment check works.
func rejoined(s string) string { return strings.ReplaceAll(s, "\n", " ") }

func TestToastsViewWraps(t *testing.T) {
	a := testSessionApp(sessionFixture())
	long := strings.Repeat("boom ", 20)
	a.toast(long)
	got := stripANSI(a.toastsView(20))
	fitsWidth(t, got, 20)
	if !strings.Contains(rejoined(got), "• "+strings.TrimRight(long, " ")) {
		t.Fatalf("toast text lost in wrap:\n%q", got)
	}
	// A short toast at a wide width stays one unchanged line.
	a2 := testSessionApp(sessionFixture())
	a2.toast(busyToast)
	if got := stripANSI(a2.toastsView(80)); got != "• "+busyToast {
		t.Fatalf("short toast changed: %q", got)
	}
}

func TestMenuViewWraps(t *testing.T) {
	a := testSessionApp(sessionFixture())
	a.prompt.input.SetValue("/q")
	long := strings.Repeat("quits the running app ", 5)
	cmds := []protocol.Command{{Name: "/quit", Description: long}}
	got := stripANSI(a.prompt.menuView(cmds, 20, a.theme))
	fitsWidth(t, got, 20)
	// Wrapping collapses the double-space separator into one.
	if !strings.Contains(rejoined(got), "/quit "+strings.TrimRight(long, " ")) {
		t.Fatalf("menu text lost in wrap:\n%q", got)
	}
}

func TestPermissionViewWraps(t *testing.T) {
	a := permApp()
	a.store.Pending[0].Patterns = []string{strings.Repeat("ls -la /very/long/path ", 6)}
	got := stripANSI(a.permissionView(20))
	fitsWidth(t, got, 20)
	if !strings.Contains(rejoined(got), "patterns: "+strings.TrimRight(strings.Repeat("ls -la /very/long/path ", 6), " ")) {
		t.Fatalf("permission text lost in wrap:\n%q", got)
	}
}

func TestAgentDlgViewWraps(t *testing.T) {
	// The long description must be in the store BEFORE open: the select
	// freezes its options at syncAgentSel time (the catalog-arrival path is
	// the re-seed — a post-open store swap no longer reaches the render).
	a := agentApp()
	long := strings.Repeat("permits tools without prompts ", 6)
	a.store.Agents = []protocol.Agent{{Name: "build", Description: long}}
	a.openAgentDialog()
	a.Cmds = nil
	got := stripANSI(a.dlg.agent().view(&a.store, 20, 24, a.theme))
	fitsWidth(t, got, 20)
	flat := strings.Join(strings.Fields(rejoined(got)), " ")
	if !strings.Contains(flat, "build") {
		t.Fatalf("agent text lost in wrap:\n%q", got)
	}
}

func TestModelDlgViewWraps(t *testing.T) {
	a := openModelAt()
	got := stripANSI(a.dlg.model().view(&a.store, 40, 24, a.theme))
	fitsWidth(t, got, 40)
	flat := strings.Join(strings.Fields(rejoined(got)), " ")
	for _, tok := range []string{"Qwen", "Claude Opus 4.7", "GPT-5 Nano"} {
		if !strings.Contains(flat, tok) {
			t.Fatalf("model dialog lost %q in wrap:\n%q", tok, got)
		}
	}
}

func TestHomeRenderWraps(t *testing.T) {
	a := testApp(protocol.Session{
		ID: "ses_1", Title: strings.Repeat("long title word ", 10),
		Model: refModel("kido", "q"), Time: protocol.SessionTime{Updated: testNow - 60000},
	})
	// w >= logoWidth: the logo is a fixed 39-column glyph block that
	// never wraps or shrinks (the upstream look; clipped on <39-column
	// terminals). Render at logoWidth+1 so the fitsWidth contract holds
	// while the long session title still exercises the wrap.
	got := stripANSI(a.home.render(&a.store, logoWidth+1, a.theme))
	fitsWidth(t, got, logoWidth+1)
	// Whitespace-normalized: continuation lines are indented.
	flat := strings.Join(strings.Fields(rejoined(got)), " ")
	if !strings.Contains(flat, strings.TrimRight(strings.Repeat("long title word ", 10), " ")) {
		t.Fatalf("home title lost in wrap:\n%q", got)
	}
}

// TestSessionFrameFitsTerminal: the composed session frame (transcript,
// overlays, error line, prompt, footer) must stay within the terminal width
// when every text surface overflows.
func TestSessionFrameFitsTerminal(t *testing.T) {
	a := testSessionApp(sessionFixture())
	long := strings.Repeat("overflow ", 20)
	a.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	a.route = routeSession
	a.lastErr = long
	a.toast(long)
	a.sess.isDirty = true
	fitsWidth(t, stripANSI(a.view()), 50)
}
