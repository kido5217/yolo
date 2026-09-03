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

func openDeleteFailedDlg() *recApp {
	a := testApp()
	a.openDeleteFailedDialog("s1", "alpha", "session not found")
	return a
}

func TestDeleteFailedDialogRender(t *testing.T) {
	a := openDeleteFailedDlg()
	got := stripANSI(a.dlg.deleteFailed().view(80, 24, a.theme))
	for _, tok := range []string{
		"Failed to Delete Session",
		`The session "alpha" could not be deleted: session not found`,
		"Choose how to proceed.",
		"Retry delete", "Keep session",
	} {
		if !strings.Contains(got, tok) {
			t.Fatalf("token %q missing:\n%s", tok, got)
		}
	}
}

func TestDeleteFailedDialogKeys(t *testing.T) {
	t.Run("right moves the active row, enter-keep closes and re-hydrates", func(t *testing.T) {
		a := openDeleteFailedDlg()
		a.handleKey(press(tea.KeyRight))
		if a.dlg.deleteFailed().active != 1 {
			t.Fatalf("active = %d, want 1", a.dlg.deleteFailed().active)
		}
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() || len(a.Cmds) != 1 {
			t.Fatalf("keep must close + hydrate: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})

	t.Run("left clamps at 0, enter-retry re-emits the delete", func(t *testing.T) {
		a := openDeleteFailedDlg()
		a.handleKey(press(tea.KeyLeft)) // already at 0: clamped
		if a.dlg.deleteFailed().active != 0 {
			t.Fatalf("active = %d, want 0 (clamped)", a.dlg.deleteFailed().active)
		}
		a.handleKey(press(tea.KeyEnter))
		if len(a.Cmds) != 1 || a.dlg.empty() {
			t.Fatalf("retry must re-emit + stay open: cmds=%d empty=%v", len(a.Cmds), a.dlg.empty())
		}
	})

	t.Run("esc closes without acting", func(t *testing.T) {
		a := openDeleteFailedDlg()
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() || len(a.Cmds) != 0 {
			t.Fatalf("esc must close silently: empty=%v cmds=%d", a.dlg.empty(), len(a.Cmds))
		}
	})
}

// TestSessionListDeleteFailureOpensDlg is the S3.1-deferred leg: a delete
// that 404s on the server opens the delete-failed dialog with the wire
// error.
func TestSessionListDeleteFailureOpensDlg(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	ghost := protocol.Session{ID: "ghost", Title: "ghost", Time: protocol.SessionTime{Updated: 1}}
	a := newRecApp(c, store.State{Sessions: []protocol.Session{ghost}}, "other")
	t.Cleanup(a.Close)
	a.openSessionListDialog()
	a.Cmds = nil
	a.handleKey(ctrlDKey)
	a.handleKey(ctrlDKey)
	driveCmds(t, a) // the server 404s the delete
	top, ok := a.dlg.top()
	if !ok || top.kind != dlgDeleteFailed {
		t.Fatalf("top = %v, want dlgDeleteFailed", top.kind)
	}
	got := stripANSI(top.deleteFailed.view(80, 24, a.theme))
	if !strings.Contains(got, `The session "ghost" could not be deleted:`) {
		t.Fatalf("wire error missing from the body:\n%s", got)
	}
}

// TestTUIDeleteFailedDialog is the teatest SGR leg: the active option row
// paints the primary background (48;5;216 — yolo dark primary, the
// homeSGRTokens-pinned index).
func TestTUIDeleteFailedDialog(t *testing.T) {
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
	a := NewApp(client.New("http://127.0.0.1:9", ""), store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1", "TERM=xterm-256color"})),
	)
	a.openDeleteFailedDialog("s1", "alpha", "session not found")

	// ONE merged condition: the plain header + both option labels + the
	// active-row primary bg SGR param.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		return strings.Contains(s, "Failed to Delete Session") &&
			strings.Contains(s, "Retry delete") &&
			strings.Contains(s, "Keep session") &&
			bytes.Contains(b, []byte("48;5;216"))
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
