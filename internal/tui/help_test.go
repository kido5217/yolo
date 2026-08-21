package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

// T28 locks the help dialog text: the spec's keymap table verbatim plus the
// scroll/newline note. The table mirrors the actual bindings: pgup/pgdn are
// the only scroll keys (v0.1.1 reconciliation, the spec's arrow row never
// existed) and alt+e/alt+t are the real expand/think toggles. \+enter
// inserts a newline in the prompt.
const wantHelp = "Help\n" +
	"  | Key | Action |\n" +
	"  |---|---|\n" +
	"  | enter | send prompt |\n" +
	"  | esc | abort turn (busy) / close dialog |\n" +
	"  | ctrl+c | quit (confirm) |\n" +
	"  | ctrl+p | model dialog |\n" +
	"  | ctrl+a | agent dialog |\n" +
	"  | / | command menu |\n" +
	"  | pgup/pgdn | viewport scroll |\n" +
	"  | 1/2/3 | permission reply |\n" +
	"  | alt+e / alt+t | expand tool part / toggle reasoning |\n" +
	"  pgup/pgdn scroll \u00B7 \\+enter newline"

func openHelp(t *testing.T) *recApp {
	t.Helper()
	a := testApp()
	a.dlg.push(dialog{kind: dlgHelp})
	return a
}

func TestHelpDialogText(t *testing.T) {
	got := stripANSI(openHelp(t).dlgView())
	if got != wantHelp {
		t.Fatalf("help dialog mismatch:\ngot:\n%q\nwant:\n%q", got, wantHelp)
	}
}

func TestHelpAnyKeyCloses(t *testing.T) {
	a := openHelp(t)
	a.handleKey(press('q'))
	if d, ok := a.dlg.top(); ok {
		t.Fatalf("dialog %d still open after a key; help closes on any key", d.kind)
	}
}

// T28 locks the quit-confirm text to `quit? [y/n]`; y exits, n/esc go back.
func TestQuitConfirmTextAndKeys(t *testing.T) {
	a := testApp()
	a.dlg.push(dialog{kind: dlgQuit})
	if got := stripANSI(a.dlgView()); got != "quit? [y/n]" {
		t.Fatalf("quit dialog = %q, want %q", got, "quit? [y/n]")
	}
	cmds := a.handleKey(press('y'))
	if len(cmds) != 1 {
		t.Fatalf("y returned %d cmds, want 1 (quit)", len(cmds))
	}
	m := cmds[0]()
	if _, ok := m.(tea.QuitMsg); !ok {
		t.Fatalf("quit cmd yields %T, want tea.QuitMsg", m)
	}
	a.dlg.items = nil // y exits; clear the stack for the n path
	a.dlg.push(dialog{kind: dlgQuit})
	if cmds := a.handleKey(press('n')); len(cmds) != 0 {
		t.Fatalf("n returned %d cmds, want 0", len(cmds))
	}
	if _, ok := a.dlg.top(); ok {
		t.Fatal("quit dialog still open after n")
	}
}

// ctrl+c must open the quit-confirm from the session route too (the plan's
// TestTUIFullTurn quits from an open session; home alone was not enough).
func TestQuitConfirmFromSessionRoute(t *testing.T) {
	a := testApp(protocol.Session{ID: "ses_1"})
	a.route = routeSession
	a.cur = "ses_1"
	a.handleKey(ctrlCKey)
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgQuit {
		t.Fatalf("session-route ctrl+c opened %v (ok=%v), want dlgQuit", d.kind, ok)
	}
}
