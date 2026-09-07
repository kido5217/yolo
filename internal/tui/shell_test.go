package tui

// shell_test.go — 0.8.0 Task 10 (shell: TUI mode): the `!` toggle (normal
// mode, cursor offset 0 — the value kept), the shell-mode exits (esc /
// backspace at offset 0), the placeholder swap on toggle/exit (the index
// re-rolls on toggle, persists on exit), the homeMeta shell branch ("Shell"
// alone), and the submit paths (home mints first — homeShellCmd; session
// direct — shellCmd). Whitebox key handling + real-stack submits.

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// TestShellModeToggle pins the `!` toggle-in and the shell-mode exits
// (whitebox, both routes): the toggle switches the mode + re-rolls the
// placeholder over the shell pool (the value kept); esc + backspace-at-0 exit
// back to normal (the normal placeholder restored, NO re-roll); a `!` at a
// non-zero cursor or in shell mode is INSERTED.
func TestShellModeToggle(t *testing.T) {
	t.Parallel()

	t.Run("`!` at offset 0 in normal toggles to shell (empty value kept)", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		if a.prompt.mode != "normal" {
			t.Fatalf("initial mode = %q, want normal", a.prompt.mode)
		}
		a.handleKey(press('!'))
		if a.prompt.mode != "shell" {
			t.Fatalf("mode = %q, want shell", a.prompt.mode)
		}
		// the value is kept (empty stays empty) and the cursor stays at 0.
		if got := a.prompt.input.Value(); got != "" {
			t.Fatalf("input = %q, want empty (value kept)", got)
		}
		if a.prompt.input.Position() != 0 {
			t.Fatalf("cursor = %d, want 0 (offset 0 referent)", a.prompt.input.Position())
		}
		// the placeholder re-rolls over the shell pool (the applyPromptChrome
		// swap): the home-route placeholder is now a shell-pool entry.
		if a.prompt.input.Placeholder != a.prompt.placeholderText() {
			t.Fatalf("placeholder = %q, want the active-mode placeholder %q", a.prompt.input.Placeholder, a.prompt.placeholderText())
		}
		for _, p := range placeholderShell {
			if a.prompt.input.Placeholder == p {
				return
			}
		}
		t.Fatalf("placeholder = %q, want a shell-pool entry", a.prompt.input.Placeholder)
	})

	t.Run("`!` at offset 0 in normal toggles to shell (non-empty value kept)", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.input.SetValue("ls -la")
		a.prompt.input.SetCursor(0) // the value is kept; the cursor-0 referent
		a.handleKey(press('!'))
		if a.prompt.mode != "shell" {
			t.Fatalf("mode = %q, want shell", a.prompt.mode)
		}
		if got := a.prompt.input.Value(); got != "ls -la" {
			t.Fatalf("input = %q, want %q (value kept)", got, "ls -la")
		}
		if a.prompt.input.Position() != 0 {
			t.Fatalf("cursor = %d, want 0 (the value becomes the command)", a.prompt.input.Position())
		}
	})

	t.Run("`!` at a non-zero cursor is inserted (not a toggle)", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.input.SetValue("ls")
		if a.prompt.input.Position() != 2 {
			t.Fatalf("cursor = %d, want 2 (end of value)", a.prompt.input.Position())
		}
		a.handleKey(press('!'))
		if a.prompt.mode != "normal" {
			t.Fatalf("mode = %q, want normal (no toggle at a non-zero cursor)", a.prompt.mode)
		}
		if got := a.prompt.input.Value(); got != "ls!" {
			t.Fatalf("input = %q, want %q (the ! inserted)", got, "ls!")
		}
	})

	t.Run("`!` in shell mode is inserted (not a toggle)", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.mode = "shell"
		a.handleKey(press('!'))
		if a.prompt.mode != "shell" {
			t.Fatalf("mode = %q, want shell (no toggle in shell mode)", a.prompt.mode)
		}
		if got := a.prompt.input.Value(); got != "!" {
			t.Fatalf("input = %q, want %q (the ! inserted)", got, "!")
		}
	})

	t.Run("esc in shell (home) exits to normal (placeholder restored, no re-roll)", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.mode = "shell"
		idx := a.prompt.placeholderIdx // pin the index: exit must keep it
		a.handleKey(press(tea.KeyEscape))
		if a.prompt.mode != "normal" {
			t.Fatalf("mode = %q, want normal", a.prompt.mode)
		}
		if got := a.prompt.placeholderIdx; got != idx {
			t.Fatalf("placeholderIdx = %d, want %d (no re-roll on exit)", got, idx)
		}
		// the placeholder is the NORMAL pool entry at the (unchanged) index.
		want := placeholderNormal[idx%len(placeholderNormal)]
		if got := a.prompt.input.Placeholder; got != want {
			t.Fatalf("placeholder = %q, want the normal pool at index %d (%q)", got, idx%len(placeholderNormal), want)
		}
	})

	t.Run("backspace at offset 0 in shell (home) exits to normal", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.mode = "shell"
		a.handleKey(press(tea.KeyBackspace))
		if a.prompt.mode != "normal" {
			t.Fatalf("mode = %q, want normal", a.prompt.mode)
		}
	})

	t.Run("backspace at offset 0 in normal is a no-op (mode unchanged)", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.handleKey(press(tea.KeyBackspace))
		if a.prompt.mode != "normal" {
			t.Fatalf("mode = %q, want normal (backspace at 0 in normal is a no-op)", a.prompt.mode)
		}
	})

	t.Run("esc in shell (session) exits to normal (route unchanged)", func(t *testing.T) {
		t.Parallel()
		a := testSessionApp(sessionFixture())
		a.prompt.mode = "shell"
		a.handleKey(press(tea.KeyEscape))
		if a.prompt.mode != "normal" {
			t.Fatalf("mode = %q, want normal", a.prompt.mode)
		}
		// the session route does NOT return home (the esc exits the shell,
		// not the route) — it also must NOT abort.
		if a.route != routeSession || a.curSessionID != "ses_0" {
			t.Fatalf("route=%v cur=%s, want routeSession/ses_0 (esc in shell does not return home)", a.route, a.curSessionID)
		}
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want 0 (no abort on the shell esc exit)", len(a.Cmds))
		}
	})

	t.Run("backspace at offset 0 in shell (session) exits to normal", func(t *testing.T) {
		t.Parallel()
		a := testSessionApp(sessionFixture())
		a.prompt.mode = "shell"
		a.handleKey(press(tea.KeyBackspace))
		if a.prompt.mode != "normal" {
			t.Fatalf("mode = %q, want normal", a.prompt.mode)
		}
	})
}

