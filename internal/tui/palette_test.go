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
	// the Suggested seed (decision A) leads: the default testApp (home route,
	// no sessions, no providers) seeds /model (always) + /connect (a provider
	// is not connected) → 2 suggested + 9 plain = 11.
	if len(opts) != 11 {
		t.Fatalf("palette = %d options, want 11 (2 suggested + 9 plain)", len(opts))
	}
	if opts[0].category != "Suggested" || opts[1].category != "Suggested" {
		t.Fatalf("leading options = %q/%q (categories %q/%q), want the Suggested group first",
			opts[0].title, opts[1].title, opts[0].category, opts[1].category)
	}
	// the plain entries carry the client-side category buckets (decision B);
	// the footer checks read them by title (the plain rows overwrite the
	// Suggested twins in the map).
	byTitle := map[string]selectOption{}
	for _, o := range opts {
		byTitle[o.title] = o
	}
	if byTitle["model"].category != "Model" {
		t.Fatalf("/model category = %q, want Model", byTitle["model"].category)
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
	// pick the plain /sessions option (the Suggested group may precede it, so
	// its index is no longer guaranteed to be 0).
	want := -1
	for i, o := range sel.filtered() {
		if o.value == "/sessions" {
			want = i
			break
		}
	}
	if want < 0 {
		t.Fatal("no /sessions option in the palette")
	}
	sel.sel = want
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

// paletteTestApp is the palette test harness: the testApp with the frozen
// 5-command server catalog (the TUI merges in the 4 locals → 9).
func paletteTestApp(sessions ...protocol.Session) *recApp {
	a := testApp(sessions...)
	a.store.Commands = []protocol.Command{
		{Name: "/help", Description: "Show help"},
		{Name: "/new", Description: "New session"},
		{Name: "/model", Description: "List models"},
		{Name: "/agents", Description: "List agents"},
		{Name: "/quit", Description: "Quit"},
	}
	return a
}

// suggestedTitles returns the set of titles in the palette's Suggested group
// (category "Suggested").
func suggestedTitles(opts []selectOption) map[string]bool {
	out := map[string]bool{}
	for _, o := range opts {
		if o.category == "Suggested" {
			out[o.title] = true
		}
	}
	return out
}

func hasSuggested(l []selectOption) bool {
	for _, o := range l {
		if o.category == "Suggested" {
			return true
		}
	}
	return false
}

func hasTitle(l []selectOption, title string) bool {
	for _, o := range l {
		if o.title == title {
			return true
		}
	}
	return false
}

// TestPaletteSuggestedSeedConditions pins the four Suggested seeds (decision
// A): /model always; /new only on the session route; /sessions only when a
// session is stored; /connect only when a provider is not connected.
func TestPaletteSuggestedSeedConditions(t *testing.T) {
	t.Run("model always", func(t *testing.T) {
		a := paletteTestApp()
		a.route = routeHome
		if got := suggestedTitles(paletteOptions(a.App)); !got["model"] {
			t.Fatalf("/model not seeded (home route, no state): %v", got)
		}
	})
	t.Run("new only on the session route", func(t *testing.T) {
		home := paletteTestApp()
		home.route = routeHome
		if got := suggestedTitles(paletteOptions(home.App)); got["new"] {
			t.Fatalf("/new seeded on the home route, want only the session route: %v", got)
		}
		sess := paletteTestApp()
		sess.route = routeSession
		if got := suggestedTitles(paletteOptions(sess.App)); !got["new"] {
			t.Fatalf("/new not seeded on the session route: %v", got)
		}
	})
	t.Run("sessions only when a session is stored", func(t *testing.T) {
		none := paletteTestApp()
		if got := suggestedTitles(paletteOptions(none.App)); got["sessions"] {
			t.Fatalf("/sessions seeded with no stored session: %v", got)
		}
		some := paletteTestApp(protocol.Session{ID: "s1"})
		if got := suggestedTitles(paletteOptions(some.App)); !got["sessions"] {
			t.Fatalf("/sessions not seeded with a stored session: %v", got)
		}
	})
	t.Run("connect only when a provider is not connected", func(t *testing.T) {
		a := paletteTestApp()
		if got := suggestedTitles(paletteOptions(a.App)); !got["connect"] {
			t.Fatalf("/connect not seeded with no provider connected: %v", got)
		}
		connected := paletteTestApp()
		connected.store.Providers = []protocol.Provider{{ID: "other"}}
		if got := suggestedTitles(paletteOptions(connected.App)); got["connect"] {
			t.Fatalf("/connect seeded with a provider connected: %v", got)
		}
	})
}

// TestPaletteSuggestedEmptyFilterRender pins the empty-filter render
// (decisions A + B): the Suggested group at the top (the accent category
// header) with the seeded entries, then the plain category buckets
// (General/Session/Model/Provider/Agent).
func TestPaletteSuggestedEmptyFilterRender(t *testing.T) {
	a := paletteTestApp(protocol.Session{ID: "s1"})
	a.route = routeSession // all four seeds active
	m := selectNew("Commands", "Filter commands", paletteOptions(a.App), nil, nil, nil)
	// (1) the Suggested header leads the rendered list (buildLines).
	lines := m.buildLines(60, a.theme)
	if lines[0].opt != -1 || !strings.HasSuffix(stripANSI(lines[0].text), "Suggested") {
		t.Fatalf("first line = %+v, want the Suggested category header at the top", lines[0])
	}
	// (2) the seeded entries lead the live list (the Suggested rows are
	// first in filtered()).
	l := m.filtered()
	if len(l) < 4 || l[0].category != "Suggested" {
		t.Fatalf("live list = %v, want the Suggested rows first", titlesOf(l))
	}
	sugg := map[string]bool{}
	for _, o := range l {
		if o.category != "Suggested" {
			break
		}
		sugg[o.title] = true
	}
	for _, want := range []string{"model", "new", "sessions", "connect"} {
		if !sugg[want] {
			t.Fatalf("Suggested group missing %q: %v", want, sugg)
		}
	}
	// (3) the plain category buckets follow (all five headers present).
	header := map[string]bool{}
	for _, ln := range lines {
		if ln.opt == -1 {
			header[strings.Trim(stripANSI(ln.text), " ")] = true
		}
	}
	for _, want := range []string{"General", "Session", "Model", "Provider", "Agent"} {
		if !header[want] {
			t.Fatalf("category header %q missing: %v", want, header)
		}
	}
}

// TestPaletteSuggestedCollapsedOnFilter pins the non-empty-filter collapse
// (the ported list()): filtered() skips the Suggested rows on any needle, so
// the Suggested group is empty-filter-only; the seeded commands still match
// via their plain entries.
func TestPaletteSuggestedCollapsedOnFilter(t *testing.T) {
	a := paletteTestApp(protocol.Session{ID: "s1"})
	a.route = routeSession
	m := selectNew("Commands", "Filter commands", paletteOptions(a.App), nil, nil, nil)
	// empty filter: the Suggested rows are present.
	if !hasSuggested(m.filtered()) {
		t.Fatal("empty filter: the Suggested rows must be present")
	}
	// non-empty filter: the Suggested rows collapse.
	m.filter = "model"
	if hasSuggested(m.filtered()) {
		t.Fatal("non-empty filter: the Suggested rows must collapse")
	}
	// the seeded command still matches via its plain entry.
	if !hasTitle(m.filtered(), "model") {
		t.Fatal("non-empty filter: /model must still match via its plain entry")
	}
}

// TestPaletteSuggestedPickUnchanged pins that the Suggested rows are real
// command rows (decision B): the value is the plain command name (no
// suggested: prefix) and picking one runs the command unchanged.
func TestPaletteSuggestedPickUnchanged(t *testing.T) {
	a := paletteTestApp()
	a.route = routeHome
	a.openPaletteDialog()
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgPalette {
		t.Fatal("the palette must be on top")
	}
	m := d.sel
	// find the Suggested /model row (the always seed).
	want := -1
	for i, o := range m.filtered() {
		if o.category == "Suggested" && o.title == "model" {
			want = i
			break
		}
	}
	if want < 0 {
		t.Fatal("no Suggested /model row in the palette")
	}
	// the value is the plain command name (no suggested: prefix).
	if v := m.filtered()[want].value; v != "/model" {
		t.Fatalf("Suggested /model value = %v, want /model (the plain command name)", v)
	}
	m.sel = want
	m.submit(a.App)
	// picking it runs /model unchanged → the model dialog.
	d, ok = a.dlg.top()
	if !ok || d.kind != dlgModel {
		t.Fatalf("after the Suggested /model pick: top=%+v (ok=%v), want the model dialog", d, ok)
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
		// home chrome min = logo 8 + box 5 + hint 1 = 14 > 24/4 = 6 →
		// panelTop = the chromeMin clamp (14)
		panelTop = 14
	}
	// inner lines = title + filter + 6 visible rows (24/2−6) + footer = 9.
	// The panel = the top-padding line + min(9, avail) inner lines (the
	// viewModal clamp, avail = 24−panelTop−1): home avail = 24−14−1 = 9 →
	// 9 lines (the footer hint row is clamped out); session avail = 24−6−1
	// = 17 → 10 lines (unclamped, the footer hint shown).
	const inner = 9
	avail := 24 - panelTop - 1
	if avail < 1 {
		avail = 1
	}
	n := min(inner+1, avail)
	clamped := n < inner+1
	panelBottom := panelTop + n - 1
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
	// the footer hint (ctrl+p commands) is the panel's last inner line —
	// shown only when the panel is not clamped (the home 80x24 case clamps
	// it out; the session route is unclamped).
	if !clamped {
		footerPlain := strings.TrimRight(stripANSI(lines[panelBottom]), " ")
		if !strings.HasSuffix(footerPlain, "ctrl+p commands") {
			t.Fatalf("footer row = %q, want the ctrl+p commands hint right-aligned", footerPlain)
		}
	}
	if strings.Contains(a.view(), "\u2191/\u2193 move") {
		t.Fatal("the generic nav hint must be replaced by the palette footer hint")
	}
	// (d) the filter-input row is unchanged (the placeholder).
	if !strings.Contains(stripANSI(lines[panelTop+2]), "Filter commands") {
		t.Fatalf("filter row = %q, want the Filter commands placeholder", stripANSI(lines[panelTop+2]))
	}
}

// TestPaletteMouseHover drives a mouse motion over a visible row of the open
// palette (S4 mouse, spec §2.7 — decision 4) and asserts the selection moves
// to the hovered row. The teatest leg sends a tea.MouseMsg at the S4 anchor
// (the first window row is panelTop+3 — below the panel's top-padding line,
// the title row and the filter row — spec §2.7) for a non-selected row; the
// selection is asserted on the model after the program has quit (not a race
// with the running program — the TestPromptSlashMouseHover idiom).
func TestPaletteMouseHover(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine(homeLogoLine), teatest.WithDuration(5*time.Second))

	// ctrl+p opens the palette (the S4.2 remap).
	tm.Send(pressCtrlP())
	teatest.WaitFor(t, tm.Output(), hasLine("Commands"), teatest.WithDuration(5*time.Second))

	// the S4 anchor (spec §2.7): the palette is a fullscreen modal
	// (viewModal) — the window rows sit below the panel's top-padding line
	// (row panelTop), the title row (panelTop+1) and the filter row
	// (panelTop+2); the first window row is panelTop+3 and the window
	// renders h/2−6 rows (selectModel.view).
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgPalette || d.sel == nil {
		t.Fatal("the palette must be on top")
	}
	h := a.size.Height
	if h < 1 {
		h = 24
	}
	panelTop := max(h/4, a.modalChromeMin())
	visible := h/2 - 6
	if visible < 1 {
		visible = 1
	}
	// the hover target: the first window row over a selectable option (the
	// category headers/blank rows carry opt -1 and are not hoverable) that
	// is not the current selection. The window is re-anchored on the
	// selection, so the row offset is added to the CURRENT window top
	// (d.sel.top — set by the last render).
	w := a.size.Width
	if w < 1 {
		w = 80
	}
	panelW := int(d.size.width())
	if panelW > w-2 {
		panelW = w - 2
	}
	lines := d.sel.buildLines(panelW, a.theme)
	target, want := -1, -1
	for winRow := 0; winRow < visible && target < 0; winRow++ {
		lineIdx := d.sel.top + winRow
		if lineIdx < len(lines) && lines[lineIdx].opt >= 0 && lines[lineIdx].opt != d.sel.sel {
			target = winRow
			want = lines[lineIdx].opt
		}
	}
	if target < 0 {
		t.Fatalf("no non-selected selectable row in the palette window (visible=%d)", visible)
	}

	// a motion over the row moves the selection there.
	tm.Send(tea.MouseMotionMsg{X: 30, Y: panelTop + 3 + target, Button: tea.MouseNone})

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	d, ok = a.dlg.top()
	if !ok || d.kind != dlgPalette {
		t.Fatalf("after the hover: top=%+v (ok=%v), want the palette still open", d, ok)
	}
	if d.sel.sel != want {
		t.Fatalf("sel after hovering window row %d = %d, want %d", target, d.sel.sel, want)
	}
}

