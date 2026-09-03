package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

func openRetryDlg() *recApp {
	a := testApp()
	a.openRetryActionDialog("Request failed", "upstream overloaded (retrying, attempt 1)", "Abort")
	return a
}

func TestRetryDialogRender(t *testing.T) {
	a := openRetryDlg()
	got := stripANSI(a.dlg.retryAction().view(80, 24))
	for _, tok := range []string{
		"Request failed",
		"esc",
		"upstream overloaded (retrying, attempt 1)",
		"don't show again",
		"Abort",
	} {
		if !strings.Contains(got, tok) {
			t.Fatalf("token %q missing:\n%s", tok, got)
		}
	}
}

func TestRetryDialogKeys(t *testing.T) {
	t.Run("starts selected on the action; left/right/tab toggle", func(t *testing.T) {
		a := openRetryDlg()
		if a.dlg.retryAction().selected != 1 {
			t.Fatalf("starts selected = %d, want 1 (the action)", a.dlg.retryAction().selected)
		}
		a.handleKey(press(tea.KeyLeft))
		if a.dlg.retryAction().selected != 0 {
			t.Fatalf("left: selected = %d, want 0", a.dlg.retryAction().selected)
		}
		a.handleKey(press(tea.KeyRight))
		a.handleKey(pressTab())
		if a.dlg.retryAction().selected != 0 {
			t.Fatalf("right then tab: selected = %d, want 0", a.dlg.retryAction().selected)
		}
	})

	t.Run("enter-action aborts and closes", func(t *testing.T) {
		a := openRetryDlg()
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() || len(a.Cmds) != 1 {
			t.Fatalf("the action must abort + close: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})

	t.Run("enter-dismiss closes without aborting", func(t *testing.T) {
		a := openRetryDlg()
		a.handleKey(press(tea.KeyLeft))
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() || len(a.Cmds) != 0 {
			t.Fatalf("the dismiss must close silently: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})

	t.Run("esc dismisses", func(t *testing.T) {
		a := openRetryDlg()
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() || len(a.Cmds) != 0 {
			t.Fatalf("esc must dismiss: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})
}

func TestRetryTransitionHook(t *testing.T) {
	ev := func(next string, attempt int) protocol.Event {
		props, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
			SessionID: "s1",
			Status:    protocol.SessionStatus{Type: next, Attempt: attempt, Message: "upstream overloaded"},
		})
		return props
	}

	t.Run("idle -> retry on the current session opens the dialog once", func(t *testing.T) {
		a := testApp()
		a.curSessionID = "s1"
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		a.onSessionStatus("idle", ev("retry", 1))
		top, ok := a.dlg.top()
		if !ok || top.kind != dlgRetryAction {
			t.Fatalf("top = %v, want dlgRetryAction", top.kind)
		}
		// a second idle->retry for the same session is suppressed (per-run)
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		a.onSessionStatus("idle", ev("retry", 2))
		if n := len(a.dlg.items); n != 1 {
			t.Fatalf("the suppression leaked: depth = %d, want 1", n)
		}
	})

	t.Run("other session / other transitions do not open", func(t *testing.T) {
		a := testApp()
		a.curSessionID = "s1"
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		// a different session
		other := protocol.SessionStatusProps{SessionID: "s2", Status: protocol.SessionStatus{Type: "retry"}}
		evOther, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, other)
		a.onSessionStatus("idle", evOther)
		if !a.dlg.empty() {
			t.Fatal("a non-current session must not open the dialog")
		}
		// busy -> retry is not the idle->retry transition
		a.store.Status = protocol.SessionStatus{Type: "busy"}
		a.onSessionStatus("busy", ev("retry", 1))
		if !a.dlg.empty() {
			t.Fatal("busy->retry must not open the dialog")
		}
	})

	t.Run("the suppression clears on the next send", func(t *testing.T) {
		a := testApp()
		a.curSessionID = "s1"
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		a.onSessionStatus("idle", ev("retry", 1))
		a.handleKey(press(tea.KeyLeft)) // dismiss
		a.handleKey(press(tea.KeyEnter))
		a.applySend(sendMsg{}) // the next send clears the suppression
		a.store.Status = protocol.SessionStatus{Type: "idle"}
		a.onSessionStatus("idle", ev("retry", 2))
		if a.dlg.empty() {
			t.Fatal("the cleared suppression must allow the dialog again")
		}
	})
}
