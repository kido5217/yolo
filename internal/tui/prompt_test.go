package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
)

// testCommands mirrors the server's locked command set (T20).
func testCommands() []protocol.Command {
	return []protocol.Command{
		{Name: "/help", Description: "show help"},
		{Name: "/new", Description: "new session"},
		{Name: "/model", Description: "pick model"},
		{Name: "/agents", Description: "pick agent"},
		{Name: "/quit", Description: "exit"},
	}
}

// pressAlt builds an alt-modified keypress (the T25 rebound expand/think keys).
func pressAlt(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Mod: tea.ModAlt} }

func typeStr(a *recApp, s string) {
	for _, r := range s {
		a.handleKey(press(r))
	}
}

func hasToast(a *recApp, msg string) bool {
	for _, t := range a.toasts {
		if t.msg == msg {
			return true
		}
	}
	return false
}

func TestPromptMenuFilter(t *testing.T) {
	tests := []struct {
		in   string
		want []string // nil = menu closed
	}{
		{"", nil},
		{"hello", nil},
		{"/", []string{"/sessions", "/connect", "/status", "/themes", "/help", "/new", "/model", "/agents", "/quit"}},
		{"/model", []string{"/model"}},
		{"/quit", []string{"/quit"}},
		{"/zz", []string{}},
	}
	for _, tt := range tests {
		t.Run("in="+tt.in, func(t *testing.T) {
			a := testApp()
			a.store.Commands = testCommands()
			a.prompt.input.SetValue(tt.in)
			got := a.menuItems()
			gotNames := []string(nil)
			if got != nil {
				gotNames = make([]string, 0, len(got))
				for _, c := range got {
					gotNames = append(gotNames, c.Name)
				}
			}
			if len(gotNames) != len(tt.want) {
				t.Fatalf("in=%q got %v, want %v", tt.in, gotNames, tt.want)
			}
			for i := range tt.want {
				if gotNames[i] != tt.want[i] {
					t.Fatalf("in=%q got %v, want %v", tt.in, gotNames, tt.want)
				}
			}
		})
	}
}

func TestPromptMenuFuzzy(t *testing.T) {
	a := testApp()
	a.store.Commands = testCommands()
	// "m" is a prefix of "model" and a subsequence of "themes": the prefix
	// match (x2 boost) ranks first.
	a.prompt.input.SetValue("/m")
	got := a.menuItems()
	if len(got) == 0 {
		t.Fatal("no matches for /m")
	}
	if got[0].Name != "/model" {
		t.Fatalf("top match = %q, want /model (the prefix boost)", got[0].Name)
	}
	// the alias is preserved: /exit maps to the canonical /quit
	a.prompt.input.SetValue("/exit")
	got = a.menuItems()
	if len(got) == 0 || got[0].Name != "/quit" {
		t.Fatalf("alias /exit -> /quit, got %v", got)
	}
}

// TestPromptMenuOpenBox pins the S2 rewire: the open slash menu (input "/")
// renders the shared bordered dropdown (spec §6 S2) — the box chrome (the
// split border, the backgroundMenu fill) wrapping the items, the selection
// row SGR'd with the primary bg + the SelectedForeground (yolo dark tokens).
func TestPromptMenuOpenBox(t *testing.T) {
	th := yoloDarkTheme(t)
	a := testApp()
	a.store.Commands = testCommands()
	a.prompt.input.SetValue("/")
	items := a.menuItems()
	if len(items) != 9 {
		t.Fatalf("items = %d, want 9 (the 4 locals + the 5 catalog)", len(items))
	}
	const w = 60
	got := a.prompt.menuView(items, w, th)
	lines := strings.Split(got, "\n")
	if len(lines) != 9 {
		t.Fatalf("box = %d rows, want 9:\n%s", len(lines), got)
	}
	for i, l := range lines {
		plain := stripANSI(l)
		if n := len([]rune(plain)); n != w {
			t.Fatalf("row %d = %d cols, want %d (the width-exact box): %q", i, n, w, plain)
		}
		if plain[0] != '|' || plain[w-1] != '|' {
			t.Fatalf("row %d = %q, want the split left/right border", i, plain)
		}
		if plain[1] != ' ' {
			t.Fatalf("row %d = %q, want 1 padding after the left border", i, plain)
		}
	}
	// The two-column layout (label + the description right-offset by "  ").
	if !strings.Contains(stripANSI(lines[0]), "/sessions  List all sessions") {
		t.Fatalf("row 0 lost the two-column layout: %q", stripANSI(lines[0]))
	}
	// The unselected row chrome (yolo dark tokens, the S1 pins): the border
	// fg #484848, the backgroundMenu fill #1e1e1e, the label #eeeeee, the
	// description #808080.
	if u := lines[1]; !strings.Contains(u, "38;2;72;72;72") ||
		!strings.Contains(u, "48;2;30;30;30") ||
		!strings.Contains(u, "38;2;238;238;238") ||
		!strings.Contains(u, "38;2;128;128;128") {
		t.Fatalf("row 1 lost the border/fill/label/description SGR: %s", u)
	}
	// The selection row (sel = 0): the primary bg (#fab283) + the
	// SelectedForeground (#0a0a0a) across the full row width.
	if sel := lines[0]; !strings.Contains(sel, "48;2;250;178;131") {
		t.Fatalf("selection row missing the primary bg SGR (#fab283): %s", sel)
	}
	if !strings.Contains(lines[0], "38;2;10;10;10") {
		t.Fatalf("selection row missing the SelectedForeground SGR (#0a0a0a): %s", lines[0])
	}
	if strings.Contains(lines[1], "48;2;250;178;131") {
		t.Fatalf("unselected row carries the primary bg: %s", lines[1])
	}
}

