package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func renameApp(t *testing.T) (*recApp, string) {
	t.Helper()
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	seed, err := c.CreateSession(context.Background(), "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := c.PatchSession(context.Background(), seed.ID, map[string]any{"title": "alpha"}); err != nil {
		t.Fatalf("PatchSession: %v", err)
	}
	a := newRecApp(c, store.State{Sessions: []protocol.Session{{ID: seed.ID, Title: "alpha"}}, Current: &protocol.Session{ID: seed.ID, Title: "alpha"}}, seed.ID)
	t.Cleanup(a.Close)
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	return a, seed.ID
}

func TestSessionRenameDialog(t *testing.T) {
	t.Run("confirm patches the title and closes, no toast", func(t *testing.T) {
		a, id := renameApp(t)
		a.openSessionRenameDialog(id)
		if got := a.dlg.form(); got == nil {
			t.Fatal("the form modal must be on top")
		}
		// typed text appends at the cursor (end of the initial title)
		updateKey(a, press(' '))
		updateKey(a, press('2'))
		updateKey(a, enterKey)
		driveCmds(t, a) // the submit cascade + the rename cmd round-trip
		if len(a.dlg.items) != 0 {
			t.Fatalf("the dialog must close: depth=%d", len(a.dlg.items))
		}
		if a.store.Sessions[0].Title != "alpha 2" {
			t.Fatalf("title = %q, want %q", a.store.Sessions[0].Title, "alpha 2")
		}
		if len(a.toasts) != 0 {
			t.Fatalf("no success toast (upstream parity), got %v", a.toasts)
		}
	})

	t.Run("esc cancels without patching", func(t *testing.T) {
		a, id := renameApp(t)
		a.openSessionRenameDialog(id)
		updateKey(a, press(tea.KeyEscape))
		driveCmds(t, a)
		if len(a.dlg.items) != 0 || a.store.Sessions[0].Title != "alpha" {
			t.Fatalf("cancel leaked: depth=%d title=%q", len(a.dlg.items), a.store.Sessions[0].Title)
		}
	})

	t.Run("empty title closes without patching (the upstream guard)", func(t *testing.T) {
		a, id := renameApp(t)
		a.openSessionRenameDialog(id)
		// backspace the whole initial title ("alpha" = 5 chars)
		for i := 0; i < 5; i++ {
			updateKey(a, press(tea.KeyBackspace))
		}
		updateKey(a, enterKey)
		driveCmds(t, a)
		if len(a.dlg.items) != 0 || a.store.Sessions[0].Title != "alpha" {
			t.Fatalf("empty-title guard failed: depth=%d title=%q", len(a.dlg.items), a.store.Sessions[0].Title)
		}
	})

	t.Run("session route ctrl+r opens the dialog", func(t *testing.T) {
		a, _ := renameApp(t)
		a.route = routeSession
		cmds := a.handleKey(ctrlRKey)
		if len(cmds) != 0 {
			t.Fatalf("ctrl+r must be consumed (no cmds), got %d", len(cmds))
		}
		if a.dlg.form() == nil {
			t.Fatal("ctrl+r must open the rename form")
		}
	})
}
