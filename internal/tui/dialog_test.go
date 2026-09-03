package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// pushTestModal pushes a modal item carrying an empty (two-pane-era)
// modelDlg payload: its view renders the "Model" title + "  loading…" line,
// enough to pin the overlay frame (S2.9 flips modelDlg to the select and
// re-points these payloads at a fixture catalog).
func pushTestModal(t *testing.T, a *App, size dlgSize, onClose func(*App)) {
	t.Helper()
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, size, onClose)
}

func TestModalStackOps(t *testing.T) {
	a := testApp()
	closed := []string{}
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed = append(closed, "first") })
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgLarge, func(*App) { closed = append(closed, "second") })
	if got := len(a.dlg.items); got != 2 {
		t.Fatalf("stack depth = %d, want 2", got)
	}
	top, _ := a.dlg.top()
	if !top.modal || top.size != dlgLarge {
		t.Fatalf("top = %+v, want modal dlgLarge", top)
	}
	a.closeTopModal()
	if len(a.dlg.items) != 1 || strings.Join(closed, ",") != "second" {
		t.Fatalf("closeTopModal: depth=%d closed=%v, want 1/[second]", len(a.dlg.items), closed)
	}
	a.closeTopModal()
	if len(a.dlg.items) != 0 || strings.Join(closed, ",") != "second,first" {
		t.Fatalf("second close: depth=%d closed=%v", len(a.dlg.items), closed)
	}
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed = append(closed, "old") })
	a.replaceModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed = append(closed, "new") })
	if len(a.dlg.items) != 1 {
		t.Fatalf("replaceModal: depth=%d, want 1", len(a.dlg.items))
	}
	if top, _ = a.dlg.top(); top.size != dlgMedium || strings.Join(closed, ",") != "second,first,old" {
		t.Fatalf("replaceModal top/closed = %v/%v", top.size, closed)
	}
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed = append(closed, "c2") })
	a.clearModals()
	if len(a.dlg.items) != 0 || strings.Join(closed, ",") != "second,first,old,c2,new" {
		t.Fatalf("clearModals: depth=%d closed=%v", len(a.dlg.items), closed)
	}
	// non-modal items are untouched by the modal ops
	a.dlg.push(dialog{kind: dlgQuit})
	a.clearModals()
	if len(a.dlg.items) != 1 {
		t.Fatalf("clearModals must keep non-modal items: %+v", a.dlg.items)
	}
	if d, _ := a.dlg.top(); d.kind != dlgQuit || d.modal {
		t.Fatalf("survivor = %+v, want non-modal dlgQuit", d)
	}
}

func TestModalEscAndCtrlCCloseTop(t *testing.T) {
	a := testApp()
	closed := 0
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed++ })
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, func(*App) { closed++ })
	a.handleKey(press(tea.KeyEscape))
	if len(a.dlg.items) != 1 || closed != 1 {
		t.Fatalf("esc: depth=%d closed=%d, want 1/1", len(a.dlg.items), closed)
	}
	a.handleKey(ctrlCKey)
	if len(a.dlg.items) != 0 || closed != 2 {
		t.Fatalf("ctrl+c: depth=%d closed=%d, want 0/2", len(a.dlg.items), closed)
	}
}

func TestModalInnerCancelEscFirst(t *testing.T) {
	a := testApp()
	mdl := &modelDlg{hasSubChoice: true}
	a.pushModal(dialog{kind: dlgModel, model: mdl}, dlgMedium, nil)
	a.handleKey(press(tea.KeyEscape))
	if mdl.hasSubChoice {
		t.Fatalf("first esc must close the subchoice")
	}
	if len(a.dlg.items) != 1 {
		t.Fatalf("subchoice esc must keep the dialog: depth=%d, want 1", len(a.dlg.items))
	}
	a.handleKey(press(tea.KeyEscape))
	if len(a.dlg.items) != 0 {
		t.Fatalf("second esc must close the dialog: depth=%d", len(a.dlg.items))
	}
}

func TestModalFrameLayout(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	a.route = routeHome
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, nil)
	lines := strings.Split(a.view(), "\n")
	if len(lines) != 24 {
		t.Fatalf("frame = %d lines, want 24", len(lines))
	}
	// panel: medium 60, lead (80-60)/2 = 10; home chrome = logo 4 + New 1 +
	// rows 0 + divider 1 + help 1 = 7 > 24/4 = 6 → panelTop = 7; the panel
	// top-padding line → "Model" on line 8, "  loading…" on line 9.
	if want := strings.Repeat(" ", 10) + "Model"; !strings.HasPrefix(stripANSI(lines[8]), want) {
		t.Fatalf("line 8 = %q, want prefix %q", stripANSI(lines[8]), want)
	}
	if want := strings.Repeat(" ", 10) + "  loading…"; !strings.HasPrefix(lines[9], want) {
		t.Fatalf("line 9 = %q, want prefix %q", stripANSI(lines[9]), want)
	}
	// the prompt line is suppressed while a modal is open
	if strings.Contains(a.view(), "> ") {
		t.Fatalf("prompt must be hidden under the modal:\n%s", a.view())
	}
	// the footer stays on the last line
	if !strings.Contains(lines[23], "no model") {
		t.Fatalf("footer line = %q, want the home footer", stripANSI(lines[23]))
	}
}

func TestModalFrameSessionClamp(t *testing.T) {
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 10}
	a.route = routeSession
	a.pushModal(dialog{kind: dlgModel, model: &modelDlg{}}, dlgMedium, nil)
	lines := strings.Split(a.view(), "\n")
	if len(lines) != 10 {
		t.Fatalf("frame = %d lines, want 10", len(lines))
	}
	// session chrome min = title 1 + viewport 1 + divider 1 + help 1 = 4;
	// 10/4 = 2 < 4 → panelTop = 4; the panel starts at line 5 (padding line 4).
	if want := strings.Repeat(" ", 10) + "Model"; !strings.HasPrefix(stripANSI(lines[5]), want) {
		t.Fatalf("line 5 = %q, want prefix %q", stripANSI(lines[5]), want)
	}
	if !strings.Contains(lines[0], "session") {
		t.Fatalf("title line = %q, want the session title", stripANSI(lines[0]))
	}
}