// TestPaletteMouseClick drives a mouse click on the /model row of the open
// palette (the always-seeded Suggested entry — the TestPaletteSelectPick
// idiom with the enter key replaced by the mouse click) and asserts the
// command runs (the run-on-enter effect — the model dialog opens) and the
// palette closes. The click runs the SAME enter path as the key handler
// (submit → paletteSelectPick).
func TestPaletteMouseClick(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine(homeLogoLine), teatest.WithDuration(5*time.Second))
	tm.Send(pressCtrlP())
	teatest.WaitFor(t, tm.Output(), hasLine("Commands"), teatest.WithDuration(5*time.Second))

	// the S4 anchor (spec §2.7): the same derivation as TestPaletteMouseHover
	// — the first window row is panelTop+3, the window renders h/2−6 rows.
	d, ok := a.dlg.top()
	if !ok || d.kind != dlgPalette || d.sel == nil {
		t.Fatal("the palette must be on top")
	}
	h := a.size.Height
	if h < 1 {
		h = 24
	}
	panelTop := max(h/4, a.modalChromeMin())
	visible := h/2 - 6
	if visible < 1 {
		visible = 1
	}
	w := a.size.Width
	if w < 1 {
		w = 80
	}
	panelW := int(d.size.width())
	if panelW > w-2 {
		panelW = w - 2
	}
	lines := d.sel.buildLines(panelW, a.theme)
	// the click target: the window row over the /model option (the always
	// Suggested seed — deterministic across fixtures).
	target := -1
	for winRow := 0; winRow < visible; winRow++ {
		lineIdx := d.sel.top + winRow
		if lineIdx >= len(lines) || lines[lineIdx].opt < 0 {
			continue
		}
		if d.sel.filtered()[lines[lineIdx].opt].value == "/model" {
			target = winRow
			break
		}
	}
	if target < 0 {
		t.Fatalf("no /model row in the palette window (visible=%d)", visible)
	}

	// a click on the row runs the command (the enter action) and closes the
	// palette.
	tm.Send(tea.MouseClickMsg{X: 30, Y: panelTop + 3 + target, Button: tea.MouseLeft})

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	d, ok = a.dlg.top()
	if !ok || d.kind != dlgModel {
		t.Fatalf("after the click: top=%+v (ok=%v), want the model dialog (the palette closed)", d, ok)
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

// TestPaletteKeyTable pins the 0.10.0 palette key table (spec §2.8 — decision
// E, S5): one leg per row. The table is already ported (the select's keys are
// matched directly by selectModel.handleKey, off the registry — deviation
// 211): the legs are verification; a failing leg is the gap to fix.
func TestPaletteKeyTable(t *testing.T) {
	t.Run("ctrl+p opens the palette (both routes)", func(t *testing.T) {
		home := paletteTestApp()
		home.handleKey(pressCtrlP()) // command_list → the palette (the live key ladder)
		if d, ok := home.dlg.top(); !ok || d.kind != dlgPalette {
			t.Fatalf("home route: after ctrl+p top=%+v (ok=%v), want the palette", d, ok)
		}
		sess := paletteTestApp()
		sess.route = routeSession
		sess.handleKey(pressCtrlP())
		if d, ok := sess.dlg.top(); !ok || d.kind != dlgPalette {
			t.Fatalf("session route: after ctrl+p top=%+v (ok=%v), want the palette", d, ok)
		}
	})

	// The registry's prompt.autocomplete.prev/next entries (up,ctrl+p /
	// down,ctrl+n, keymap.go) are ported-but-inert names: with the palette
	// open the dialog owns every key (the registry is not consulted) and the
	// select's prev/next is the arrow (deviation 211 — the select's keys stay
	// off the registry). ctrl+p/ctrl+n fall to the filter input: the query
	// gains the literal character and the selection never takes the nav step
	// (up would wrap 0 → last; down would give 1) — the sel=0 is the
	// syncFilter reset, not a nav move.
	t.Run("ctrl+p/ctrl+n type the filter (not the nav)", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			k    tea.KeyPressMsg
		}{{
			name: "ctrl+p",
			k:    tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl, Text: "p"},
		}, {
			name: "ctrl+n",
			k:    tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl, Text: "n"},
		}} {
			t.Run(tc.name, func(t *testing.T) {
				a := paletteTestApp()
				a.openPaletteDialog()
				d, ok := a.dlg.top()
				if !ok || d.kind != dlgPalette {
					t.Fatal("the palette must be on top")
				}
				m := d.sel
				a.handleKey(tc.k)
				if v := m.input.Value(); v != string(tc.k.Text) {
					t.Fatalf("filter = %q, want the literal %q (the key goes to the filter input)", v, tc.k.Text)
				}
				if m.sel != 0 {
					t.Fatalf("sel = %d, want 0 (not the nav step — the arrows are the prev/next)", m.sel)
				}
				if d2, ok := a.dlg.top(); !ok || d2.kind != dlgPalette {
					t.Fatalf("after %s: top=%+v (ok=%v), want the palette still open (no re-open)", tc.name, d2, ok)
				}
			})
		}
	})

	t.Run("up/down wrap", func(t *testing.T) {
		a := paletteTestApp()
		a.openPaletteDialog()
		d, ok := a.dlg.top()
		if !ok {
			t.Fatal("the palette must be on top")
		}
		m := d.sel
		n := len(m.filtered())
		m.sel = n - 1
		m.handleKey(a.App, downKey)
		if m.sel != 0 {
			t.Fatalf("down on the last row = %d, want 0 (wrap)", m.sel)
		}
		m.handleKey(a.App, upKey)
		if m.sel != n-1 {
			t.Fatalf("up on the first row = %d, want the last (%d) (wrap)", m.sel, n-1)
		}
	})

	t.Run("enter runs the selected command", func(t *testing.T) {
		a := paletteTestApp()
		a.openPaletteDialog()
		d, ok := a.dlg.top()
		if !ok {
			t.Fatal("the palette must be on top")
		}
		m := d.sel
		// pick the plain /sessions option (the Suggested group may precede
		// it, so its index is found, not assumed).
		want := -1
		for i, o := range m.filtered() {
			if o.value == "/sessions" {
				want = i
				break
			}
		}
		if want < 0 {
			t.Fatal("no /sessions option in the palette")
		}
		m.sel = want
		a.handleKey(enterKey) // submit → paletteSelectPick (the run-on-enter)
		d2, ok := a.dlg.top()
		if !ok || d2.kind != dlgSessions {
			t.Fatalf("after enter: top=%+v (ok=%v), want the session-list dialog (the palette ran /sessions and closed)", d2, ok)
		}
	})

	t.Run("esc/ctrl+c close without running", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			k    tea.KeyPressMsg
		}{{
			name: "esc",
			k:    press(tea.KeyEscape),
		}, {
			name: "ctrl+c",
			k:    ctrlCKey,
		}} {
			t.Run(tc.name, func(t *testing.T) {
				a := paletteTestApp()
				a.openPaletteDialog()
				d, ok := a.dlg.top()
				if !ok {
					t.Fatal("the palette must be on top")
				}
				d.sel.sel = 3 // a real option: a run would push its dialog
				a.handleKey(tc.k)
				if !a.dlg.empty() {
					if d2, ok := a.dlg.top(); ok {
						t.Fatalf("after %s: top=%+v, want the palette closed with nothing run (ctrl+c must not arm the quit)", tc.name, d2)
					}
					t.Fatalf("after %s: the dialog stack is not empty", tc.name)
				}
				if v := a.prompt.input.Value(); v != "" {
					t.Fatalf("after %s: the prompt input = %q, want it untouched", tc.name, v)
				}
			})
		}
	})

	t.Run("pgup/pgdown ±10 (flat, deviation 176)", func(t *testing.T) {
		// The env-machined acceleration is not ported: the window shifts a
		// flat ±10 rows (the selection stays). The palette pool (11 options
		// — more built rows with the category headers) is larger than 10, so
		// the shift is visible.
		a := paletteTestApp()
		a.openPaletteDialog()
		d, ok := a.dlg.top()
		if !ok {
			t.Fatal("the palette must be on top")
		}
		m := d.sel
		m.view(60, 24, a.theme) // the first render anchors the window at the top
		if m.top != 0 {
			t.Fatalf("initial top = %d, want 0", m.top)
		}
		a.handleKey(selPgDnMsg)
		m.view(60, 24, a.theme)
		if m.top != 10 {
			t.Fatalf("pgdown: top = %d, want 10 (the flat ±10)", m.top)
		}
		a.handleKey(selPgUpMsg)
		m.view(60, 24, a.theme)
		if m.top != 0 {
			t.Fatalf("pgup: top = %d, want 0 (the flat ±10 back)", m.top)
		}
	})

	t.Run("home/end jump", func(t *testing.T) {
		a := paletteTestApp()
		a.openPaletteDialog()
		d, ok := a.dlg.top()
		if !ok {
			t.Fatal("the palette must be on top")
		}
		m := d.sel
		n := len(m.filtered())
		m.sel = n / 2
		m.handleKey(a.App, homeKeyTest)
		if m.sel != 0 {
			t.Fatalf("home: sel = %d, want 0 (jump to the top)", m.sel)
		}
		m.handleKey(a.App, endKey)
		if m.sel != n-1 {
			t.Fatalf("end: sel = %d, want the last (%d) (jump to the bottom)", m.sel, n-1)
		}
	})

	t.Run("tab/shift+tab no-op", func(t *testing.T) {
		// The palette select has no footer actions: focusAction is a no-op
		// (the selection is unchanged); the slash-menu tab-complete is
		// dialog-gated (the dialog owns the keys while the palette is open).
		a := paletteTestApp()
		a.openPaletteDialog()
		d, ok := a.dlg.top()
		if !ok {
			t.Fatal("the palette must be on top")
		}
		m := d.sel
		m.sel = 3
		a.handleKey(selTabMsg)
		if m.sel != 3 || m.focAct != -1 {
			t.Fatalf("tab: sel=%d focAct=%d, want 3/-1 (no footer actions)", m.sel, m.focAct)
		}
		a.handleKey(selShiftTabMsg)
		if m.sel != 3 || m.focAct != -1 {
			t.Fatalf("shift+tab: sel=%d focAct=%d, want 3/-1 (no footer actions)", m.sel, m.focAct)
		}
		if d2, ok := a.dlg.top(); !ok || d2.kind != dlgPalette {
			t.Fatalf("after tab: top=%+v (ok=%v), want the palette still open", d2, ok)
		}
	})

	t.Run("sel resets on query change", func(t *testing.T) {
		// syncFilter: a non-empty needle resets the selection to 0 (the
		// ported filter-rerun reset, select.go).
		a := paletteTestApp()
		a.openPaletteDialog()
		d, ok := a.dlg.top()
		if !ok {
			t.Fatal("the palette must be on top")
		}
		m := d.sel
		m.sel = 3
		a.handleKey(press('m'))
		if m.filter != "m" {
			t.Fatalf("filter = %q, want m (the typed char feeds the filter)", m.filter)
		}
		if m.sel != 0 {
			t.Fatalf("sel after the query change = %d, want 0 (the reset)", m.sel)
		}
	})

	t.Run("empty text (No results found)", func(t *testing.T) {
		// The empty state is already parity (select.go renders
		// "  No results found" in TextMuted — locked decision 3): this leg
		// verifies it with an impossible query on the real palette select
		// (no change).
		a := paletteTestApp()
		a.openPaletteDialog()
		d, ok := a.dlg.top()
		if !ok {
			t.Fatal("the palette must be on top")
		}
		m := d.sel
		m.input.SetValue("zzzzqq") // nothing matches by title or category
		lines := strings.Split(m.view(60, 24, a.theme), "\n")
		if got := stripANSI(lines[len(lines)-1]); got != "  No results found" {
			t.Fatalf("empty-state line = %q, want the exact pinned text %q", got, "  No results found")
		}
	})
}
