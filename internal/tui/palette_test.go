package tui

import (
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
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
