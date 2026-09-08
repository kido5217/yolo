package tui

// home_mock_test.go — yolo-dhf.3 (0.8.0 start-screen parity): the MOCK of
// the NEW home route (start screen) at a real 200x50 terminal size, for
// user reaction BEFORE the implementation. This is a mock, not the route:
// it hand-assembles the target layout from lipgloss blocks — the REAL
// logo art (renderLogo), the REAL keymap shortcut format (NewKeymap), the
// REAL tip markup parser (parseTip), and the resolved yolo dark theme
// (theme engine) — and renders the frame (ANSI preserved) to
// docs/superpowers/mockups/home-mock-200x50.txt (deterministic, so the
// test never leaves the tree dirty).
//
// Geometry = the upstream v1.18.18 home layout (strict-copy bar, spec
// 2026-08-24-opencode-tui-parity-design.md), ported from:
//
//	routes/home.tsx:72               root paddingLeft/Right 2, alignItems center
//	routes/home.tsx:73-87            spacer(grow) · h4 shrink · logo · h1 shrink ·
//	                                 prompt (maxWidth 75, paddingTop 1) · home_bottom ·
//	                                 spacer(grow)
//	prompt/index.tsx:1350-1512       the left-border box (┃ + the backgroundElement
//	                                 interior fill + the ╹/▀ bottom row)
//	prompt/index.tsx:1444-1484       the meta line (gap 1: agent · model provider)
//	prompt/index.tsx:1513-1690       the hint row (single child → flex-start at the
//	                                 box left edge; gap 2 between the segments)
//	feature-plugins/home/tips.tsx:27 the tip box (maxWidth 75, centered,
//	                                 paddingTop 3)
//	feature-plugins/home/footer.tsx:64-82 the footer (pad 1/1, padL/R 2,
//	                                 directory left, version right)
//
// Spacing ground truth: the pinned 1.18.18 capture
// testdata/parity/upstream/home.screen.json (80x24). Its centering
// convention — the extra column goes to the FIRST item (logo left margin
// 19 = ceil(18.5), box left margin 1 = ceil(0.5), the hint origin at the
// border col) — is extrapolated to 200x50 for every odd margin and the
// 14/13 free-row split (the fixture's even split has no odd case).
//
// Strings = the yolo-dhf.2 standing decisions: the placeholder (the
// upstream pool[0] + the "Ask anything..." prefix), the meta line (agent
// Titlecase, the auto word OMITTED — decision 6; the yolo catalog
// default model/provider), the verbatim hint, the tab-agents tip (the
// pool entry, decision 1), the zero-MCP footer (the segment hidden —
// decision 5) and the plain semver version (epic note).

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// mockW/mockH are the mock frame size (the task's real terminal size).
const (
	mockW = 200
	mockH = 50
)

// mock geometry — every value cites its source in the file header.
const (
	// the root box padding (home.tsx:72): the content area is cols
	// 2..197 (196 wide) over rows 0..46 (47 rows — the footer box owns
	// the bottom 3 rows, footer.tsx:66-80).
	mockContentL = 2
	mockContentW = 196

	// the vertical stack (rows): 14 free-top + 4 (h4, home.tsx:74) +
	// 4 (logo) + 1 (h1, home.tsx:80) + 1 (wrapper paddingTop,
	// home.tsx:81) + 5 (box) + 1 (hint) + 3 (tip paddingTop,
	// tips.tsx:27) + 1 (tip) + 13 free-bottom = 47. The free 27
	// (47 - the 20 fixed) splits 14/13 — ceil-first (the fixture
	// convention).
	mockLogoTop = 18
	mockBoxTop  = mockLogoTop + 4 + 1 + 1
	mockHintRow = mockBoxTop + 5
	mockTipRow  = mockHintRow + 1 + 3

	// the footer (footer.tsx:64-82): pad 1 / content / pad 1 → the
	// content row is mockH-2.
	mockFooterRow = mockH - 2

	// the horizontal positions (cols): the 19-wide logo and the 75-wide
	// prompt box centered over the 196-wide content area — the odd
	// margin goes LEFT (ceil-first). The logo spans 91..109, the box
	// 63..137; the box left border col is the hint line origin (the
	// fixture row 16 starts at the border col).
	mockBoxW   = 75 // the prompt maxWidth (home.tsx:36 default)
	mockBoxPad = 2  // the box interior paddingLeft/Right (prompt/index.tsx:1361-1362)

	mockLogoL = mockContentL + (mockContentW-logoWidth+1)/2 // 91
	mockBoxL  = mockContentL + (mockContentW-mockBoxW+1)/2  // 63

	// the 75-wide tip box (tips.tsx:27) is centered like the prompt
	// box (cols 63..137); the 54-col tip line centers inside it
	// (ceil-first) → col 74.
	mockTipL = 74

	// the version right-aligned at the paddingRight edge (col 197 —
	// the fixture row 22 version ends at col 77 = 79-2).
	mockVersion  = "0.8.0"
	mockVersionL = mockContentL + mockContentW - len(mockVersion) // 193
)

