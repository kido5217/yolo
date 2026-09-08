package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

func TestHelpDialogView(t *testing.T) {
	a := testApp()
	a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
	got := stripANSI(a.helpDialogView(80, 24, a.theme))
	for _, tok := range []string{
		"Help",
		"esc/enter",
		"Press ctrl+p to see all available actions and commands in any context.",
		"pgup/pgdn scroll \u00B7 \\+enter newline",
		"ok",
	} {
		if !strings.Contains(got, tok) {
			t.Fatalf("token %q missing:\n%s", tok, got)
		}
	}
	// the pre-S3 markdown table is gone
	for _, gone := range []string{"| enter |", "| pgup |"} {
		if strings.Contains(got, gone) {
			t.Fatalf("stale table token %q still present:\n%s", gone, got)
		}
	}
}

func TestHelpPaletteHintFromRegistry(t *testing.T) {
	a := testApp()
	// S4.7: the default palette hint is the registry's command_list binding
	// (byte-identical to the pre-S4 "ctrl+p" — the existing goldens hold).
	if got := a.paletteShortcut(); got != "ctrl+p" {
		t.Fatalf("default palette hint = %q, want ctrl+p (the registry default)", got)
	}
	// A remap is reflected in the hint and in the /help body (registry-driven).
	if err := a.keymap.Set("command_list", "ctrl+k"); err != nil {
		t.Fatal(err)
	}
	if got := a.paletteShortcut(); got != "ctrl+k" {
		t.Fatalf("remapped palette hint = %q, want ctrl+k", got)
	}
	v := stripANSI(a.helpDialogView(80, 24, a.theme))
	if !strings.Contains(v, "Press ctrl+k to see all available actions") {
		t.Fatalf("the /help palette hint must reflect the remap:\n%s", v)
	}
	// The V1-pinned line is untouched (kept byte-identical).
	if !strings.Contains(v, "pgup/pgdn scroll \u00B7 \\+enter newline") {
		t.Fatalf("the V1-pinned help line must stay byte-identical:\n%s", v)
	}
}

func TestHelpDialogKeys(t *testing.T) {
	a := testApp()
	a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
	// a plain key is ignored (pre-S3: any key closed)
	if cmds := a.handleKey(press('x')); len(cmds) != 0 || a.dlg.empty() {
		t.Fatalf("a plain key must be ignored: cmds=%d empty=%v", len(cmds), a.dlg.empty())
	}
	// enter closes
	a.handleKey(press(tea.KeyEnter))
	if !a.dlg.empty() {
		t.Fatal("enter must close the help dialog")
	}
	// esc closes
	a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
	a.handleKey(press(tea.KeyEscape))
	if !a.dlg.empty() {
		t.Fatal("esc must close the help dialog")
	}
}

// TestTUIHelpDialog is the teatest SGR golden: the modal help on the real
// stack — the "ok" pill paints the primary bg (48;5;216) + the
// SelectedForeground fg (38;5;232 — the homeSGRTokens-pinned indices).
func TestTUIHelpDialog(t *testing.T) {
	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}

	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1", "TERM=xterm-256color"})),
	)

	teatest.WaitFor(t, tm.Output(), hasLines(homeLogoLine), teatest.WithDuration(5*time.Second))
	suiteType(tm, "/help")
	tm.Send(press(tea.KeyEnter))
	// ONE merged condition: the plain header + the palette line + the V1
	// note + the ok pill's SGR params.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		return strings.Contains(s, "Help") &&
			strings.Contains(s, "Press ctrl+p to see all available actions") &&
			strings.Contains(s, "pgup/pgdn scroll") &&
			bytes.Contains(b, []byte("48;5;216")) &&
			bytes.Contains(b, []byte("38;5;232"))
	}, teatest.WithDuration(5*time.Second))

	tm.Send(press(tea.KeyEscape))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// ctrl+c must quit from the session route too (the plan's TestTUIFullTurn
// quits from an open session; home alone was not enough). Immediate quit: it
// emits tea.Quit with no confirmation dialog.
func TestQuitFromSessionRoute(t *testing.T) {
	a := testApp(protocol.Session{ID: "ses_1"})
	a.route = routeSession
	a.curSessionID = "ses_1"
	cmds := a.handleKey(ctrlCKey)
	if len(cmds) != 1 {
		t.Fatalf("session-route ctrl+c emitted %d cmds, want 1 (quit)", len(cmds))
	}
	if _, ok := cmds[0]().(tea.QuitMsg); !ok {
		t.Fatalf("session-route ctrl+c cmd yields %T, want tea.QuitMsg", cmds[0]())
	}
	if _, ok := a.dlg.top(); ok {
		t.Fatal("session-route ctrl+c pushed a dialog, want none (immediate quit)")
	}
}