// TestHomeMetaShell pins the homeMeta shell branch (Task 10 Step 4): shell
// mode returns ("Shell", "", "") — the model/provider box is inside the
// normal-mode Show (upstream prompt/index.tsx:1450).
func TestHomeMetaShell(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.store = metaCatalog()
	a.store.Config = map[string]any{"model": "kido/q"}
	a.prompt.mode = "shell"
	agent, model, provider := a.homeMeta()
	if agent != "Shell" || model != "" || provider != "" {
		t.Fatalf("homeMeta (shell) = (%q, %q, %q), want (%q, %q, %q)", agent, model, provider, "Shell", "", "")
	}
	// the meta line renders the "Shell" label alone (the model/provider
	// segments are dropped in shell mode), width-exact at the interior width
	// (the zero theme degrades the runs to plain text).
	innerW := a.boxInnerWidth()
	want := "Shell" + strings.Repeat(" ", innerW-len("Shell"))
	if got := stripANSI(a.boxMetaLine()); got != want {
		t.Fatalf("boxMetaLine (shell) = %q, want %q (the Shell label + width-exact fill)", got, want)
	}
}

// TestShellModeSubmit pins the shell submit paths (real stack): the home
// route mints the session (seeded) then shells (homeShellCmd); the session
// route posts to the CURRENT session (shellCmd, no mint). Both toggle shell
// mode via the `!` key (the Task-10 toggle) and wait for the bash tool part
// to finalize over the wire.
func TestShellModeSubmit(t *testing.T) {
	t.Parallel()

	t.Run("home shell submit (toggled via `!`) mints and shells", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		a := newRecApp(c, store.State{}, "")
		t.Cleanup(a.Close)
		a.handleKey(press('!')) // the Task-10 toggle (not a direct mode set)
		if a.prompt.mode != "shell" {
			t.Fatalf("mode = %q, want shell (the ! toggle)", a.prompt.mode)
		}
		a.prompt.input.SetValue("echo shell-home-t10")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 home shell cmd", len(a.Cmds))
		}
		msg := a.Cmds[0]()
		m, ok := msg.(homeSubmitMsg)
		if !ok || m.err != nil {
			t.Fatalf("cmd delivered %v (%T), want a successful homeSubmitMsg", m, msg)
		}
		if m.text != "echo shell-home-t10" {
			t.Fatalf("submit text = %q, want the typed command", m.text)
		}
		if a.route != routeHome {
			t.Fatalf("route = %v before the apply, want routeHome", a.route)
		}
		_, _ = a.Update(m)
		if a.route != routeSession || a.curSessionID != m.ses.ID {
			t.Fatalf("route = %v curSessionID = %q, want the minted session", a.route, a.curSessionID)
		}
		// the shell post landed: the minted session carries the user row +
		// the assistant's bash tool part (finalized `completed` — a message
		// send would have no bash part). Wait for the part over the wire and
		// close the lazily-spawned persistent shell so its readLoop does not
		// outlive the test.
		t.Cleanup(func() { ts.Eng.Close(m.ses.ID) })
		waitShellPartCompleted(t, c, m.ses.ID)
	})

	t.Run("session shell submit (no mint) posts to the current session", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		ctx := t.Context()
		ses, err := c.CreateSession(ctx, "")
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		t.Cleanup(func() { ts.Eng.Close(ses.ID) })
		a := newRecApp(c, store.State{}, "")
		t.Cleanup(a.Close)
		a.openSession(ses.ID)
		if a.route != routeSession || a.curSessionID != ses.ID {
			t.Fatalf("route=%v cur=%s, want routeSession/%s", a.route, a.curSessionID, ses.ID)
		}
		a.handleKey(press('!')) // the Task-10 toggle
		if a.prompt.mode != "shell" {
			t.Fatalf("mode = %q, want shell (the ! toggle)", a.prompt.mode)
		}
		a.prompt.input.SetValue("echo shell-sess-t10")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 session shell cmd", len(a.Cmds))
		}
		msg := a.Cmds[0]()
		m, ok := msg.(shellMsg)
		if !ok || m.err != nil {
			t.Fatalf("cmd delivered %v (%T), want a successful shellMsg", m, msg)
		}
		if m.text != "echo shell-sess-t10" {
			t.Fatalf("submit text = %q, want the typed command", m.text)
		}
		// the session route does NOT mint — the current session is kept.
		_, _ = a.Update(m)
		if a.curSessionID != ses.ID {
			t.Fatalf("curSessionID = %q, want %q (no mint on the session route)", a.curSessionID, ses.ID)
		}
		// the input clears on success (applyShell post-send state).
		if got := a.prompt.input.Value(); got != "" {
			t.Fatalf("input = %q, want empty (cleared on success)", got)
		}
		// the current session carries the user row + the assistant's bash
		// tool part (finalized `completed`).
		waitShellPartCompleted(t, c, ses.ID)
	})
}

// waitShellPartCompleted polls the session's message list until it carries a
// user row + a bash tool part that has reached a terminal state (the
// waitShellPart idiom via the client — the TUI tests stay on the wire
// contract).
func waitShellPartCompleted(t *testing.T, c *client.Service, sessionID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		msgs, err := c.ListMessages(t.Context(), sessionID)
		if err != nil {
			t.Fatalf("ListMessages: %v", err)
		}
		var sawUser, sawBash, completed bool
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
					completed = true
				}
			}
		}
		if sawUser && sawBash && completed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("shell part did not finalize (sawUser=%v sawBash=%v): %+v", sawUser, sawBash, msgs)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