// mock content strings (the yolo-dhf.2 decisions + the yolo defaults —
// see the file header for the sources).
const (
	mockPlaceholder = "Ask anything... \"Fix a TODO in the codebase\""
	mockAgent       = "Build"
	mockModel       = "Qwen3.8-27B"
	mockProvider    = "kido"
	mockTipText     = "Press {highlight}tab{/highlight} to cycle between Build and Plan agents"
	mockDir         = "~/network/projects/yolo:main"
)

// mockRun is one styled run of a mock frame row (col = the display
// column of the first cell; a nil fg/bg = the plain pen — the terminal
// default).
type mockRun struct {
	col  int
	text string
	fg   color.Color
	bg   color.Color
}

// mockRow renders one mockW-wide frame row from ordered,
// non-overlapping runs: each run styled (fg/bg) via lipgloss, the gaps
// and the tail plain spaces. (lipgloss v2 Render appends a trailing SGR
// reset after each styled run, so the gaps pick up the default pen —
// the internal/tui/AGENTS.md house note.)
func mockRow(runs ...mockRun) string {
	var b strings.Builder
	col := 0
	for _, r := range runs {
		if r.col > col {
			b.WriteString(strings.Repeat(" ", r.col-col))
		}
		if r.fg == nil && r.bg == nil {
			b.WriteString(r.text)
		} else {
			st := lipgloss.NewStyle()
			if r.fg != nil {
				st = st.Foreground(r.fg)
			}
			if r.bg != nil {
				st = st.Background(r.bg)
			}
			b.WriteString(st.Render(r.text))
		}
		col = r.col + runeWidth(r.text)
	}
	if col < mockW {
		b.WriteString(strings.Repeat(" ", mockW-col))
	}
	return b.String()
}

// rowSegment extracts n display cols starting at col (every frame rune
// is one column — box drawing included).
func rowSegment(row string, col, n int) string {
	var b strings.Builder
	c := 0
	for _, r := range row {
		if c >= col+n {
			break
		}
		if c >= col {
			b.WriteRune(r)
		}
		c++
	}
	return b.String()
}

