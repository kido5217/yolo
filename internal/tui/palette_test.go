package tui

import (
	"fmt"
	"math"
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

// paletteModalTheme is the fixed resolved theme the S1 dim/geometry legs
// render with: the dim derives from the background token (30,30,30 →
// round(30×105/255) = 12 per channel), distinct from the backgroundPanel
// fill (38,38,38).
func paletteModalTheme() theme.Theme {
	return theme.Theme{R: theme.Resolved{Colors: map[string]theme.RGBA{
		"background":      theme.FromHex("#1e1e1e"),
		"backgroundPanel": theme.FromHex("#262626"),
		"text":            theme.FromHex("#eeeeee"),
		"textMuted":       theme.FromHex("#808080"),
	}}}
}

// dimSGROf derives the DimBackdrop's truecolor SGR from the same resolved
// theme the render uses: black (alpha 150/255) over background →
// bg×105/255 per channel (the black-overlay source-over formula).
func dimSGROf(th theme.Theme) string {
	bg, _ := th.Color("background")
	dim := func(v uint8) uint8 { return uint8(math.Round(float64(v) * 105 / 255)) }
	return fmt.Sprintf("48;2;%d;%d;%d", dim(bg.R), dim(bg.G), dim(bg.B))
}

func TestPaletteModalDimBackdrop(t *testing.T) {
	t.Run("home route", func(t *testing.T) { testPaletteModalDim(t, routeHome) })
	t.Run("session route", func(t *testing.T) { testPaletteModalDim(t, routeSession) })
}

// testPaletteModalDim pins the S1 dim + chrome parity (whitebox, the
// testApp harness with the palette pushed): the flat dim field on every
// non-panel line (the chrome region, the tail lines and the footer line —
// no content), the panel geometry unchanged (w=60, top max(h/4,
// modalChromeMin), centered, backgroundPanel fill) and the inner lines
// (the title row's esc hint, the command_list footer hint replacing the
// generic nav hint, the filter row untouched). Both routes.
func testPaletteModalDim(t *testing.T, route route) {
	a := testApp()
	a.theme = paletteModalTheme()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	a.route = route
	a.store.Commands = []protocol.Command{
		{Name: "/help", Description: "Show help"},
		{Name: "/new", Description: "New session"},
		{Name: "/model", Description: "List models"},
		{Name: "/agents", Description: "List agents"},
		{Name: "/quit", Description: "Quit"},
	}
	a.openPaletteDialog()
	lines := strings.Split(a.view(), "\n")
	if len(lines) != 24 {
		t.Fatalf("frame = %d lines, want 24", len(lines))
	}
	dimSGR := dimSGROf(a.theme)
	panelSGR := "48;2;38;38;38" // the backgroundPanel fill (#262626)
	var panelTop int
	if route == routeSession {
		// session chrome min = title 1 + viewport 1 + divider 1 + help 1 = 4
		// < 24/4 = 6 → panelTop = h/4 (6)
		panelTop = 6
	} else {
		// home chrome min = logo 4 + box 5 + hint 1 = 10 > 24/4 = 6 →
		// panelTop = the chromeMin clamp (10)
		panelTop = 10
	}
	// inner lines = title + filter + 6 visible rows (24/2−6) + footer = 9;
	// the panel = the top-padding line + 9 = 10 lines (well under avail).
	panelBottom := panelTop + 9
	assertDim := func(i int, what string) {
		if !strings.Contains(lines[i], dimSGR) {
			t.Fatalf("%s line %d lacks the dim SGR %s:\n%s", what, i, dimSGR, lines[i])
		}
		if s := strings.TrimSpace(stripANSI(lines[i])); s != "" {
			t.Fatalf("%s line %d carries content %q, want the flat dim field", what, i, s)
		}
	}
	// (a) the dim: the chrome region (lines 0..panelTop−1 — the session
	// route's clamped chrome, the home route's logo/box clamp region), the
	// tail lines (panelBottom+1..h−2) and the footer line (h−1) — all
	// dimmed, all carrying no content (the flat dim field, spec §2.2).
	for i := 0; i < panelTop; i++ {
		assertDim(i, "chrome")
	}
	for i := panelBottom + 1; i < len(lines)-1; i++ {
		assertDim(i, "tail")
	}
	assertDim(len(lines)-1, "footer")
	// (b) the panel geometry is unchanged: w=60 (w−2=78 unclamped at 80),
	// top max(h/4, chromeMin) = panelTop, centered (lead (80−60)/2 = 10),
	// the backgroundPanel fill (no dim SGR on the panel cells).
	for i := panelTop; i <= panelBottom; i++ {
		if strings.Contains(lines[i], dimSGR) {
			t.Fatalf("panel line %d carries the dim SGR:\n%s", i, lines[i])
		}
		if !strings.Contains(lines[i], panelSGR) {
			t.Fatalf("panel line %d lacks the backgroundPanel fill:\n%s", i, lines[i])
		}
	}
	if got := stripANSI(lines[panelTop+1]); !strings.HasPrefix(got, strings.Repeat(" ", 10)) {
		t.Fatalf("title line %d not centered (lead 10):\n%q", panelTop+1, got)
	}
	// (c) the inner lines: the title row (Commands left, esc right in
	// textMuted, space-between the panel width) + the footer row (the
	// ctrl+p commands keymap hint, right-aligned, replacing the generic
	// nav hint for the palette select only).
	titlePlain := strings.Trim(stripANSI(lines[panelTop+1]), " ")
	if want := "Commands" + strings.Repeat(" ", 60-8-3) + "esc"; titlePlain != want {
		t.Fatalf("title row = %q, want %q (space-between 60)", titlePlain, want)
	}
	if !strings.Contains(lines[panelTop+1], "38;2;128;128;128") {
		t.Fatalf("esc hint not textMuted (#808080):\n%s", lines[panelTop+1])
	}
	footerPlain := strings.TrimRight(stripANSI(lines[panelBottom]), " ")
	if !strings.HasSuffix(footerPlain, "ctrl+p commands") {
		t.Fatalf("footer row = %q, want the ctrl+p commands hint right-aligned", footerPlain)
	}
	if strings.Contains(a.view(), "\u2191/\u2193 move") {
		t.Fatal("the generic nav hint must be replaced by the palette footer hint")
	}
	// (d) the filter-input row is unchanged (the placeholder).
	if !strings.Contains(stripANSI(lines[panelTop+2]), "Filter commands") {
		t.Fatalf("filter row = %q, want the Filter commands placeholder", stripANSI(lines[panelTop+2]))
	}
}

func TestTUICommandPalette(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine(homeLogoLine), teatest.WithDuration(5*time.Second))

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