// TestPromptMenuEmptyLine pins the S2 empty-filter case (input "/zzz"): the
// no-match line renders with the CURRENT text (the "No matching items" text
// lands in S7, which re-baselines this pin).
func TestPromptMenuEmptyLine(t *testing.T) {
	th := yoloDarkTheme(t)
	a := testApp()
	a.store.Commands = testCommands()
	a.prompt.input.SetValue("/zzz")
	items := a.menuItems()
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %v, want an open menu with no match", items)
	}
	got := a.prompt.menuView(items, 60, th)
	lines := strings.Split(got, "\n")
	if len(lines) != 1 {
		t.Fatalf("empty filter = %d lines, want 1 (the no-match line):\n%s", len(lines), got)
	}
	if !strings.Contains(stripANSI(lines[0]), "no match") {
		t.Fatalf("no-match line lost the current text: %q", stripANSI(lines[0]))
	}
	if !strings.Contains(lines[0], "38;2;128;128;128") {
		t.Fatalf("no-match line missing the textMuted fg SGR (#808080): %s", lines[0])
	}
}

func TestPromptMenuKeys(t *testing.T) {
	t.Run("arrows move the selection while the menu is open", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		typeStr(a, "/")
		if !a.prompt.slashActive() {
			t.Fatal("menu must be open for \"/\"")
		}
		a.handleKey(press(tea.KeyDown))
		if a.prompt.sel != 1 {
			t.Fatalf("sel = %d after down, want 1", a.prompt.sel)
		}
		a.handleKey(press(tea.KeyDown))
		if a.prompt.sel != 2 {
			t.Fatalf("sel = %d, want 2", a.prompt.sel)
		}
		a.handleKey(press(tea.KeyUp))
		if a.prompt.sel != 1 {
			t.Fatalf("sel = %d after up, want 1", a.prompt.sel)
		}
		// 9 items (the S3.1/S3.4/S3.5/S3.8 local merge adds /sessions +
		// /connect + /status + /themes): down from 1 wraps after item 8
		for i := 0; i < 8; i++ {
			a.handleKey(press(tea.KeyDown))
		}
		if a.prompt.sel != 0 {
			t.Fatalf("sel = %d after wrap, want 0", a.prompt.sel)
		}
	})

	t.Run("enter with no match clears the input (locked)", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		typeStr(a, "/zz")
		if !a.prompt.slashActive() || len(a.menuItems()) != 0 {
			t.Fatal("menu must be open with no match")
		}
		a.handleKey(press(tea.KeyEnter))
		if a.prompt.input.Value() != "" {
			t.Fatalf("input = %q, want cleared", a.prompt.input.Value())
		}
	})

	t.Run("esc closes the menu by clearing the input", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		typeStr(a, "/m")
		a.handleKey(press(tea.KeyEscape))
		if a.prompt.input.Value() != "" {
			t.Fatalf("input = %q, want cleared", a.prompt.input.Value())
		}
	})

	t.Run("enter executes the selected command", func(t *testing.T) {
		tests := []struct {
			in   string
			want dialogKind
		}{
			{"/help", dlgHelp},
			{"/quit", dlgQuit},
			{"/exit", dlgQuit}, // alias of /quit
			{"/model", dlgModel},
			{"/agents", dlgAgents},
		}
		for _, tt := range tests {
			t.Run(tt.in, func(t *testing.T) {
				a := testApp()
				a.store.Commands = testCommands()
				typeStr(a, tt.in)
				a.handleKey(press(tea.KeyEnter))
				d, ok := a.dlg.top()
				if !ok || d.kind != tt.want {
					t.Fatalf("dialog = %v (ok=%v), want %v", d.kind, ok, tt.want)
				}
				if a.prompt.input.Value() != "" {
					t.Fatalf("input = %q, want cleared after executing", a.prompt.input.Value())
				}
			})
		}
	})
}

