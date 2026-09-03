package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func TestPaletteOptions(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{
		{Name: "/help", Description: "Show help"},
		{Name: "/new", Description: "New session"},
		{Name: "/model", Description: "List models"},
		{Name: "/agents", Description: "List agents"},
		{Name: "/quit", Description: "Quit"},
	}
	opts := paletteOptions(a.App)
	if len(opts) != 9 {
		t.Fatalf("palette = %d options, want 9 (4 local + 5 server)", len(opts))
	}
	if opts[0].title != "sessions" {
		t.Fatalf("first option = %q, want sessions (the local /sessions first)", opts[0].title)
	}
	byTitle := map[string]selectOption{}
	for _, o := range opts {
		byTitle[o.title] = o
	}
	if byTitle["model"].footer != "ctrl+x m" {
		t.Fatalf("/model footer = %q, want ctrl+x m", byTitle["model"].footer)
	}
	if byTitle["help"].footer != "" {
		t.Fatalf("/help footer = %q, want blank (help_show = none)", byTitle["help"].footer)
	}
	if byTitle["quit"].footer != "ctrl+c / ctrl+d / ctrl+x q" {
		t.Fatalf("/quit footer = %q, want the app_exit comma-list display", byTitle["quit"].footer)
	}
}

func TestPaletteOpen(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{{Name: "/help", Description: "Show help"}}
	a.openPaletteDialog()
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgPalette || d.sel == nil {
		t.Fatalf("after openPaletteDialog: top=%+v (ok=%v), want the palette select", d, ok)
	}
}

func TestPaletteDispatch(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{{Name: "/help", Description: "Show help"}}
	a.handleKey(pressCtrlP()) // command_list → the palette (the S4.2 remap lands)
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgPalette {
		t.Fatalf("after ctrl+p: top=%+v (ok=%v), want the palette", d, ok)
	}
}

func TestPaletteSelectPick(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{{Name: "/help", Description: "Show help"}}
	a.openPaletteDialog()
	d, ok := a.dlg.top()
	if !ok {
		t.Fatal("the palette must be on top")
	}
	sel := d.sel
	sel.sel = 0 // the local /sessions (first)
	sel.submit(a.App)
	d, ok = a.dlg.top()
	if ok && d.kind == dlgPalette {
		t.Fatal("the palette must close after a run")
	}
	if d.kind != dlgSessions {
		t.Fatalf("after the palette run: top=%+v, want the session-list dialog", d)
	}
}

func TestPaletteNav(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{
		{Name: "/help", Description: "Show help"},
		{Name: "/new", Description: "New session"},
	}
	a.openPaletteDialog()
	d, ok := a.dlg.top()
	if !ok {
		t.Fatal("the palette must be on top")
	}
	sel := d.sel
	n := len(sel.filtered())
	if sel.sel != 0 {
		t.Fatalf("initial sel = %d, want 0", sel.sel)
	}
	sel.handleKey(a.App, press(tea.KeyDown))
	if sel.sel != 1 {
		t.Fatalf("sel after down = %d, want 1", sel.sel)
	}
	sel.handleKey(a.App, press(tea.KeyUp))
	sel.handleKey(a.App, press(tea.KeyUp)) // wraps to the last
	if sel.sel != n-1 {
		t.Fatalf("sel after wrap-up = %d, want last (%d)", sel.sel, n-1)
	}
}

func TestPaletteEsc(t *testing.T) {
	a := testApp()
	a.store.Commands = []protocol.Command{{Name: "/help", Description: "Show help"}}
	a.openPaletteDialog()
	a.handleKey(press(tea.KeyEscape))
	if d, ok := a.dlg.top(); ok {
		t.Fatalf("after esc: top=%+v, want the palette closed", d)
	}
}

func TestTUICommandPalette(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))

	// S4.4: ctrl+p opens the command palette (the remap).
	tm.Send(pressCtrlP())
	teatest.WaitFor(t, tm.Output(), hasLine("Commands"), teatest.WithDuration(5*time.Second))

	// filter to "help" (the S2.5 fuzzy narrows), enter runs /help.
	for _, r := range "help" {
		tm.Send(press(r))
	}
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), hasLine("Help"), teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
