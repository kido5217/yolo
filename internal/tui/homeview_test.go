package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// homeLogoLine is the home route's stable settle marker (the logo's second
// line, combined left + gap + right, 37 display cols). The 0.8.0 frame always
// renders the logo on home, so it replaces the retired "New session" list-row
// marker for the home-settle teatest WaitFors (package tui only — app_test.go,
// the external tui_test package, uses the literal).
var homeLogoLine = logoLeft[1] + " " + logoRight[1]

// atFrame is the whitebox frame assertion helper (row, col, want). The row is
// stripped (stripANSI) before the col slice — the SGR styling is zero-width
// glue, so the cols address the visible row.
func atFrame(t *testing.T, rows []string, row, col int, want string) {
	t.Helper()
	if row < 0 || row >= len(rows) {
		t.Fatalf("row %d out of range (frame has %d rows)", row, len(rows))
	}
	if got := rowSegment(stripANSI(rows[row]), col, runeWidth(want)); got != want {
		t.Fatalf("row %d col %d = %q, want %q", row, col, got, want)
	}
}

// fitsFrame asserts the frame is exactly h rows, each of width w display cols
// (the alt-screen fixed-frame contract). The width is measured on the
// stripped row (stripANSI — the SGR styling is zero-width glue, the
// package's ANSI-aware measurement idiom), so a styled row (the degraded
// bold selection) measures its visible width, not its escape bytes.
func fitsFrame(t *testing.T, out string, w, h int) []string {
	t.Helper()
	rows := strings.Split(out, "\n")
	if len(rows) != h {
		t.Fatalf("frame rows = %d, want %d", len(rows), h)
	}
	for i, r := range rows {
		if cw := runeWidth(stripANSI(r)); cw != w {
			t.Fatalf("row %d width = %d, want %d", i, cw, w)
		}
	}
	return rows
}

// TestHomeViewFrame pins the 0.8.0 start-screen frame geometry at 200x50 (the
// mock contract — the constants cited to home_mock_test.go). Zero-theme
// testApp (plain text, no SGR).
func TestHomeViewFrame(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: mockW, Height: mockH}
	a.Service.Dir = "/home/kido/network/projects/yolo"
	a.homeDirFunc = func() string { return "/home/kido" }
	a.branch = "main"
	a.version = "v0.8.0-4-gabcdef"

	rows := fitsFrame(t, a.homeView(nil, "", "", "", "", ""), mockW, mockH)
	// the logo (rows mockLogoTop..+3) at the mock's left margin.
	for i, l := range logoPlainLines() {
		atFrame(t, rows, mockLogoTop+i, mockLogoL, l)
	}
	// the box border ┃ at col mockBoxL, rows mockBoxTop..+3; ╹ on row +4.
	for i := 0; i < 4; i++ {
		atFrame(t, rows, mockBoxTop+i, mockBoxL, "┃")
	}
	atFrame(t, rows, mockBoxTop+4, mockBoxL, "╹")
	// the box interior (Task 5): the placeholder at the 2-pad col (the
	// pool's first entry), and the meta line (the testApp has no providers
	// and no config model → the agent segment alone).
	m := mockBoxL + 1 + mockBoxPad
	atFrame(t, rows, mockBoxTop+1, m, "Ask anything... ")
	atFrame(t, rows, mockBoxTop+3, m, "Build")
	// the hint row at the box edge (the defaults tab / ctrl+p, the 2-col
	// gap).
	atFrame(t, rows, mockHintRow, mockBoxL, "tab agents  ")
	// the tip row (the NO_MODELS nudge — testApp has no providers) centered in
	// the box: rowL = boxL + (boxW-rowW+1)/2 (ceil-first).
	tipPlain := "● Tip Run /connect to add an AI provider and start coding"
	tipL := mockBoxL + (mockBoxW-runeWidth(tipPlain)+1)/2
	atFrame(t, rows, mockTipRow, tipL, "● Tip Run /connect")
	// the footer content at row mockFooterRow: the dir (dir:branch) at col 2,
	// the plain-semver version at mockVersionL.
	atFrame(t, rows, mockFooterRow, mockContentL, "~/network/projects/yolo:main")
	atFrame(t, rows, mockFooterRow, mockVersionL, "0.8.0")
	// the spacers: 12 top (rows 0..11) + 11 bottom (rows 36..46) are blank
	// (w-wide space rows — the alt-screen fixed-frame contract).
	blank := strings.Repeat(" ", mockW)
	for i := 0; i < 12; i++ {
		if rows[i] != blank {
			t.Fatalf("top spacer row %d = %q, want blank", i, rows[i])
		}
	}
	for i := 36; i < 47; i++ {
		if rows[i] != blank {
			t.Fatalf("bottom spacer row %d = %q, want blank", i, rows[i])
		}
	}
}