// TestPromptMenuTabComplete pins the S6 key behavior (spec §6 S6, §3.2):
// tab with the menu open completes the selected command (the whole input
// becomes the name + a trailing space, the menu closes, the command does
// NOT run); tab closed cycles the agent (deviation 289's closed-menu
// behavior); shift+tab cycles the agent reverse with the menu staying open;
// a typed query change (or backspace) resets sel to 0. The esc-open clear is
// pinned by TestPromptMenuKeys' esc leg (kept).
func TestPromptMenuTabComplete(t *testing.T) {
	t.Run("tab open: the input is the name + a space, the menu closes, the command does not run", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		typeStr(a, "/new")
		if !a.prompt.slashActive() {
			t.Fatal("menu must be open for \"/new\"")
		}
		items := a.menuItems()
		if len(items) != 1 || items[0].Name != "/new" {
			t.Fatalf("items = %v, want [/new]", items)
		}
		if !strings.Contains(a.view(), "|") {
			t.Fatal("the open menu must render the bordered dropdown (the frame pin)")
		}
		a.handleKey(pressTab())
		if got := a.prompt.input.Value(); got != "/new " {
			t.Fatalf("input = %q, want \"/new \" (the selected name + a trailing space)", got)
		}
		if a.prompt.slashActive() {
			t.Fatal("the menu must close on tab (the value still starts with \"/\")")
		}
		if strings.Contains(a.view(), "|") {
			t.Fatal("the dropdown must no longer render after the tab completion (the frame pin)")
		}
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want 0 (tab completes, it does NOT run)", len(a.Cmds))
		}
		if a.route != routeHome {
			t.Fatalf("route = %v, want routeHome (no session mint)", a.route)
		}
		// the next edit re-opens the menu (the backspace deletes the space).
		a.handleKey(press(tea.KeyBackspace))
		if got := a.prompt.input.Value(); got != "/new" {
			t.Fatalf("input = %q, want \"/new\" after the backspace", got)
		}
		if !a.prompt.slashActive() {
			t.Fatal("the menu must re-open on the next edit")
		}
		if !strings.Contains(a.view(), "|") {
			t.Fatal("the dropdown must render again after the next edit (the frame pin)")
		}
	})

	t.Run("tab closed: the agent cycles (unchanged)", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agentFixture()
		a.prompt.input.SetValue("hello")
		a.handleKey(pressTab())
		if got := a.pendingAgent; got != "plan" {
			t.Fatalf("after tab = %q, want plan (the closed-menu agent cycle)", got)
		}
		if got := a.prompt.input.Value(); got != "hello" {
			t.Fatalf("input = %q, want hello (the tab is consumed, not inserted)", got)
		}
	})

	t.Run("shift+tab open: the agent cycles reverse, the menu stays open", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agentFixture()
		typeStr(a, "/")
		if !a.prompt.slashActive() {
			t.Fatal("menu must be open for \"/\"")
		}
		a.handleKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		if got := a.pendingAgent; got != "yolo" {
			t.Fatalf("after shift+tab = %q, want yolo (the reverse wrap from build)", got)
		}
		if got := a.prompt.input.Value(); got != "/" {
			t.Fatalf("input = %q, want \"/\" (the menu stays open)", got)
		}
		if !a.prompt.slashActive() {
			t.Fatal("the menu must stay open on shift+tab")
		}
		if !strings.Contains(a.view(), "|") {
			t.Fatal("the dropdown must still render after shift+tab (the frame pin)")
		}
	})

	t.Run("a typed query change resets sel to 0 (backspace too)", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		typeStr(a, "/")
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyDown))
		if a.prompt.sel != 2 {
			t.Fatalf("sel = %d after two downs, want 2", a.prompt.sel)
		}
		typeStr(a, "o") // the query change: value "/o"
		if got := a.prompt.input.Value(); got != "/o" {
			t.Fatalf("input = %q, want \"/o\"", got)
		}
		if a.prompt.sel != 0 {
			t.Fatalf("sel = %d after the query change, want 0 (the upstream reset)", a.prompt.sel)
		}
		if len(a.menuItems()) < 2 {
			t.Fatalf("items for /o = %v, want >= 2 for the backspace leg", a.menuItems())
		}
		a.handleKey(press(tea.KeyDown))
		if a.prompt.sel != 1 {
			t.Fatalf("sel = %d after down, want 1", a.prompt.sel)
		}
		a.handleKey(press(tea.KeyBackspace)) // the value change: "/o" -> "/"
		if got := a.prompt.input.Value(); got != "/" {
			t.Fatalf("input = %q, want \"/\" after the backspace", got)
		}
		if a.prompt.sel != 0 {
			t.Fatalf("sel = %d after the backspace, want 0", a.prompt.sel)
		}
	})
}