func TestHomeMockRender(t *testing.T) {
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
	th, err := e.ActiveTheme()
	if err != nil {
		t.Fatalf("ActiveTheme: %v", err)
	}
	if th.Name != "yolo" || th.Mode != "dark" {
		t.Fatalf("active theme = %s %s, want yolo dark (no config, no KV, no palette)", th.Name, th.Mode)
	}
	col := func(name string) color.Color {
		c, ok := th.Color(name)
		if !ok {
			t.Fatalf("theme %q lacks token %s", th.Name, name)
		}
		return lipgloss.Color(c.Hex()[:7])
	}
	text := col("text")
	muted := col("textMuted")
	warning := col("warning")
	// agentColor is the secondary token — the fixture's agent/border
	// color #5c9cf5 (opencode darkSecondary; borderHighlight = the
	// agent color at full fade, prompt/index.tsx:1309). The yolo wire
	// carries no per-agent color yet, so the mock refers to the theme
	// token.
	agentColor := col("secondary")
	bgEl := col("backgroundElement")

	km, err := NewKeymap(nil)
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	agentK := km.Format("agent_cycle")
	paletteK := km.Format("command_list")
	if agentK != "tab" || paletteK != "ctrl+p" {
		t.Fatalf("keymap defaults drifted: agent_cycle=%q command_list=%q", agentK, paletteK)
	}

	blank := strings.Repeat(" ", mockW)
	frames := make([]string, mockH)
	for i := range frames {
		frames[i] = blank
	}

	// the logo (renderLogo — the real YOLO art), centered at mockLogoL.
	logoLines := strings.Split(renderLogo(th), "\n")
	if len(logoLines) != 4 {
		t.Fatalf("logo lines = %d, want 4", len(logoLines))
	}
	for i, l := range logoLines {
		frames[mockLogoTop+i] = strings.Repeat(" ", mockLogoL) + l + strings.Repeat(" ", mockW-mockLogoL-logoWidth)
	}

	// the prompt box (rows mockBoxTop..+4): the ┃ border (fg
	// agentColor) + the backgroundElement interior fill (cols
	// mockBoxL+1..mockBoxL+mockBoxW-1).
	border := mockRun{mockBoxL, "┃", agentColor, nil}
	fill := mockRun{mockBoxL + 1, strings.Repeat(" ", mockBoxW-1), nil, bgEl}
	pad := mockRun{mockBoxL + 1, strings.Repeat(" ", mockBoxPad), nil, bgEl}
	m := mockBoxL + 1 + mockBoxPad
	frames[mockBoxTop] = mockRow(border, fill)
	frames[mockBoxTop+1] = mockRow(border, pad,
		mockRun{m, mockPlaceholder, muted, bgEl},
		// the trailing run ends at the last interior col (the meta
		// row's idiom) — the content rows are width-exact at mockBoxW.
		mockRun{m + len(mockPlaceholder), strings.Repeat(" ", mockBoxL+mockBoxW-1-m-len(mockPlaceholder)+1), nil, bgEl})
	frames[mockBoxTop+2] = mockRow(border, fill)

	// the meta line (the fixture row 14, gap 1: agent · model
	// provider — the auto word omitted, decision 6).
	metaRuns := []mockRun{border, pad}
	mc := m
	addMeta := func(s string, fg color.Color) {
		metaRuns = append(metaRuns, mockRun{mc, s, fg, bgEl})
		mc += runeWidth(s) // · is 2 bytes / 1 col (the house note)
	}
	addMeta(mockAgent, agentColor)
	addMeta(" ", nil)
	addMeta("·", muted)
	addMeta(" ", nil)
	addMeta(mockModel, text)
	addMeta(" ", nil)
	addMeta(mockProvider, muted)
	metaRuns = append(metaRuns, mockRun{mc, strings.Repeat(" ", mockBoxL+mockBoxW-1-mc+1), nil, bgEl})
	frames[mockBoxTop+3] = mockRow(metaRuns...)
	frames[mockBoxTop+4] = mockRow(
		mockRun{mockBoxL, "╹", agentColor, nil},
		mockRun{mockBoxL + 1, strings.Repeat("▀", mockBoxW-1), bgEl, nil},
	)

	// the hint row (the fixture row 16): the segments at the box left
	// edge, the shortcuts in text and the words muted, the 2-col gap.
	seg1 := agentK + " agents"
	frames[mockHintRow] = mockRow(
		mockRun{mockBoxL, agentK, text, nil},
		mockRun{mockBoxL + len(agentK), " agents", muted, nil},
		mockRun{mockBoxL + len(seg1), "  ", nil, nil},
		mockRun{mockBoxL + len(seg1) + 2, paletteK, text, nil},
		mockRun{mockBoxL + len(seg1) + 2 + len(paletteK), " commands", muted, nil},
	)

	// the tip row (tips-view.tsx:150-161): the "● Tip " prefix in
	// warning, the parts muted, the highlight runs in text — centered
	// at mockTipL.
	tipRuns := []mockRun{{mockTipL, "● Tip ", warning, nil}}
	c := mockTipL + runeWidth("● Tip ")
	for _, p := range parseTip(mockTipText) {
		fg := muted
		if p.hi {
			fg = text
		}
		tipRuns = append(tipRuns, mockRun{c, p.text, fg, nil})
		c += len(p.text)
	}
	frames[mockTipRow] = mockRow(tipRuns...)

	// the footer row (the fixture row 22): the abbreviated
	// path:branch directory muted at the paddingLeft col, the plain
	// semver version muted at the right edge. (The zero-MCP machine
	// renders NO MCP segment — decision 5.)
	frames[mockFooterRow] = mockRow(
		mockRun{mockContentL, mockDir, muted, nil},
		mockRun{mockVersionL, mockVersion, muted, nil},
	)

	render := strings.Join(frames, "\n")

	// the geometry contract: mockH rows, every row mockW display cols.
	rows := strings.Split(stripANSI(render), "\n")
	if len(rows) != mockH {
		t.Fatalf("frame rows = %d, want %d", len(rows), mockH)
	}
	for i, r := range rows {
		if w := runeWidth(r); w != mockW {
			t.Fatalf("row %d width = %d, want %d", i, w, mockW)
		}
	}
	at := func(row, col int, want string) {
		t.Helper()
		if got := rowSegment(rows[row], col, runeWidth(want)); got != want {
			t.Fatalf("row %d col %d = %q, want %q", row, col, got, want)
		}
	}
	at(mockLogoTop+1, mockLogoL, logoLeft[1])
	at(mockBoxTop+1, m, "Ask anything... ")
	at(mockBoxTop+3, m, mockAgent)
	at(mockBoxTop+3, m+len(mockAgent)+1, "·")
	at(mockBoxTop+3, m+len(mockAgent)+3, mockModel)
	at(mockBoxTop+4, mockBoxL, "╹")
	at(mockBoxTop+4, mockBoxL+1, "▀▀")
	at(mockHintRow, mockBoxL, agentK+" agents  "+paletteK+" commands")
	at(mockTipRow, mockTipL, "● Tip Press tab")
	at(mockFooterRow, mockContentL, mockDir)
	at(mockFooterRow, mockVersionL, mockVersion)

	// the ANSI is preserved (truecolor fg + the box interior bg).
	if !strings.Contains(render, "38;2;") || !strings.Contains(render, "48;2;") {
		t.Fatal("mock render lost its ANSI (lipgloss stripped the styles)")
	}

	out := filepath.Join("..", "..", "docs", "superpowers", "mockups", "home-mock-200x50.txt")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(out, []byte(render+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Log("mock written to", out)
}

// slashOpenItems is the command list the slash-open mock shows (a few
// representative commands; the mock is hand-assembled, not driven by the
// command catalog). slashSel marks the highlighted row.
var slashOpenItems = []dropdownRow{
	{label: "/help", description: "Show the help dialog"},
	{label: "/model", description: "List models"},
	{label: "/agents", description: "List agents"},
	{label: "/status", description: "Show session status"},
	{label: "/themes", description: "Browse themes"},
}

// slashSel is the selected (highlighted) row in the slash-open mock.
const slashSel = 1

// TestHomeMockSlashOpenRender is the SECOND mock (spec §6 S7, §5): the home
// frame with the slash dropdown OPEN above the box (the S3 anchor — the same
// left edge and width as the box, the logo overlaid while open) and the S6
// context-aware hint (the first segment reads "tab complete" while the menu
// is open). The dropdown box is the REAL shared primitive (newDropdown,
// dropdown.go) rendered at the box width; the rest of the frame is
// hand-assembled exactly as TestHomeMockRender (the clean home mock). It
// writes the deterministic ANSI frame to
// docs/superpowers/mockups/home-mock-200x50-slash-open.txt; the existing
// clean-home mock (home-mock-200x50.txt) is untouched.
func TestHomeMockSlashOpenRender(t *testing.T) {
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
	th, err := e.ActiveTheme()
	if err != nil {
		t.Fatalf("ActiveTheme: %v", err)
	}
	if th.Name != "yolo" || th.Mode != "dark" {
		t.Fatalf("active theme = %s %s, want yolo dark", th.Name, th.Mode)
	}
	col := func(name string) color.Color {
		c, ok := th.Color(name)
		if !ok {
			t.Fatalf("theme %q lacks token %s", th.Name, name)
		}
		return lipgloss.Color(c.Hex()[:7])
	}
	sgr := func(name string) string {
		c, ok := th.Color(name)
		if !ok {
			t.Fatalf("theme %q lacks token %s", th.Name, name)
		}
		return fmt.Sprintf("%d;%d;%d", int(c.R), int(c.G), int(c.B))
	}
	selFg := th.SelectedForeground()
	selSGR := fmt.Sprintf("%d;%d;%d", int(selFg.R), int(selFg.G), int(selFg.B))
	primSGR := sgr("primary")
	menuSGR := sgr("backgroundMenu")

	text := col("text")
	muted := col("textMuted")
	warning := col("warning")
	agentColor := col("secondary")
	bgEl := col("backgroundElement")

	km, err := NewKeymap(nil)
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	agentK := km.Format("agent_cycle")
	paletteK := km.Format("command_list")
	if agentK != "tab" || paletteK != "ctrl+p" {
		t.Fatalf("keymap defaults drifted: agent_cycle=%q command_list=%q", agentK, paletteK)
	}

	blank := strings.Repeat(" ", mockW)
	frames := make([]string, mockH)
	for i := range frames {
		frames[i] = blank
	}

	// the logo (renderLogo), centered at mockLogoL. The dropdown (below)
	// overlays its lower rows (the logo is overlaid while open, spec §3.1);
	// the top row stays visible above the dropdown.
	logoLines := strings.Split(renderLogo(th), "\n")
	if len(logoLines) != 4 {
		t.Fatalf("logo lines = %d, want 4", len(logoLines))
	}
	for i, l := range logoLines {
		frames[mockLogoTop+i] = strings.Repeat(" ", mockLogoL) + l + strings.Repeat(" ", mockW-mockLogoL-logoWidth)
	}

	// the prompt box (rows mockBoxTop..+4): the ┃ border (fg agentColor) +
	// the backgroundElement interior fill.
	border := mockRun{mockBoxL, "┃", agentColor, nil}
	fill := mockRun{mockBoxL + 1, strings.Repeat(" ", mockBoxW-1), nil, bgEl}
	pad := mockRun{mockBoxL + 1, strings.Repeat(" ", mockBoxPad), nil, bgEl}
	m := mockBoxL + 1 + mockBoxPad
	frames[mockBoxTop] = mockRow(border, fill)
	frames[mockBoxTop+1] = mockRow(border, pad,
		mockRun{m, mockPlaceholder, muted, bgEl},
		mockRun{m + len(mockPlaceholder), strings.Repeat(" ", mockBoxL+mockBoxW-1-m-len(mockPlaceholder)+1), nil, bgEl})
	frames[mockBoxTop+2] = mockRow(border, fill)

	// the meta line (agent · model provider — the auto word omitted).
	metaRuns := []mockRun{border, pad}
	mc := m
	addMeta := func(s string, fg color.Color) {
		metaRuns = append(metaRuns, mockRun{mc, s, fg, bgEl})
		mc += runeWidth(s)
	}
	addMeta(mockAgent, agentColor)
	addMeta(" ", nil)
	addMeta("·", muted)
	addMeta(" ", nil)
	addMeta(mockModel, text)
	addMeta(" ", nil)
	addMeta(mockProvider, muted)
	metaRuns = append(metaRuns, mockRun{mc, strings.Repeat(" ", mockBoxL+mockBoxW-1-mc+1), nil, bgEl})
	frames[mockBoxTop+3] = mockRow(metaRuns...)
	frames[mockBoxTop+4] = mockRow(
		mockRun{mockBoxL, "╹", agentColor, nil},
		mockRun{mockBoxL + 1, strings.Repeat("▀", mockBoxW-1), bgEl, nil},
	)

	// the hint row (S6 context-aware): the first segment is "tab complete"
	// while the slash menu is open (vs "tab agents" in the clean mock); the
	// ctrl+p commands segment is unchanged.
	seg1 := agentK + " complete"
	frames[mockHintRow] = mockRow(
		mockRun{mockBoxL, agentK, text, nil},
		mockRun{mockBoxL + len(agentK), " complete", muted, nil},
		mockRun{mockBoxL + len(seg1), "  ", nil, nil},
		mockRun{mockBoxL + len(seg1) + 2, paletteK, text, nil},
		mockRun{mockBoxL + len(seg1) + 2 + len(paletteK), " commands", muted, nil},
	)

	// the tip row + footer row: identical to the clean mock.
	tipRuns := []mockRun{{mockTipL, "● Tip ", warning, nil}}
	c := mockTipL + runeWidth("● Tip ")
	for _, p := range parseTip(mockTipText) {
		fg := muted
		if p.hi {
			fg = text
		}
		tipRuns = append(tipRuns, mockRun{c, p.text, fg, nil})
		c += len(p.text)
	}
	frames[mockTipRow] = mockRow(tipRuns...)
	frames[mockFooterRow] = mockRow(
		mockRun{mockContentL, mockDir, muted, nil},
		mockRun{mockVersionL, mockVersion, muted, nil},
	)

	// the slash dropdown (the S3 anchor): the REAL shared primitive rendered
	// at the box width, bottom-aligned above the box's top edge. Each box row
	// is mockBoxW cols; pad the leading margin to the box's left edge so the
	// box spans cols mockBoxL..mockBoxL+mockBoxW-1.
	box := newDropdown(slashOpenItems, slashSel, mockBoxW, len(slashOpenItems), th)
	dd := strings.Split(box.view(), "\n")
	if len(dd) != len(slashOpenItems) {
		t.Fatalf("dropdown = %d rows, want %d", len(dd), len(slashOpenItems))
	}
	ddTop := mockBoxTop - len(dd) // bottom-aligned above the box top edge
	for i, r := range dd {
		if w := runeWidth(stripANSI(r)); w != mockBoxW {
			t.Fatalf("dropdown row %d width = %d, want %d (the box width)", i, w, mockBoxW)
		}
		// the raw box row carries its SGR; pad both margins to the box's left
		// edge + width so the frame row is mockW display cols.
		frames[ddTop+i] = strings.Repeat(" ", mockBoxL) + r + strings.Repeat(" ", mockW-mockBoxL-mockBoxW)
	}

	render := strings.Join(frames, "\n")

	// geometry contract: mockH rows, every row mockW display cols.
	rows := strings.Split(stripANSI(render), "\n")
	rawRows := strings.Split(render, "\n")
	if len(rows) != mockH {
		t.Fatalf("frame rows = %d, want %d", len(rows), mockH)
	}
	for i, r := range rows {
		if w := runeWidth(r); w != mockW {
			t.Fatalf("row %d width = %d, want %d", i, w, mockW)
		}
	}
	at := func(row, coln int, want string) {
		t.Helper()
		if got := rowSegment(rows[row], coln, runeWidth(want)); got != want {
			t.Fatalf("row %d col %d = %q, want %q", row, coln, got, want)
		}
	}
	// the S3 anchor: a bordered box at the box's left edge, its last row just
	// above the box's top edge.
	at(ddTop, mockBoxL, "|")
	at(ddTop+len(dd)-1, mockBoxL, "|")
	at(ddTop+len(dd)-1, mockBoxL+mockBoxW-1, "|")
	// the selected row (slashSel = /model) carries the primary bg + the
	// SelectedForeground fg across its full width.
	selRow := rawRows[ddTop+slashSel]
	if !strings.Contains(selRow, "48;2;"+primSGR) {
		t.Fatalf("selected row missing the primary bg SGR (%s): %s", primSGR, selRow)
	}
	if !strings.Contains(selRow, "38;2;"+selSGR) {
		t.Fatalf("selected row missing the SelectedForeground SGR (%s): %s", selSGR, selRow)
	}
	// the logo is overlaid while open: the non-selected dropdown rows over the
	// logo (ddTop, ddTop+2) carry the backgroundMenu fill (the selected row,
	// ddTop+1, carries the primary bg — pinned above); the clean mock's logo
	// rows carry no interior bg.
	for _, k := range []int{0, 2} {
		if !strings.Contains(rawRows[ddTop+k], "48;2;"+menuSGR) {
			t.Fatalf("row %d missing the dropdown backgroundMenu fill (logo overlay): %s", ddTop+k, rawRows[ddTop+k])
		}
	}
	// the S6 context-aware hint + the unchanged box/tip/footer.
	at(mockHintRow, mockBoxL, agentK+" complete  "+paletteK+" commands")
	at(mockBoxTop+4, mockBoxL, "╹")
	at(mockTipRow, mockTipL, "● Tip Press tab")
	at(mockFooterRow, mockContentL, mockDir)
	at(mockFooterRow, mockVersionL, mockVersion)

	// the ANSI is preserved (truecolor fg + the dropdown/box interior bg).
	if !strings.Contains(render, "38;2;") || !strings.Contains(render, "48;2;") {
		t.Fatal("mock render lost its ANSI (lipgloss stripped the styles)")
	}

	out := filepath.Join("..", "..", "docs", "superpowers", "mockups", "home-mock-200x50-slash-open.txt")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(out, []byte(render+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Log("slash-open mock written to", out)
}

// mentionOpenItems is the @ picker's candidate list the mention-open mock
// shows (a few deterministic file rows + one directory row with the trailing-
// "/" kind marker; the mock is hand-assembled, not driven by the walk).
// mentionSel marks the highlighted row (the directory row).
var mentionOpenItems = []dropdownRow{
	{label: "alpha.go", description: ""},
	{label: "cmd/", description: ""},
	{label: "cmd/main.go", description: "cmd"},
	{label: "beta.go", description: ""},
}

// mentionSel is the selected (highlighted) row in the mention-open mock.
const mentionSel = 1

// TestHomeMockMentionOpenRender is the THIRD mock (spec §6 S6, §5): the home
// frame with the @ picker OPEN above the box (the S1 anchor — the same left
// edge and width as the box, the logo overlaid while open) and the S5
// context-aware hint (the first segment reads "tab complete" while the @ menu
// is open). The dropdown box is the REAL shared primitive (newDropdown,
// dropdown.go) rendered at the box width; the rest of the frame is
// hand-assembled exactly as TestHomeMockRender (the clean home mock) and
// TestHomeMockSlashOpenRender (the slash-open mock). It writes the
// deterministic ANSI frame to
// docs/superpowers/mockups/home-mock-200x50-mention-open.txt; the existing
// clean-home and slash-open mocks are untouched.
func TestHomeMockMentionOpenRender(t *testing.T) {
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
	th, err := e.ActiveTheme()
	if err != nil {
		t.Fatalf("ActiveTheme: %v", err)
	}
	if th.Name != "yolo" || th.Mode != "dark" {
		t.Fatalf("active theme = %s %s, want yolo dark", th.Name, th.Mode)
	}
	col := func(name string) color.Color {
		c, ok := th.Color(name)
		if !ok {
			t.Fatalf("theme %q lacks token %s", th.Name, name)
		}
		return lipgloss.Color(c.Hex()[:7])
	}
	sgr := func(name string) string {
		c, ok := th.Color(name)
		if !ok {
			t.Fatalf("theme %q lacks token %s", th.Name, name)
		}
		return fmt.Sprintf("%d;%d;%d", int(c.R), int(c.G), int(c.B))
	}
	selFg := th.SelectedForeground()
	selSGR := fmt.Sprintf("%d;%d;%d", int(selFg.R), int(selFg.G), int(selFg.B))
	primSGR := sgr("primary")
	menuSGR := sgr("backgroundMenu")

	text := col("text")
	muted := col("textMuted")
	warning := col("warning")
	agentColor := col("secondary")
	bgEl := col("backgroundElement")

	km, err := NewKeymap(nil)
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	agentK := km.Format("agent_cycle")
	paletteK := km.Format("command_list")
	if agentK != "tab" || paletteK != "ctrl+p" {
		t.Fatalf("keymap defaults drifted: agent_cycle=%q command_list=%q", agentK, paletteK)
	}

	blank := strings.Repeat(" ", mockW)
	frames := make([]string, mockH)
	for i := range frames {
		frames[i] = blank
	}

	// the logo (renderLogo), centered at mockLogoL. The dropdown (below)
	// overlays its lower rows (the logo is overlaid while open, spec §3.1);
	// the top row stays visible above the dropdown.
	logoLines := strings.Split(renderLogo(th), "\n")
	if len(logoLines) != 4 {
		t.Fatalf("logo lines = %d, want 4", len(logoLines))
	}
	for i, l := range logoLines {
		frames[mockLogoTop+i] = strings.Repeat(" ", mockLogoL) + l + strings.Repeat(" ", mockW-mockLogoL-logoWidth)
	}

	// the prompt box (rows mockBoxTop..+4): the ┃ border (fg agentColor) +
	// the backgroundElement interior fill.
	border := mockRun{mockBoxL, "┃", agentColor, nil}
	fill := mockRun{mockBoxL + 1, strings.Repeat(" ", mockBoxW-1), nil, bgEl}
	pad := mockRun{mockBoxL + 1, strings.Repeat(" ", mockBoxPad), nil, bgEl}
	m := mockBoxL + 1 + mockBoxPad
	frames[mockBoxTop] = mockRow(border, fill)
	frames[mockBoxTop+1] = mockRow(border, pad,
		mockRun{m, mockPlaceholder, muted, bgEl},
		mockRun{m + len(mockPlaceholder), strings.Repeat(" ", mockBoxL+mockBoxW-1-m-len(mockPlaceholder)+1), nil, bgEl})
	frames[mockBoxTop+2] = mockRow(border, fill)

	// the meta line (agent · model provider — the auto word omitted).
	metaRuns := []mockRun{border, pad}
	mc := m
	addMeta := func(s string, fg color.Color) {
		metaRuns = append(metaRuns, mockRun{mc, s, fg, bgEl})
		mc += runeWidth(s)
	}
	addMeta(mockAgent, agentColor)
	addMeta(" ", nil)
	addMeta("·", muted)
	addMeta(" ", nil)
	addMeta(mockModel, text)
	addMeta(" ", nil)
	addMeta(mockProvider, muted)
	metaRuns = append(metaRuns, mockRun{mc, strings.Repeat(" ", mockBoxL+mockBoxW-1-mc+1), nil, bgEl})
	frames[mockBoxTop+3] = mockRow(metaRuns...)
	frames[mockBoxTop+4] = mockRow(
		mockRun{mockBoxL, "╹", agentColor, nil},
		mockRun{mockBoxL + 1, strings.Repeat("▀", mockBoxW-1), bgEl, nil},
	)

	// the hint row (S5 context-aware): the first segment is "tab complete"
	// while the @ menu is open (vs "tab agents" in the clean mock); the
	// ctrl+p commands segment is unchanged.
	seg1 := agentK + " complete"
	frames[mockHintRow] = mockRow(
		mockRun{mockBoxL, agentK, text, nil},
		mockRun{mockBoxL + len(agentK), " complete", muted, nil},
		mockRun{mockBoxL + len(seg1), "  ", nil, nil},
		mockRun{mockBoxL + len(seg1) + 2, paletteK, text, nil},
		mockRun{mockBoxL + len(seg1) + 2 + len(paletteK), " commands", muted, nil},
	)

	// the tip row + footer row: identical to the clean mock.
	tipRuns := []mockRun{{mockTipL, "● Tip ", warning, nil}}
	c := mockTipL + runeWidth("● Tip ")
	for _, p := range parseTip(mockTipText) {
		fg := muted
		if p.hi {
			fg = text
		}
		tipRuns = append(tipRuns, mockRun{c, p.text, fg, nil})
		c += len(p.text)
	}
	frames[mockTipRow] = mockRow(tipRuns...)
	frames[mockFooterRow] = mockRow(
		mockRun{mockContentL, mockDir, muted, nil},
		mockRun{mockVersionL, mockVersion, muted, nil},
	)

	// the @ dropdown (the S1 anchor): the REAL shared primitive rendered at
	// the box width, bottom-aligned above the box's top edge. Each box row is
	// mockBoxW cols; pad the leading margin to the box's left edge so the box
	// spans cols mockBoxL..mockBoxL+mockBoxW-1.
	box := newDropdown(mentionOpenItems, mentionSel, mockBoxW, len(mentionOpenItems), th)
	dd := strings.Split(box.view(), "\n")
	if len(dd) != len(mentionOpenItems) {
		t.Fatalf("dropdown = %d rows, want %d", len(dd), len(mentionOpenItems))
	}
	ddTop := mockBoxTop - len(dd) // bottom-aligned above the box top edge
	for i, r := range dd {
		if w := runeWidth(stripANSI(r)); w != mockBoxW {
			t.Fatalf("dropdown row %d width = %d, want %d (the box width)", i, w, mockBoxW)
		}
		frames[ddTop+i] = strings.Repeat(" ", mockBoxL) + r + strings.Repeat(" ", mockW-mockBoxL-mockBoxW)
	}

	render := strings.Join(frames, "\n")

	// geometry contract: mockH rows, every row mockW display cols.
	rows := strings.Split(stripANSI(render), "\n")
	rawRows := strings.Split(render, "\n")
	if len(rows) != mockH {
		t.Fatalf("frame rows = %d, want %d", len(rows), mockH)
	}
	for i, r := range rows {
		if w := runeWidth(r); w != mockW {
			t.Fatalf("row %d width = %d, want %d", i, w, mockW)
		}
	}
	at := func(row, coln int, want string) {
		t.Helper()
		if got := rowSegment(rows[row], coln, runeWidth(want)); got != want {
			t.Fatalf("row %d col %d = %q, want %q", row, coln, got, want)
		}
	}
	// the S1 anchor: a bordered box at the box's left edge, its last row just
	// above the box's top edge.
	at(ddTop, mockBoxL, "|")
	at(ddTop+len(dd)-1, mockBoxL, "|")
	at(ddTop+len(dd)-1, mockBoxL+mockBoxW-1, "|")
	// the selected row (mentionSel = the cmd/ dir row) carries the primary bg
	// + the SelectedForeground fg across its full width.
	selRow := rawRows[ddTop+mentionSel]
	if !strings.Contains(selRow, "48;2;"+primSGR) {
		t.Fatalf("selected row missing the primary bg SGR (%s): %s", primSGR, selRow)
	}
	if !strings.Contains(selRow, "38;2;"+selSGR) {
		t.Fatalf("selected row missing the SelectedForeground SGR (%s): %s", selSGR, selRow)
	}
	// the logo is overlaid while open: the non-selected dropdown rows carry the
	// backgroundMenu fill (the dropdown paints over the logo rows); the
	// selected row carries the primary bg (pinned above).
	for i := 0; i < len(mentionOpenItems); i++ {
		if i == mentionSel {
			continue
		}
		if !strings.Contains(rawRows[ddTop+i], "48;2;"+menuSGR) {
			t.Fatalf("row %d missing the dropdown backgroundMenu fill (logo overlay): %s", ddTop+i, rawRows[ddTop+i])
		}
	}
	// the S5 context-aware hint + the unchanged box/tip/footer.
	at(mockHintRow, mockBoxL, agentK+" complete  "+paletteK+" commands")
	at(mockBoxTop+4, mockBoxL, "╹")
	at(mockTipRow, mockTipL, "● Tip Press tab")
	at(mockFooterRow, mockContentL, mockDir)
	at(mockFooterRow, mockVersionL, mockVersion)

	// the ANSI is preserved (truecolor fg + the dropdown/box interior bg).
	if !strings.Contains(render, "38;2;") || !strings.Contains(render, "48;2;") {
		t.Fatal("mock render lost its ANSI (lipgloss stripped the styles)")
	}

	out := filepath.Join("..", "..", "docs", "superpowers", "mockups", "home-mock-200x50-mention-open.txt")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(out, []byte(render+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Log("mention-open mock written to", out)
}