// TestHomeViewFits80x24 pins the frame at the default terminal (24 rows, no
// top-drop). The boxW = 75 (80-4 = 76 >= 75) and boxL = 3 reflect the
// centering; the box rows land at 11..15 (free = 1 → top = 1).
func TestHomeViewFits80x24(t *testing.T) {
	t.Parallel()
	a := testApp() // NewApp default size 80x24
	rows := fitsFrame(t, a.homeView(nil, "", "", "", "", ""), 80, 24)
	// boxW = min(75, 80-4) = 75 (innerW = 75-1-2*2 = 70); boxL = 2 +
	// (76-75+1)/2 = 3.
	const boxL = 3
	for i := 0; i < 4; i++ {
		atFrame(t, rows, 11+i, boxL, "┃")
	}
	atFrame(t, rows, 15, boxL, "╹")
	// the logo (free = -3 → the top 3 rows are dropped: logo 1..8; the box
	// stays at 11..15).
	logoPad := 2 + (76-logoWidth+1)/2 // 2 + 20 = 22
	for i, l := range logoPlainLines() {
		atFrame(t, rows, 1+i, logoPad, l)
	}
}

// TestHomeViewClamps70x30 pins the narrow-terminal clamp: boxW clamps to the
// content width (66 < 75) so boxL = 2, and the logo centers in the content
// area (not full width).
func TestHomeViewClamps70x30(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 70, Height: 30}
	rows := fitsFrame(t, a.homeView(nil, "", "", "", "", ""), 70, 30)
	const contentW = 66 // 70 - 4
	// boxW = min(75, 66) = 66 (innerW = 66-1-4 = 61); boxL = 2 + (66-66+1)/2 = 2.
	const boxL = 2
	for i := 0; i < 4; i++ {
		atFrame(t, rows, 16+i, boxL, "┃")
	}
	atFrame(t, rows, 20, boxL, "╹")
	// the logo centered in the 66-wide content area: logoPad = 2 + (66-37+1)/2
	// = 17 (free = 3 → top = 2: pad 2..5, logo 6..13).
	logoPad := 2 + (contentW-logoWidth+1)/2
	for i, l := range logoPlainLines() {
		atFrame(t, rows, 6+i, logoPad, l)
	}
}

// TestHomeViewOverflow80x10 pins the overflow clamp: the fixed stack (24) +
// footer (3) = 27 rows exceeds the 10-row terminal, so the spacers clamp to 0
// and the frame drops the TOP 17 rows (the alt-screen anchor is the bottom —
// the footer + box bottom stay visible, the logo + box top are dropped).
func TestHomeViewOverflow80x10(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: 80, Height: 10}
	rows := fitsFrame(t, a.homeView(nil, "", "", "", "", ""), 80, 10)
	// the footer content is the last-but-one row (h-2 = 8).
	// the box bottom (╹) is near the top: the content rows 0..26, the visible
	// frame is rows 17..26 (10 rows); content row 18 (the ╹ bottom border) is
	// visible row 1.
	boxL := 2 + (76-75+1)/2 // 3
	atFrame(t, rows, 1, boxL, "╹")
	// the footer block occupies the last 3 rows (7,8,9); the content (row 8)
	// is blank here (no dir/version set on the testApp) — padded to w.
	if rows[8] != strings.Repeat(" ", 80) {
		t.Fatalf("footer content row = %q, want blank (no dir/version)", rows[8])
	}
}

