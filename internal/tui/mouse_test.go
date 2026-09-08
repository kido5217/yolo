package tui

import (
	"bytes"
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// TestPromptSlashMouseHover drives a mouse motion over a row of the open slash
// dropdown (S5 mouse, spec §3.1) and asserts the selection moves to the hovered
// row. The teatest leg sends a tea.MouseMsg at the S3 anchor for row 1
// (slashDropdownRows). The selection is asserted on the model after the program
// has quit (not a race with the running program).
func TestPromptSlashMouseHover(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", nil)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("● Tip"))
	}, teatest.WithDuration(5*time.Second))
	for _, r := range "/" {
		tm.Send(press(r))
	}
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("|"))
	}, teatest.WithDuration(3*time.Second))

	// the S3 anchor for the open dropdown: the first row's cell + the visible
	// count (the menu opens with at least two rows, at sel 0).
	firstRow, vis := a.slashDropdownRows()
	if firstRow < 0 || vis < 2 {
		t.Fatalf("dropdown not open (firstRow=%d vis=%d)", firstRow, vis)
	}

	// a motion over row 1 (a non-top row) moves the selection there.
	tm.Send(tea.MouseMotionMsg{X: 10, Y: firstRow + 1, Button: tea.MouseNone})

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	if a.prompt.sel != 1 {
		t.Fatalf("expected sel 1 after hovering row 1, got %d", a.prompt.sel)
	}
}

// TestPromptSlashMouseClick drives a mouse click on the /new row of the open
// slash dropdown (a home without session) and asserts the command runs — a
// session is minted (the TestPromptSlashNewWithoutSession idiom, with the enter
// key replaced by the mouse click). The click runs the SAME enter path as the
// key handler (runCommand, so the S4 touchCommandFrecency fires there too).
func TestPromptSlashMouseClick(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", nil)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	ctx := context.Background()

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("● Tip"))
	}, teatest.WithDuration(5*time.Second))
	for _, r := range "/new" {
		tm.Send(press(r))
	}
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("|"))
	}, teatest.WithDuration(3*time.Second))

	// /new is the only dropdown row; the S3 anchor is its cell.
	firstRow, vis := a.slashDropdownRows()
	if firstRow < 0 || vis != 1 {
		t.Fatalf("expected 1 dropdown row (/new), got vis=%d firstRow=%d", vis, firstRow)
	}

	// a click on the /new row runs the command (mints a session on a home).
	tm.Send(tea.MouseClickMsg{X: 10, Y: firstRow, Button: tea.MouseLeft})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		sessions, err := c.ListSessions(ctx)
		return err == nil && len(sessions) >= 1
	}, teatest.WithDuration(5*time.Second))

	sessions, err := c.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 minted session after the click, got %d", len(sessions))
	}

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
