package tui

import (
	"regexp"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// sgrRe matches SGR escape sequences emitted by the locked styles so the
// whitebox layout test can compare plain text.
var sgrRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return sgrRe.ReplaceAllString(s, "") }

const testNow int64 = 1_000_000_000_000

func refModel(p, m string) *protocol.ModelRef {
	r := protocol.ModelRef{ProviderID: p, ID: m}
	return &r
}

func testApp(sessions ...protocol.Session) *recApp {
	a := newRecApp(client.New("http://127.0.0.1:9", ""), store.State{}, "")
	a.store.Sessions = sessions
	return a
}

func press(r rune) tea.KeyPressMsg {
	switch r {
	case tea.KeyUp, tea.KeyDown, tea.KeyEnter, tea.KeyEscape, tea.KeyLeft, tea.KeyRight, tea.KeyBackspace:
		return tea.KeyPressMsg{Code: r}
	}
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

var ctrlCKey = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

var (
	ctrlDKey = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	ctrlRKey = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
)

func pressTab() tea.KeyPressMsg { return tea.KeyPressMsg{Code: '\t'} }

func TestAppHandleKeyHome(t *testing.T) {
	// the 0.8.0 start screen has no session list: enter creates a new
	// session (the old cursor-0 "New session" path — Task 6 rewrites it to
	// the decision-2 submit), n creates, ctrl+c quits, /help opens help.
	// Up/down fall through to the prompt history recall (no cursor to move).

	t.Run("enter creates a session without opening", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.handleKey(press(tea.KeyEnter))
		if a.route != routeHome {
			t.Fatalf("route = %v, want routeHome (open happens on created msg)", a.route)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 create cmd", len(a.Cmds))
		}
	})

	t.Run("up/down fall through to the prompt input", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.input.SetValue("draft")
		a.handleKey(press(tea.KeyUp))
		// the draft is unchanged (no cursor to move; the empty history
		// recall is a no-op) and the prompt keeps focus.
		if got := a.prompt.input.Value(); got != "draft" {
			t.Fatalf("prompt = %q, want %q (up/down no longer moves a home cursor)", got, "draft")
		}
	})

	t.Run("n issues create cmd", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.handleKey(press('n'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 create cmd", len(a.Cmds))
		}
		if a.route != routeHome {
			t.Fatalf("route = %v, want routeHome", a.route)
		}
	})

	t.Run("ctrl+c opens quit dialog, y confirms, esc cancels", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.handleKey(ctrlCKey)
		if a.dlg.empty() {
			t.Fatal("quit dialog not opened")
		}
		a.handleKey(press('y'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 quit cmd", len(a.Cmds))
		}

		b := testApp()
		b.handleKey(ctrlCKey)
		b.handleKey(press(tea.KeyEscape))
		if !b.dlg.empty() {
			t.Fatal("dialog should be closed after esc")
		}
	})

	// T25 (deviation 52): the T23 auto-open command buffer is replaced by the
	// slash menu — typing "/help" opens the menu, enter executes it.
	t.Run("typing /help + enter opens help dialog", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.store.Commands = testCommands()
		for _, r := range "/help" {
			a.handleKey(press(r))
		}
		a.handleKey(press(tea.KeyEnter))
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgHelp {
			t.Fatalf("dialog = %v (ok=%v), want dlgHelp", d.kind, ok)
		}
	})
}

// TestInterruptMsgOpensQuitDialog pins SIGINT handling (cli-2): a
// tea.InterruptMsg delivered during Run is treated exactly like the ctrl+c
// keystroke — it opens the quit-confirm dialog.
func TestInterruptMsgOpensQuitDialog(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	a.Update(tea.InterruptMsg{})
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgQuit {
		t.Fatalf("after InterruptMsg dialog = %+v (ok=%v), want dlgQuit on top", d, ok)
	}
}
