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

// sessionListFixture: three sessions; the updated-desc order is s3, s2, s1
// (s1 is the current).
func sessionListFixture() []protocol.Session {
	mk := func(id, title string, updated int64) protocol.Session {
		return protocol.Session{
			ID: id, Title: title, Directory: "/work/" + id,
			Time: protocol.SessionTime{Created: updated - 60_000, Updated: updated},
		}
	}
	return []protocol.Session{
		mk("s1", "alpha", 1_000),
		mk("s2", "beta", 2_000),
		mk("s3", "gamma", 3_000),
	}
}

func openSessionsDlg(t *testing.T, s []protocol.Session) *recApp {
	t.Helper()
	a := testApp(s...)
	a.curSessionID = "s1"
	a.openSessionListDialog()
	a.Cmds = nil // the status-snapshot cmd (dummy client; not executed here)
	return a
}

func TestSessionCategory(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	older := time.UnixMilli(1_700_000_000_000)
	tests := []struct {
		name    string
		updated time.Time
		want    string
	}{
		{"today", now, "Today"},
		{"older", older, older.Format("Mon Jan 2 2006")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sessionCategory(tc.updated, now); got != tc.want {
				t.Fatalf("category = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSessionListDialogRender(t *testing.T) {
	t.Run("updated-desc order, title, search input, current marker", func(t *testing.T) {
		a := openSessionsDlg(t, sessionListFixture())
		got := stripANSI(a.dlg.sessions().view(80, 24))
		if !strings.Contains(got, "Sessions") || !strings.Contains(got, "Search") {
			t.Fatalf("title/placeholder missing:\n%s", got)
		}
		i3, i2, i1 := strings.Index(got, "gamma"), strings.Index(got, "beta"), strings.Index(got, "alpha")
		if i3 < 0 || i2 < 0 || i1 < 0 || !(i3 < i2 && i2 < i1) {
			t.Fatalf("rows not in updated-desc order (gamma < beta < alpha):\n%s", got)
		}
		if !strings.Contains(got, "●") {
			t.Fatalf("current-session gutter missing:\n%s", got)
		}
	})

	t.Run("skipFilter: typed text client-filters the titles", func(t *testing.T) {
		a := openSessionsDlg(t, sessionListFixture())
		a.handleKey(press('g')) // only "gamma" contains g
		got := stripANSI(a.dlg.sessions().view(80, 24))
		if !strings.Contains(got, "gamma") || strings.Contains(got, "beta") || strings.Contains(got, "alpha") {
			t.Fatalf("client-side filter did not narrow to gamma:\n%s", got)
		}
	})

	t.Run("enter opens the selected session and closes", func(t *testing.T) {
		a := openSessionsDlg(t, sessionListFixture())
		a.handleKey(press(tea.KeyDown)) // wrap to the first row (gamma)
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() || a.curSessionID != "s3" {
			t.Fatalf("open failed: empty=%v cur=%s", a.dlg.empty(), a.curSessionID)
		}
		if len(a.Cmds) == 0 {
			t.Fatal("no hydrate cmd emitted after the open")
		}
	})

	t.Run("two-step delete: arm, onMove clears, confirm emits", func(t *testing.T) {
		a := openSessionsDlg(t, sessionListFixture())
		a.handleKey(ctrlDKey) // arm on the current selection (alpha)
		got := stripANSI(a.dlg.sessions().view(80, 24))
		if !strings.Contains(got, "Press ctrl+d again to confirm") {
			t.Fatalf("armed title missing:\n%s", got)
		}
		a.handleKey(press(tea.KeyUp)) // onMove clears the armed state
		if got := stripANSI(a.dlg.sessions().view(80, 24)); strings.Contains(got, "Press ctrl+d again to confirm") {
			t.Fatalf("onMove must clear the armed row:\n%s", got)
		}
		a.handleKey(ctrlDKey) // re-arm on the new selection (beta)
		a.handleKey(ctrlDKey) // confirm
		if len(a.Cmds) != 1 {
			t.Fatalf("confirm emitted %d cmds, want 1 (the delete)", len(a.Cmds))
		}
		if a.dlg.empty() {
			t.Fatal("the dialog must stay open until the delete resolves")
		}
	})
}

func TestSessionListDeleteResolves(t *testing.T) {
	t.Run("success closes and goes home when the current session dies", func(t *testing.T) {
		ts := testutil.Boot(t)
		c := client.New(ts.URL, ts.Dir)
		seed, err := c.CreateSession(context.Background(), "")
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		a := newRecApp(c, store.State{Sessions: []protocol.Session{seed}, Current: &seed}, seed.ID)
		t.Cleanup(a.Close)
		a.openSessionListDialog()
		a.Cmds = nil
		a.handleKey(ctrlDKey)
		a.handleKey(ctrlDKey)
		driveCmds(t, a) // the delete cmd round-trips; applySessionDelete fires
		if !a.dlg.empty() {
			t.Fatalf("the dialog must close on success: depth=%d", len(a.dlg.items))
		}
		if a.route != routeHome {
			t.Fatalf("the deleted session was current: route = %v, want home", a.route)
		}
	})
}

// TestTUISessionListDialog is the teatest leg: the real stack + the real
// engine, /sessions opens the dialog, the two-step delete arms the
// error-background row. ONE merged terminal condition (the multi-token
// state must be a single WaitFor — the shared buffer drains per wait),
// and the pinned TTY env for the SGR assertion.
func TestTUISessionListDialog(t *testing.T) {
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
	seed, err := c.CreateSession(context.Background(), "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := NewApp(c, store.State{Sessions: []protocol.Session{seed}, Current: &seed}, seed.ID, e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1", "TERM=xterm-256color"})),
	)

	teatest.WaitFor(t, tm.Output(), hasLines("New session"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "/sessions")
	tm.Send(press(tea.KeyEnter))
	// ONE merged condition: the dialog title + the session row + the
	// search input (skipFilter keeps it rendered).
	teatest.WaitFor(t, tm.Output(), hasLines("Sessions", "Search"), teatest.WithDuration(5*time.Second))

	tm.Send(ctrlDKey)
	tm.Send(ctrlDKey)
	// ONE merged condition: the armed title (plain) + the error-background
	// SGR param (48;5;246 = yolo dark error #e06c75 under the pinned
	// env — the Convert256 scratch output wins over findings §6's 174).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(stripANSI(string(b)), "Press ctrl+d again to confirm") &&
			bytes.Contains(b, []byte("48;5;246"))
	}, teatest.WithDuration(5*time.Second))

	tm.Send(press(tea.KeyEscape))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