// TestHomeFooterContentRow pins the footer content row (the 3-row footer
// block's middle row): the dir (muted) at col 2, the version (muted)
// right-aligned ending at col w-2; the dir is cut (no ellipsis) on a narrow
// terminal. Zero-theme (plain text).
func TestHomeFooterContentRow(t *testing.T) {
	t.Parallel()

	t.Run("dir only (no branch)", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.Service.Dir = "/home/u/proj"
		a.homeDirFunc = func() string { return "/home/u" }
		if got := stripANSI(a.homeFooterContentRow(80)); got != "  ~/proj" {
			t.Fatalf("footer = %q, want %q", got, "  ~/proj")
		}
	})

	t.Run("dir + branch", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.Service.Dir = "/home/u/proj"
		a.homeDirFunc = func() string { return "/home/u" }
		a.branch = "main"
		if got := stripANSI(a.homeFooterContentRow(80)); got != "  ~/proj:main" {
			t.Fatalf("footer = %q, want %q", got, "  ~/proj:main")
		}
	})

	t.Run("version present, no dir", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.version = "v0.8.0-4-gabcdef"
		// ver = "0.8.0" (5); verCol = 80-2-5 = 73 → 73 spaces + "0.8.0".
		if got := stripANSI(a.homeFooterContentRow(80)); got != strings.Repeat(" ", 73)+"0.8.0" {
			t.Fatalf("footer = %q, want the right-aligned version", got)
		}
	})

	t.Run("version absent", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.Service.Dir = "/home/u/proj"
		a.homeDirFunc = func() string { return "/home/u" }
		// no version → the dir only (no right segment).
		if got := stripANSI(a.homeFooterContentRow(80)); got != "  ~/proj" {
			t.Fatalf("footer = %q, want %q", got, "  ~/proj")
		}
	})

	t.Run("dir cut at 40 cols", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.size = tea.WindowSizeMsg{Width: 40, Height: 24}
		a.Service.Dir = "/tmp/this/is/a/long/destination/path"
		a.homeDirFunc = func() string { return "/home/u" }
		a.branch = "main"
		a.version = "v0.8.0-4-gabcdef"
		// ver = "0.8.0" (5); verCol = 40-2-5 = 33; the dir (43 cols) is cut
		// to verCol-5 = 28 (no ellipsis); a 3-col gap precedes the version.
		dir := "/tmp/this/is/a/long/destination/path:main"
		cut, _ := cutWidth(dir, 28)
		want := "  " + cut + strings.Repeat(" ", 33-(2+runeWidth(cut))) + "0.8.0"
		if got := stripANSI(a.homeFooterContentRow(40)); got != want {
			t.Fatalf("footer = %q, want %q", got, want)
		}
	})
}

// TestHomeViewSlashMenuAnchors pins the S3 home anchor (spec §6 S3): with the
// slash menu open (input "/"), the dropdown sits above the box top edge at the
// box's left edge / width, overlaying the logo rows while open. The box + hint
// stay intact below (the 0.8.0 placement put the slash menu on the left-overlay
// rows below the tips).
func TestHomeViewSlashMenuAnchors(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: mockW, Height: mockH}
	a.store.Commands = testCommands()
	a.prompt.input.SetValue("/")
	a.prompt.sel = 0
	items := a.menuItems()
	if items == nil {
		t.Fatal("slash menu should be open for input \"/\"")
	}
	n := len(items)
	if n > maxDropdownRows {
		n = maxDropdownRows
	}
	rows := fitsFrame(t, a.homeView(items, "", "", "", "", ""), mockW, mockH)
	// the dropdown's bottom row is right above the box top edge (mockBoxTop-1);
	// the top row (n rows above) is the selected row (sel=0).
	top := mockBoxTop - n
	bottom := mockBoxTop - 1
	atFrame(t, rows, top, mockBoxL, "┃")
	atFrame(t, rows, bottom, mockBoxL, "┃")
	// the logo is overlaid while open: the mock logo row (mockLogoTop+1) now
	// carries the dropdown border at the box's left edge (not the logo art at
	// mockLogoL).
	atFrame(t, rows, mockLogoTop+1, mockBoxL, "┃")
	// the selected row (sel=0) is the first merged command at the box's left
	// edge (the dropdown's top row).
	if !strings.Contains(stripANSI(rows[top]), "/sessions") {
		t.Fatalf("selected dropdown row = %q, want the first merged command (/sessions)", rows[top])
	}
	// the box + hint stay intact below the dropdown; the hint's first segment
	// is context-aware (S6, spec §3.2): "tab complete" while the menu is open
	// (re-baseline of the S3 pin's "tab agents" — the behavior change is
	// S6's).
	atFrame(t, rows, mockBoxTop, mockBoxL, "┃")
	atFrame(t, rows, mockHintRow, mockBoxL, "tab complete")
}
