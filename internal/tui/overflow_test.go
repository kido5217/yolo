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
// The footer and the locked quit dialog stay single-line by design (the help
// dialog is modal since S3.6). The slash menu since S2 keeps its width-exact
// box rows instead: the over-wide row is truncated at the box content width.

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
	// The bordered chrome (S2) is width-exact and truncates (no wrap): the
	// single row is 20 cols, the label + the description cut at the box
	// content width (avail 16: the 5-col label + the "  " offset + 9 cols of
	// the description).
	if got != "| /quit  quits the |" {
		t.Fatalf("menu row = %q, want the width-exact truncated box row", got)
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

func TestHomeTipsRowsWrap(t *testing.T) {
	// the home tip wraps at the TIP BOX width (min(75, w-4)): at a narrow
	// terminal (w=40 → tip box width 36) the NO_MODELS nudge (54 cols —
	// the testApp has no providers) wraps to 2 rows, each within the box.
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 40, Height: 24}
	rows := a.homeTipsRows()
	if len(rows) != 2 {
		t.Fatalf("tips rows = %d, want 2 (wrapped at the tip box width)", len(rows))
	}
	for i, r := range rows {
		if cw := runeWidth(stripANSI(r)); cw > 36 {
			t.Fatalf("tips row %d width = %d, want <= 36: %q", i, cw, stripANSI(r))
		}
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
