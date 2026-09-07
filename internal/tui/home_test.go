package tui

import (
	"regexp"
	"testing"
	"time"

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
	// the 0.8.0 start screen has no session list (decision 2): enter with
	// non-empty text mints the session seeded with the pending agent +
	// model and sends the typed text as its first message; empty text is a
	// no-op; n still mints an EMPTY session (the server defaults);
	// ctrl+c quits, /help opens help. Up/down recall the prompt history
	// (no cursor to move).

	t.Run("enter with empty text is a no-op", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want 0 (empty text is a no-op)", len(a.Cmds))
		}
		if a.route != routeHome {
			t.Fatalf("route = %v, want routeHome", a.route)
		}
		// a whitespace-only line is also a no-op (the trimmed check) and
		// the input is kept for retry.
		a.prompt.input.SetValue("   ")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want 0 (whitespace-only text is a no-op)", len(a.Cmds))
		}
		if got := a.prompt.input.Value(); got != "   " {
			t.Fatalf("input = %q, want %q (kept for retry)", got, "   ")
		}
	})

	t.Run("trailing backslash soft-enters the draft", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.input.SetValue(`line1\`)
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want 0 (soft enter keeps typing)", len(a.Cmds))
		}
		if got := a.prompt.draft.String(); got != "line1\n" {
			t.Fatalf("draft = %q, want %q", got, "line1\n")
		}
		if got := a.prompt.input.Value(); got != "" {
			t.Fatalf("input = %q, want empty (soft-entered)", got)
		}
		a.prompt.input.SetValue("line2")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 (the draft + line submit)", len(a.Cmds))
		}
	})

	t.Run("up/down recall the prompt history (no cursor)", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.input.SetValue("draft")
		a.handleKey(press(tea.KeyUp))
		// the draft is unchanged (no cursor to move; the empty history
		// recall is a no-op) and the prompt keeps focus.
		if got := a.prompt.input.Value(); got != "draft" {
			t.Fatalf("prompt = %q, want %q (up with empty history is a no-op)", got, "draft")
		}
		// with history, up recalls the newest entry.
		a.hist = []string{"first", "second"}
		a.handleKey(press(tea.KeyUp))
		if got := a.prompt.input.Value(); got != "second" {
			t.Fatalf("prompt = %q, want %q (the newest history entry)", got, "second")
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

	t.Run("n mints an empty session with the server defaults", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		a := newRecApp(c, store.State{}, "")
		t.Cleanup(a.Close)
		a.handleKey(press('n'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 create cmd", len(a.Cmds))
		}
		msg := a.Cmds[0]()
		m, ok := msg.(sessionCreatedMsg)
		if !ok || m.err != nil {
			t.Fatalf("cmd delivered %v (%T), want a successful sessionCreatedMsg", m, msg)
		}
		// the empty-session path keeps the server defaults: the storage
		// "build" agent + the catalog default model.
		if m.ses.Agent != "build" {
			t.Fatalf("session agent = %q, want build", m.ses.Agent)
		}
		if m.ses.Model == nil || m.ses.Model.ProviderID != "kido" || m.ses.Model.ID != "q" {
			t.Fatalf("session model = %+v, want kido/q (the server catalog default)", m.ses.Model)
		}
	})

	t.Run("enter with text seeds the session and sends", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		a := newRecApp(c, store.State{}, "")
		t.Cleanup(a.Close)
		a.prompt.input.SetValue("hello submit")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 home submit cmd", len(a.Cmds))
		}
		msg := a.Cmds[0]()
		m, ok := msg.(homeSubmitMsg)
		if !ok {
			t.Fatalf("cmd delivered %T, want homeSubmitMsg", msg)
		}
		if m.err != nil {
			t.Fatalf("home submit failed: %v", m.err)
		}
		if m.ses.ID == "" || m.text != "hello submit" {
			t.Fatalf("submit msg = %+v, want the minted session + the typed text", m)
		}
		// the seeds: the pending agent default + the catalog default model
		// (no config model ref — the server applies the default on blank).
		if m.ses.Agent != "build" {
			t.Fatalf("session agent = %q, want build", m.ses.Agent)
		}
		if m.ses.Model == nil || m.ses.Model.ProviderID != "kido" || m.ses.Model.ID != "q" {
			t.Fatalf("session model = %+v, want kido/q (the server catalog default)", m.ses.Model)
		}
		if a.route != routeHome {
			t.Fatalf("route = %v before the apply, want routeHome", a.route)
		}
		_, cmd := a.Update(m)
		if a.route != routeSession {
			t.Fatalf("route = %v, want routeSession", a.route)
		}
		if a.curSessionID != m.ses.ID {
			t.Fatalf("curSessionID = %q, want %q", a.curSessionID, m.ses.ID)
		}
		if len(a.store.Sessions) != 1 || a.store.Sessions[0].ID != m.ses.ID {
			t.Fatalf("store.Sessions = %+v, want the minted session first", a.store.Sessions)
		}
		if got := a.prompt.input.Value(); got != "" {
			t.Fatalf("input = %q, want empty (cleared on success)", got)
		}
		if a.prompt.draft.Len() != 0 {
			t.Fatalf("draft = %q, want empty (reset on success)", a.prompt.draft.String())
		}
		// the hydrate leg: the message list holds the typed user message.
		if cmd == nil {
			t.Fatal("no hydrate cmd after the apply")
		}
		_, _ = a.Update(cmd())
		var userText string
		for _, mwp := range a.store.Messages {
			if mwp.Info.Role != "user" {
				continue
			}
			for _, p := range mwp.Parts {
				if p.Type == "text" {
					userText = p.Text
				}
			}
		}
		if userText != "hello submit" {
			t.Fatalf("user message text = %q, want %q", userText, "hello submit")
		}
	})

	t.Run("shell-mode enter mints and posts the shell command", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		a := newRecApp(c, store.State{}, "")
		t.Cleanup(a.Close)
		a.prompt.mode = "shell"
		a.prompt.input.SetValue("echo shell-home")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 home shell cmd", len(a.Cmds))
		}
		msg := a.Cmds[0]()
		m, ok := msg.(homeSubmitMsg)
		if !ok || m.err != nil {
			t.Fatalf("cmd delivered %v (%T), want a successful homeSubmitMsg", m, msg)
		}
		if a.route != routeHome {
			t.Fatalf("route = %v before the apply, want routeHome", a.route)
		}
		_, _ = a.Update(m)
		if a.route != routeSession || a.curSessionID != m.ses.ID {
			t.Fatalf("route = %v curSessionID = %q, want the minted session", a.route, a.curSessionID)
		}
		// the shell post landed (the Task 9 wire contract): the minted
		// session carries the user row + the assistant's bash tool part —
		// a message send would have no bash part. The exec runs in the
		// engine goroutine, so wait for the part to finalize over the wire
		// (the waitShellPart idiom via the client — the tui tests stay on
		// the wire contract), and close the lazily-spawned persistent
		// shell so its readLoop does not outlive the test.
		t.Cleanup(func() { ts.Eng.Close(m.ses.ID) })
		deadline := time.Now().Add(5 * time.Second)
		for {
			msgs, err := c.ListMessages(t.Context(), m.ses.ID)
			if err != nil {
				t.Fatalf("ListMessages: %v", err)
			}
			var sawUser, sawBash, terminal bool
			for _, mwp := range msgs {
				if mwp.Info.Role == "user" {
					sawUser = true
				}
				for _, p := range mwp.Parts {
					if p.Type != "tool" || p.Tool != "bash" {
						continue
					}
					sawBash = true
					if p.State != nil && (p.State.Status == "completed" || p.State.Status == "error") {
						terminal = true
					}
				}
			}
			if sawUser && sawBash && terminal {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("shell part did not finalize (sawUser=%v sawBash=%v): %+v", sawUser, sawBash, msgs)
			}
			time.Sleep(5 * time.Millisecond)
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