func TestPromptQuitAlias(t *testing.T) {
	for _, in := range []string{"/quit", "/exit"} {
		t.Run(in, func(t *testing.T) {
			a := testApp()
			a.runCommand(in)
			d, ok := a.dlg.top()
			if !ok || d.kind != dlgQuit {
				t.Fatalf("dialog = %v (ok=%v), want dlgQuit", d.kind, ok)
			}
		})
	}
}

func TestPromptNewCommand(t *testing.T) {
	t.Run("no current session: /new issues the create cmd (locked)", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		typeStr(a, "/new")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 create cmd", len(a.Cmds))
		}
		if a.route != routeHome {
			t.Fatalf("route = %v, want routeHome (switch happens on sessionCreatedMsg)", a.route)
		}
		if a.prompt.input.Value() != "" {
			t.Fatal("input must clear")
		}
	})

	t.Run("with a current session: /new issues the command cmd", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.store.Commands = testCommands()
		typeStr(a, "/new")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 command cmd", len(a.Cmds))
		}
		if a.prompt.input.Value() != "" {
			t.Fatal("input must clear")
		}
	})

	t.Run("command response with session_id switches and hydrates", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.Update(commandExecMsg{resp: protocol.CommandResponse{SessionID: "ses_9"}})
		if a.route != routeSession || a.curSessionID != "ses_9" {
			t.Fatalf("route=%v cur=%s, want routeSession/ses_9", a.route, a.curSessionID)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 hydrate cmd", len(a.Cmds))
		}
	})

	t.Run("command error toasts, stays put", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.Update(commandExecMsg{err: errors.New("nope")})
		if !hasToast(a, "nope") {
			t.Fatalf("toasts = %v, want nope", a.toasts)
		}
		if a.curSessionID != "ses_0" {
			t.Fatalf("cur = %s, want unchanged", a.curSessionID)
		}
	})
}

