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
		{"/", []string{"/help", "/new", "/model", "/agents", "/quit"}},
		{"/m", []string{"/model"}},
		{"/n", []string{"/new"}},
		{"/h", []string{"/help"}},
		{"/quit", []string{"/quit"}},
		{"/exit", []string{"/quit"}}, // alias: canonical /quit is surfaced
		{"/ex", []string{"/quit"}},
		{"/zz", []string{}},
	}
	for _, tt := range tests {
		t.Run("in="+tt.in, func(t *testing.T) {
			a := testApp()
			a.store.Commands = testCommands()
			a.prompt.input.SetValue(tt.in)
			got := a.prompt.menuItems(a.store.Commands)
			gotNames := []string(nil)
			if got != nil {
				gotNames = make([]string, 0, len(got))
				for _, c := range got {
					gotNames = append(gotNames, c.Name)
				}
			}
			if len(gotNames) != len(tt.want) {
				t.Fatalf("menuItems(%q) = %v, want %v", tt.in, gotNames, tt.want)
			}
			for i := range tt.want {
				if gotNames[i] != tt.want[i] {
					t.Fatalf("menuItems(%q) = %v, want %v", tt.in, gotNames, tt.want)
				}
			}
		})
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

	t.Run("menu open: arrows do not move the home cursor", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		typeStr(a, "/")
		a.handleKey(press(tea.KeyDown))
		if a.home.cursor != 0 {
			t.Fatalf("home cursor = %d, want 0 (menu owns arrows)", a.home.cursor)
		}
	})

	t.Run("enter with no match clears the input (locked)", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		typeStr(a, "/zz")
		if !a.prompt.slashActive() || len(a.prompt.menuItems(a.store.Commands)) != 0 {
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

	t.Run("home: up/down drive the list, letters go to the prompt", func(t *testing.T) {
		sessions := []protocol.Session{
			{ID: "ses_0", Title: "T1", Time: protocol.SessionTime{Updated: testNow}},
			{ID: "ses_1", Title: "T2", Time: protocol.SessionTime{Updated: testNow}},
		}
		a := testApp(sessions...)
		a.store.Commands = testCommands()
		a.handleKey(press(tea.KeyDown))
		if a.home.cursor != 1 {
			t.Fatalf("cursor = %d, want 1", a.home.cursor)
		}
		typeStr(a, "hi")
		if a.prompt.input.Value() != "hi" {
			t.Fatalf("value = %q, want hi", a.prompt.input.Value())
		}
		if a.home.cursor != 1 {
			t.Fatalf("cursor = %d, want unchanged 1", a.home.cursor)
		}
		a.handleKey(press('n'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 (n still creates)", len(a.Cmds))
		}
		if a.prompt.input.Value() != "hi" {
			t.Fatalf("value = %q, want hi (n is reserved)", a.prompt.input.Value())
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
