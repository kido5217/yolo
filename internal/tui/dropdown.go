// dropdown.go — the shared bordered-dropdown primitive (spec §6 S1, design
// §3.1): the slash menu (Task 2) and the later @-picker/palette surfaces
// render through this. The box is anchored above the input, rows
// bottom-aligned to the input's top edge: the split left/right theme.border
// border columns (no top/bottom edges), the backgroundMenu inner fill, one
// padding column after each border; the selection row paints primary bg +
// SelectedForeground fg across the full row width (select.go's active-row
// idiom). No scroll — every rendered row is one of the ranked top N
// (spec §3.1).

package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// maxDropdownRows caps the box height (spec §3.1's top N).
const maxDropdownRows = 10

// borderChar is the double-line box-drawing vertical (opencode's SplitBorder
// vertical, ui/border.ts): the dropdown's left/right border renders this
// instead of the ASCII '|'.
const borderChar = "\u2503"

// dropdownRow is one dropdown row: the label (left column) + an optional
// description (right column, rendered muted).
type dropdownRow struct {
	label       string
	description string
}

// dropdown is the clamped box geometry: the visible rows (the min of the
// row count, maxDropdownRows, and the space above) and the derived layout
// widths, with the row styles built from the theme.
type dropdown struct {
	rows      []dropdownRow
	sel       int
	vis       int
	innerW    int // the fill columns between the border columns
	avail     int // the content columns inside the padding columns
	maxLabelW int // the widest label over all rows (the fixed description column)

	border   lipgloss.Style
	bgMenu   lipgloss.Style
	labelSty lipgloss.Style
	descSty  lipgloss.Style
	selSty   lipgloss.Style
	selOK    bool
}

// newDropdown clamps the visible window and builds the row styles (the
// border columns, the backgroundMenu fill, the text/textMuted columns, and
// the selection-row paint).
func newDropdown(rows []dropdownRow, sel, width, spaceAbove int, th theme.Theme) dropdown {
	d := dropdown{
		rows:   rows,
		sel:    sel,
		vis:    min(len(rows), maxDropdownRows, spaceAbove),
		innerW: width - 2,
		avail:  width - 4,
		border: th.Border(),
		bgMenu: th.BackgroundMenu(),
	}
	for _, r := range rows {
		if w := runeWidth(r.label); w > d.maxLabelW {
			d.maxLabelW = w
		}
	}
	if d.vis < 0 {
		d.vis = 0
	}
	if d.innerW < 0 {
		d.innerW = 0
	}
	if d.avail < 0 {
		d.avail = 0
	}
	d.labelSty = th.Text().Inherit(d.bgMenu)
	d.descSty = th.TextMuted().Inherit(d.bgMenu)
	if p, ok := th.Color("primary"); ok {
		s := th.SelectedForeground()
		d.selSty = lipgloss.NewStyle().
			Foreground(lipgloss.Color(s.Hex()[:7])).
			Background(lipgloss.Color(p.Hex()[:7]))
		d.selOK = true
	} else {
		// No primary token (a custom theme may omit it): the selection
		// degrades to the cursor style (select.go's missing-primary idiom)
		// over the box fill — bold in the text fg, never invisible.
		d.selSty = cursorStyle(th).Inherit(d.bgMenu)
	}
	return d
}

// height is the rendered box height in rows (0 when the clamp leaves none).
func (d dropdown) height() int { return d.vis }

// rowAt maps a cell row (0 = the box's first row) to the row index;
// outside the box (above the first or below the last row) there is no row.
func (d dropdown) rowAt(cellRow int) (int, bool) {
	if cellRow < 0 || cellRow >= d.vis {
		return 0, false
	}
	return cellRow, true
}

// view renders the bordered box ("" when the clamp leaves no rows).
func (d dropdown) view() string {
	if d.vis == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < d.vis; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(d.row(i))
	}
	return b.String()
}

// row renders one box row (the two-column layout mirroring menuView: the
// name left, the description right-offset by "  "). The non-selection row
// is the border columns + the backgroundMenu fill (the label in the text
// token, the description in textMuted); the selection row paints its content
// in the full-row highlight (primary bg + SelectedForeground fg) while the
// border columns stay in the border token, or, without a primary token, the
// whole content degrades to the bold text-fg selection style (select.go's
// missing-primary idiom).
func (d dropdown) row(i int) string {
	r := d.rows[i]
	label, desc := d.fit(r)
	if i == d.sel && d.selOK {
		// The highlight covers the content only; the border columns stay in
		// the border token (opencode keeps the active highlight off the border).
		return d.border.Render(borderChar) + d.selSty.Width(d.innerW).Render(" "+label+desc) + d.border.Render(borderChar)
	}
	labelSty, descSty := d.labelSty, d.descSty
	if i == d.sel {
		// The degraded selection (no primary token): the whole content
		// (label + description) in the bold text-fg style, not muted.
		labelSty, descSty = d.selSty, d.selSty
	}
	content := " " + labelSty.Render(label)
	if desc != "" {
		content += descSty.Render(desc)
	}
	return d.border.Render(borderChar) + d.bgMenu.Width(d.innerW).Render(content) + d.border.Render(borderChar)
}

// fit truncates the row's columns to the content area (avail) on a FIXED
// description column: an over-wide label is cut at avail (no description);
// otherwise the label is padded to maxLabelW (the widest label over all rows,
// the opencode padEnd(max+2) idiom) and the description (with its "  " right
// offset) is cut to the shared width avail-maxLabelW, so every description
// starts at the same column. When maxLabelW alone exceeds avail the
// description column is pushed off the edge and dropped.
func (d dropdown) fit(r dropdownRow) (label, desc string) {
	lw := runeWidth(r.label)
	if lw > d.avail {
		label, _ = cutWidth(r.label, d.avail)
		return label, ""
	}
	if r.description == "" {
		return r.label, ""
	}
	if d.maxLabelW > d.avail {
		label, _ = cutWidth(r.label, d.avail)
		return label, ""
	}
	label = r.label
	if d.maxLabelW > lw {
		label = r.label + strings.Repeat(" ", d.maxLabelW-lw)
	}
	desc, _ = cutWidth("  "+r.description, d.avail-d.maxLabelW)
	return label, desc
}