func TestPromptSend(t *testing.T) {
	t.Run("enter sends and success clears the input", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		typeStr(a, "hello")
		if a.prompt.input.Value() != "hello" {
			t.Fatalf("value = %q, want hello", a.prompt.input.Value())
		}
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 send cmd", len(a.Cmds))
		}
		if a.prompt.input.Value() != "hello" {
			t.Fatal("input clears only on the success msg")
		}
		a.Update(sendMsg{err: nil})
		if a.prompt.input.Value() != "" {
			t.Fatalf("input = %q after success, want cleared", a.prompt.input.Value())
		}
	})

	t.Run("whitespace-only input is ignored", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		typeStr(a, "  ")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want 0", len(a.Cmds))
		}
		if len(a.toasts) != 0 {
			t.Fatalf("toasts = %v, want none", a.toasts)
		}
	})

	t.Run("busy store: locked toast, no send", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.store.Status = protocol.SessionStatus{Type: protocol.SessionStatusBusy}
		typeStr(a, "x")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want 0 (busy)", len(a.Cmds))
		}
		if !hasToast(a, "abort or wait (esc aborts)") {
			t.Fatalf("toasts = %v, want the locked busy toast", a.toasts)
		}
	})

	t.Run("retry store also blocks with the toast", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.store.Status = protocol.SessionStatus{Type: protocol.SessionStatusRetry}
		typeStr(a, "x")
		a.handleKey(press(tea.KeyEnter))
		if !hasToast(a, "abort or wait (esc aborts)") {
			t.Fatalf("toasts = %v, want the locked busy toast", a.toasts)
		}
	})

	t.Run("ErrBusy from the server: toast, input kept", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		typeStr(a, "y")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1", len(a.Cmds))
		}
		a.Update(sendMsg{err: client.ErrBusy})
		if !hasToast(a, "abort or wait (esc aborts)") {
			t.Fatalf("toasts = %v, want the locked busy toast", a.toasts)
		}
		if a.prompt.input.Value() != "y" {
			t.Fatalf("input = %q, want kept", a.prompt.input.Value())
		}
	})

	t.Run("other send error lands in lastErr", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.Update(sendMsg{err: errors.New("boom")})
		if a.lastErr != "boom" {
			t.Fatalf("lastErr = %q, want boom", a.lastErr)
		}
		if len(a.toasts) != 0 {
			t.Fatalf("toasts = %v, want none", a.toasts)
		}
	})

	t.Run("backslash+enter soft-enter (locked multiline escape)", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		typeStr(a, "a\\")
		if a.prompt.input.Value() != "a\\" {
			t.Fatalf("value = %q, want a\\", a.prompt.input.Value())
		}
		a.handleKey(press(tea.KeyEnter)) // soft enter: no send, draft accumulates
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want 0 on soft enter", len(a.Cmds))
		}
		if a.prompt.draft.String() != "a\n" {
			t.Fatalf("draft = %q, want a\\n", a.prompt.draft.String())
		}
		if a.prompt.input.Value() != "" {
			t.Fatal("the line must start empty after soft enter")
		}
		typeStr(a, "b")
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 final send", len(a.Cmds))
		}
		a.Update(sendMsg{err: nil})
		if a.prompt.draft.String() != "" || a.prompt.input.Value() != "" {
			t.Fatalf("draft=%q value=%q after success, want both empty", a.prompt.draft.String(), a.prompt.input.Value())
		}
	})
}

func TestPromptKeyRouting(t *testing.T) {
	t.Run("e and t type into the prompt on the session route", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		typeStr(a, "et")
		if a.prompt.input.Value() != "et" {
			t.Fatalf("value = %q, want et", a.prompt.input.Value())
		}
		if len(a.sess.expanded) != 0 {
			t.Fatalf("expanded = %v, want none (e/t are prompt chars now)", a.sess.expanded)
		}
	})

	t.Run("alt+e expands the last tool, alt+t toggles thinking (rebound)", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.handleKey(pressAlt('e'))
		if len(a.sess.expanded) != 1 || !a.sess.expanded["t3"] {
			t.Fatalf("expanded = %v, want {t3:true}", a.sess.expanded)
		}
		a.handleKey(pressAlt('t'))
		if len(a.sess.expanded) != 2 || !a.sess.expanded["r1"] {
			t.Fatalf("expanded = %v, want t3+r1", a.sess.expanded)
		}
	})

	t.Run("pgup still pauses follow with a prompt present", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.view()
		a.handleKey(tea.KeyPressMsg{Code: tea.KeyPgUp})
		if a.sess.following {
			t.Fatal("follow must pause on pgup")
		}
	})

	t.Run("home esc clears the prompt", func(t *testing.T) {
		a := testApp()
		typeStr(a, "x")
		a.handleKey(press(tea.KeyEscape))
		if a.prompt.input.Value() != "" {
			t.Fatalf("value = %q, want cleared", a.prompt.input.Value())
		}
	})
}

// TestDraftSoftEnterAmortized pins the draft growth path (datastruct-9):
// many soft-enters must stay linear in total draft bytes (the old
// `draft += line` string concat is quadratic).
func TestDraftSoftEnterAmortized(t *testing.T) {
	a := testSessionApp(sessionFixture())
	t.Cleanup(a.Close)
	line := strings.Repeat("x", 100) + "\\" // 100 chars + the soft-enter backslash
	start := time.Now()
	for i := 0; i < 40000; i++ { // 4 MB of draft total
		a.prompt.input.SetValue(line)
		a.handleKey(press(tea.KeyEnter))
	}
	if d := time.Since(start); d > draftAmortizedLimit {
		t.Fatalf("40k soft-enters took %v, want < %v (draft growth must be amortized)", d, draftAmortizedLimit)
	}
	if got := a.prompt.draft.String(); len(got) != 40000*101 {
		t.Fatalf("draft length = %d, want %d", len(got), 40000*101)
	}
}
