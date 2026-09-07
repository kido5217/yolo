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
	rows   []dropdownRow
	sel    int
	vis    int
	innerW int // the fill columns between the border columns
	avail  int // the content columns inside the padding columns

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
// token, the description in textMuted); the selection row is the full-row
// paint (primary bg + SelectedForeground fg across every column, the
// borders and padding inside) or, without a primary token, the chrome with
// the whole content in the degraded selection style (select.go's
// missing-primary idiom).
func (d dropdown) row(i int) string {
	r := d.rows[i]
	label, desc := d.fit(r)
	if i == d.sel && d.selOK {
		return d.selSty.Render("|") + d.selSty.Width(d.innerW).Render(" "+label+desc) + d.selSty.Render("|")
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
	return d.border.Render("|") + d.bgMenu.Width(d.innerW).Render(content) + d.border.Render("|")
}

// fit truncates the row's columns to the content area (avail): an over-wide
// label eats the description and is cut at avail; otherwise the description
// (with its "  " right offset) is cut at the remaining width.
func (d dropdown) fit(r dropdownRow) (label, desc string) {
	lw := runeWidth(r.label)
	if lw > d.avail {
		label, _ = cutWidth(r.label, d.avail)
		return label, ""
	}
	if r.description == "" {
		return r.label, ""
	}
	desc, _ = cutWidth("  "+r.description, d.avail-lw)
	return r.label, desc
}
