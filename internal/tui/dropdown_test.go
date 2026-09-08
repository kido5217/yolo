// dropdown_test.go — the S1 render pins for the shared bordered-dropdown
// primitive (spec §6 S1): the box shape (the split left/right border, the
// backgroundMenu inner fill, the padding after the left border), the
// selection-row SGR (primary bg + SelectedForeground fg across the full row
// width), the height clamp, and the row hit-test. Whitebox, yolo dark tokens.

package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// yoloDarkTheme resolves the yolo dark theme straight from the embedded
// assets (the S0.2 goldens) — the SGR pins below derive from its tokens.
func yoloDarkTheme(t *testing.T) theme.Theme {
	t.Helper()
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["yolo"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	return theme.Theme{R: r, Name: "yolo", Mode: "dark"}
}

func TestDropdownBoxShape(t *testing.T) {
	th := yoloDarkTheme(t)
	rows := []dropdownRow{
		{label: "/quit", description: "quit the app"},
		{label: "/help"},
	}
	d := newDropdown(rows, 1, 40, 10, th)
	lines := strings.Split(d.view(), "\n")
	if len(lines) != 2 {
		t.Fatalf("box = %d rows, want 2:\n%s", len(lines), d.view())
	}
	for i, l := range lines {
		plain := stripANSI(l)
		cols := []rune(plain)
		if n := len(cols); n != 40 {
			t.Fatalf("row %d = %d cols, want 40: %q", i, n, plain)
		}
		if cols[0] != '┃' || cols[len(cols)-1] != '┃' {
			t.Fatalf("row %d = %q, want the split left/right border (no top/bottom edges)", i, plain)
		}
		if cols[1] != ' ' {
			t.Fatalf("row %d = %q, want 1 padding after the left border", i, plain)
		}
	}
	// The two-column layout (menuView's mirror): name left, description
	// right-offset by "  ".
	if !strings.Contains(stripANSI(lines[0]), "/quit  quit the app") {
		t.Fatalf("row 0 lost the two-column layout: %q", stripANSI(lines[0]))
	}
	if !strings.Contains(stripANSI(lines[1]), "/help") {
		t.Fatalf("row 1 lost the label: %q", stripANSI(lines[1]))
	}
	// SGR pins (yolo dark tokens): the border fg #484848, the backgroundMenu
	// fill #1e1e1e (the backgroundElement fallback), the label #eeeeee, the
	// description #808080.
	if !strings.Contains(lines[0], "38;2;72;72;72") {
		t.Fatalf("border fg SGR missing (want #484848): %s", lines[0])
	}
	if !strings.Contains(lines[0], "48;2;30;30;30") {
		t.Fatalf("backgroundMenu fill SGR missing (want #1e1e1e): %s", lines[0])
	}
	if !strings.Contains(lines[0], "38;2;238;238;238") {
		t.Fatalf("label fg SGR missing (want #eeeeee): %s", lines[0])
	}
	if !strings.Contains(lines[0], "38;2;128;128;128") {
		t.Fatalf("description fg SGR missing (want #808080): %s", lines[0])
	}
}

func TestDropdownSelectionRowSGR(t *testing.T) {
	th := yoloDarkTheme(t)
	rows := []dropdownRow{
		{label: "/quit", description: "quit the app"},
		{label: "/help", description: "show the help"},
		{label: "/model"},
	}
	d := newDropdown(rows, 1, 40, 10, th)
	lines := strings.Split(d.view(), "\n")
	sel := lines[1]
	// yolo dark: primary #fab283 (the row bg), SelectedForeground = the
	// background token #0a0a0a (the row fg) — the full row (borders +
	// padding + content) paints across all 40 columns.
	if !strings.Contains(sel, "48;2;250;178;131") {
		t.Fatalf("selection row missing the primary bg SGR (#fab283): %s", sel)
	}
	if !strings.Contains(sel, "38;2;10;10;10") {
		t.Fatalf("selection row missing the SelectedForeground SGR (#0a0a0a): %s", sel)
	}
	// The full-row-width pin: every SGR open inside the row carries the
	// primary bg (the padding run included — no unpainted column).
	for _, seg := range strings.Split(sel, "\x1b[") {
		switch {
		case seg == "", seg == "m", seg == "0m":
			continue
		}
		if !strings.Contains(seg, "48;2;250;178;131") {
			t.Fatalf("selection row has an unpainted run %q (row: %s)", seg, sel)
		}
	}
	if n := len([]rune(stripANSI(sel))); n != 40 {
		t.Fatalf("selection row = %d cols, want 40: %q", n, stripANSI(sel))
	}
	// The unselected rows keep the plain chrome (no primary paint).
	if strings.Contains(lines[0], "48;2;250;178;131") {
		t.Fatalf("unselected row carries the primary bg: %s", lines[0])
	}
}

// TestDropdownSelectionRowNoPrimary pins the missing-primary degradation:
// a theme without a primary token (a reachable state — a custom theme JSON
// is an arbitrary token map and ResolveTheme does not require one) degrades
// the selection row to the cursor style (select.go's missing-primary idiom):
// the whole content (label + description) bold in the text fg, the box
// chrome intact (the description NOT muted).
func TestDropdownSelectionRowNoPrimary(t *testing.T) {
	// The yolo dark tokens minus primary (the SGR pins below reuse the S1
	// hexes).
	th := theme.Theme{R: theme.Resolved{Colors: map[string]theme.RGBA{
		"background":     theme.FromHex("#0a0a0a"),
		"backgroundMenu": theme.FromHex("#1e1e1e"),
		"text":           theme.FromHex("#eeeeee"),
		"textMuted":      theme.FromHex("#808080"),
		"border":         theme.FromHex("#484848"),
	}}}
	rows := []dropdownRow{
		{label: "/quit", description: "quit the app"},
		{label: "/help", description: "show the help"},
	}
	d := newDropdown(rows, 1, 40, 10, th)
	if d.selOK {
		t.Fatal("no primary token: selOK = true, want the degraded path")
	}
	lines := strings.Split(d.view(), "\n")
	sel := lines[1]
	// The degraded selection: the whole content bold in the text fg
	// #eeeeee, carrying the backgroundMenu fill #1e1e1e under it (the
	// chrome matches the non-selection rows).
	if !strings.Contains(sel, "1;38;2;238;238;238") {
		t.Fatalf("selection row missing the bold text-fg SGR: %s", sel)
	}
	// The description is NOT muted on the selection row (the pre-rewire
	// menuView behavior).
	if strings.Contains(sel, "38;2;128;128;128") {
		t.Fatalf("selection row mutes the description: %s", sel)
	}
	// No primary-background SGR anywhere in the selection row.
	if strings.Contains(sel, "48;2;250;178;131") {
		t.Fatalf("selection row carries the primary bg SGR: %s", sel)
	}
	// The box chrome (the S1 pins): the split border, the border fg
	// #484848, the backgroundMenu fill #1e1e1e — every SGR open inside the
	// row carries the fill, the border columns excepted (unpainted, exactly
	// like the non-selection rows).
	plain := stripANSI(sel)
	cols := []rune(plain)
	if n := len(cols); n != 40 {
		t.Fatalf("selection row = %d cols, want 40: %q", n, plain)
	}
	if cols[0] != '┃' || cols[len(cols)-1] != '┃' {
		t.Fatalf("selection row lost the split border: %q", plain)
	}
	if !strings.Contains(sel, "38;2;72;72;72") {
		t.Fatalf("selection row missing the border fg SGR: %s", sel)
	}
	if !strings.Contains(sel, "48;2;30;30;30") {
		t.Fatalf("selection row missing the backgroundMenu fill SGR: %s", sel)
	}
	for _, seg := range strings.Split(sel, "\x1b[") {
		switch {
		case seg == "", seg == "m", seg == "0m":
			continue
		}
		if strings.HasPrefix(seg, "38;2;72;72;72") {
			continue // the border column (no fill, like the non-selection rows)
		}
		if !strings.Contains(seg, "48;2;30;30;30") {
			t.Fatalf("selection row has an unfilled SGR open %q (row: %s)", seg, sel)
		}
	}
	// The content survives intact: label + the "  "-offset description.
	if !strings.Contains(plain, "/help  show the help") {
		t.Fatalf("selection row lost the two-column layout: %q", plain)
	}
}

func TestDropdownHeightClamp(t *testing.T) {
	th := yoloDarkTheme(t)
	rows := make([]dropdownRow, 12)
	for i := range rows {
		rows[i] = dropdownRow{label: fmt.Sprintf("item%d", i)}
	}
	// 12 items → the 10-row cap.
	d := newDropdown(rows, 0, 40, 24, th)
	if got := d.height(); got != 10 {
		t.Fatalf("12 items = %d rows, want 10 (the cap)", got)
	}
	if lines := strings.Split(d.view(), "\n"); len(lines) != 10 {
		t.Fatalf("12 items rendered %d rows, want 10", len(lines))
	}
	// 10 items with only 4 rows of space above → 4 rows.
	rows10 := rows[:10]
	d2 := newDropdown(rows10, 0, 40, 4, th)
	if got := d2.height(); got != 4 {
		t.Fatalf("4 rows of space above = %d rows, want 4", got)
	}
	if lines := strings.Split(d2.view(), "\n"); len(lines) != 4 {
		t.Fatalf("4 rows of space above rendered %d rows, want 4", len(lines))
	}
	// No space above → no box.
	if got := newDropdown(rows10, 0, 40, 0, th).view(); got != "" {
		t.Fatalf("no space above rendered %q, want \"\"", got)
	}
}

func TestDropdownRowHitTest(t *testing.T) {
	th := yoloDarkTheme(t)
	rows := make([]dropdownRow, 5)
	for i := range rows {
		rows[i] = dropdownRow{label: "i" + string(rune('a'+i))}
	}
	d := newDropdown(rows, 0, 40, 24, th)
	// A cell row within the box → the row index (the box's first row = 0).
	for i := 0; i < 5; i++ {
		if got, ok := d.rowAt(i); !ok || got != i {
			t.Fatalf("rowAt(%d) = %d, %v; want %d, true", i, got, ok, i)
		}
	}
	// A cell row outside the box → no row.
	for _, r := range []int{-1, 5, 99} {
		if got, ok := d.rowAt(r); ok || got != 0 {
			t.Fatalf("rowAt(%d) = %d, %v; want no row", r, got, ok)
		}
	}
	// The clamped box only hit-tests its visible rows.
	d2 := newDropdown(rows, 0, 40, 2, th)
	if got, ok := d2.rowAt(1); !ok || got != 1 {
		t.Fatalf("clamped rowAt(1) = %d, %v; want 1, true", got, ok)
	}
	if _, ok := d2.rowAt(2); ok {
		t.Fatal("clamped rowAt(2) matched a row outside the 2-row box")
	}
}
