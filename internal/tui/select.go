// select.go — the port of upstream DialogSelect (dialog-select.tsx), the
// shared list primitive behind the modal dialogs (S2.9 model, S2.10 agent,
// the S3 dialogs). S2.5 lands the option list + navigation + the fuzzy
// filter; S2.6 adds categories, details and the footer tail; S2.7 adds
// actions, the footer hints and the scroll acceleration.

package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sahilm/fuzzy"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// selectOption is one selectable row (port of DialogSelectOption; the JSX
// rendering fields have no Go analog — the row render lives in selectModel).
type selectOption struct {
	title       string
	description string
	details     []string
	footer      string
	category    string
	value       any
	disabled    bool // excluded from the filtered list entirely (upstream)
}

// selectModel is the DialogSelect state machine.
type selectModel struct {
	title       string
	placeholder string
	options     []selectOption
	isCurrent   func(selectOption) bool
	onSelect    func(*App, selectOption)
	onMove      func(selectOption)
	sel         int
	top         int // first visible row (S2.6 counts rendered rows)
	filter      string
	input       textinput.Model
}

// selectNew builds a select (isCurrent/onMove/onSelect may be nil).
func selectNew(title, placeholder string, options []selectOption,
	isCurrent func(selectOption) bool, onSelect func(*App, selectOption),
	onMove func(selectOption)) *selectModel {
	m := &selectModel{
		title:       title,
		placeholder: placeholder,
		options:     options,
		isCurrent:   isCurrent,
		onSelect:    onSelect,
		onMove:      onMove,
	}
	m.input = textinput.New()
	m.input.Prompt = ""
	m.input.Placeholder = placeholder
	m.input.SetWidth(40)
	_ = m.input.Focus()
	return m
}

// filtered is the live list (upstream `filtered` memo): disabled options are
// excluded entirely; an empty needle returns the rest in order, otherwise
// the fuzzy hits sorted by the weighted score (title ×2, category ×1 — the
// port of the fuzzysort keys/scoreFn, dialog-select.tsx:154-173).
func (m *selectModel) filtered() []selectOption {
	enabled := make([]selectOption, 0, len(m.options))
	for _, o := range m.options {
		if !o.disabled {
			enabled = append(enabled, o)
		}
	}
	if m.filter == "" {
		return enabled
	}
	n := len(enabled)
	titles := make([]string, n)
	cats := make([]string, n)
	for i, o := range enabled {
		titles[i] = o.title
		cats[i] = o.category
	}
	score := make([]int, n)
	for _, hit := range fuzzy.Find(m.filter, titles) {
		score[hit.Index] += 2 * hit.Score
	}
	for _, hit := range fuzzy.Find(m.filter, cats) {
		score[hit.Index] += hit.Score
	}
	type scored struct {
		opt selectOption
		s   int
	}
	var hits []scored
	for i, o := range enabled {
		if score[i] > 0 {
			hits = append(hits, scored{o, score[i]})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].s > hits[j].s })
	out := make([]selectOption, len(hits))
	for i, h := range hits {
		out[i] = h.opt
	}
	return out
}

// syncFilter reads the filter input and resets the selection when the needle
// becomes non-empty (upstream: filter>0 → moveTo(0)).
func (m *selectModel) syncFilter() {
	if f := m.input.Value(); f != m.filter {
		m.filter = f
		if f != "" {
			m.sel = 0
		}
	}
}

// handleKey drives the select while the modal stack owns the frame: arrows
// move with wraparound, home/end jump, enter submits the selection; every
// other key feeds the fuzzy filter input (esc/ctrl+c are consumed by the
// stack first — S2.2).
func (m *selectModel) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	switch {
	case key.Matches(k, homeKeyMap.Up):
		m.move(-1)
	case key.Matches(k, homeKeyMap.Down):
		m.move(1)
	case key.Matches(k, selHomeKey):
		m.jump(0)
	case key.Matches(k, selEndKey):
		m.jump(-1)
	case key.Matches(k, homeKeyMap.Enter):
		m.submit(a)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		m.syncFilter()
		if cmd != nil {
			return []tea.Cmd{cmd}
		}
		return nil
	}
	return nil
}

var (
	selHomeKey = key.NewBinding(key.WithKeys("home"))
	selEndKey  = key.NewBinding(key.WithKeys("end"))
)

// move steps the selection with wraparound (upstream move, 290-297).
func (m *selectModel) move(d int) {
	l := m.filtered()
	if len(l) == 0 {
		return
	}
	m.sel = ((m.sel+d)%len(l) + len(l)) % len(l)
	if m.onMove != nil {
		m.onMove(l[m.sel])
	}
}

// jump lands the selection at the start (0) or end (-1) of the list.
func (m *selectModel) jump(i int) {
	l := m.filtered()
	if len(l) == 0 {
		return
	}
	if i < 0 {
		i = len(l) - 1
	}
	m.sel = i
	if m.onMove != nil {
		m.onMove(l[m.sel])
	}
}

// submit fires onSelect on the selected option (upstream select).
func (m *selectModel) submit(a *App) {
	l := m.filtered()
	if len(l) == 0 || m.onSelect == nil {
		return
	}
	m.onSelect(a, l[m.sel])
}

// view renders the select's inner lines (the modal stack draws the panel
// chrome — S2.2): the title row, the filter input row, the visible row window
// (the scroll window counts the BUILT rows — headers + options + details —
// upstream height = min(rows, floor(h/2)-6)) and the keymap hint row.
func (m *selectModel) view(w, h int, th theme.Theme) string {
	m.syncFilter()
	lines := m.buildLines(w, th)
	visible := h/2 - 6
	if visible < 1 {
		visible = 1
	}
	if visible > len(lines) {
		visible = len(lines)
	}
	if len(lines) == 0 {
		var b strings.Builder
		b.WriteString(title.Render(m.title) + "\n  " + m.input.View() + "\n")
		b.WriteString(th.TextMuted().Render("  No results found"))
		return b.String()
	}
	// the selection's FIRST row anchors the window (S2.5: the option row)
	selRow := -1
	for i, l := range lines {
		if l.opt == m.sel {
			selRow = i
			break
		}
	}
	if selRow >= 0 {
		if selRow < m.top {
			m.top = selRow
		}
		if selRow >= m.top+visible {
			m.top = selRow - visible + 1
		}
	}
	if m.top > len(lines)-visible {
		m.top = max(0, len(lines)-visible)
	}
	m.input.SetWidth(max(1, w-4))
	var b strings.Builder
	b.WriteString(title.Render(m.title))
	b.WriteByte('\n')
	b.WriteString("  " + m.input.View())
	b.WriteByte('\n')
	for i := m.top; i < min(m.top+visible, len(lines)); i++ {
		if i > m.top {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i].text)
	}
	b.WriteByte('\n')
	b.WriteString(th.TextMuted().Render("  \u2191/\u2193 move \u00B7 enter select \u00B7 esc close"))
	return b.String()
}

// selLine is one rendered line of the select list (the scroll window slices
// these): opt is the option index (-1 for a header/blank row).
type selLine struct {
	opt  int
	text string
}

// buildLines renders the full list (S2.6): the category header rows (accent
// bold, indent 3, a blank row between groups — hidden while filtering, the
// upstream `flat` behavior), each option row (S2.5's rowLine) and its
// detail rows (muted, indent 7, truncateMiddle'd), the per-option footer
// tail right-aligned to the row width.
func (m *selectModel) buildLines(w int, th theme.Theme) []selLine {
	l := m.filtered()
	flat := m.filter != ""
	var lines []selLine
	lastCat := ""
	for i, o := range l {
		if !flat && o.category != "" && o.category != lastCat {
			if lastCat != "" {
				lines = append(lines, selLine{opt: -1, text: ""})
			}
			lines = append(lines, selLine{opt: -1, text: th.Accent().Render("   " + o.category)})
			lastCat = o.category
		}
		active := i == m.sel
		cur := m.isCurrent != nil && m.isCurrent(o)
		var row string
		if o.footer != "" {
			row = m.rowWithFooter(o, active, cur, w, th)
		} else {
			row = m.rowLine(o, active, cur, th, w)
		}
		lines = append(lines, selLine{opt: i, text: row})
		for _, d := range o.details {
			lines = append(lines, selLine{opt: i, text: th.TextMuted().Render("       " + truncateMiddle(d, max(1, w-7)))})
		}
	}
	return lines
}

// rowLine renders one option row with the S0.9 home SELECT token chain
// (dialog-select's active row 667-678 + Option 732-791): the active row is
// the full-row paint in the selection background (theme primary) with the
// SelectedForeground text and the bold title; the current option carries the
// "●" gutter in primary (non-active rows) or the selection fg; other rows:
// the title in the text token, the description tail in textMuted. A zero
// Theme degrades to plain rows with the cursorStyle-bold active title.
func (m *selectModel) rowLine(o selectOption, active, cur bool, th theme.Theme, w int) string {
	gutter := "  "
	desc := ""
	if o.description != "" {
		desc = "  " + o.description
	}
	if active {
		bg, ok := th.Color("primary")
		if !ok {
			return cursorStyle(th).Render(gutter+o.title) + desc
		}
		sel := th.SelectedForeground()
		fg := lipgloss.Color(sel.Hex()[:7])
		bgC := lipgloss.Color(bg.Hex()[:7])
		head := lipgloss.NewStyle().Foreground(fg).Background(bgC).Bold(true)
		tail := lipgloss.NewStyle().Foreground(fg).Background(bgC)
		if cur {
			gutter = "● "
		}
		return lipgloss.NewStyle().Background(bgC).Width(w).Render(head.Render(gutter+o.title) + tail.Render(desc))
	}
	gutterSty := th.TextMuted()
	if cur {
		gutter = "● "
		gutterSty = th.Primary()
	}
	line := gutterSty.Render(gutter) + th.Text().Render(o.title)
	if desc != "" {
		line += th.TextMuted().Render(desc)
	}
	return line
}

// rowWithFooter renders an option row with the per-option footer tail at the
// right edge of the row width (the port of upstream Option's flex layout,
// dialog-select.tsx:732-791): the plain content (gutter + title + description)
// is built first, the tail gap is computed from the plain rune widths, and the
// line renders in one pass — active: the S2.5 full-row paint with the footer
// inside it (the footer in the row's selection fg); unselected: the S2.5
// segment styles with the footer muted.
func (m *selectModel) rowWithFooter(o selectOption, active, cur bool, w int, th theme.Theme) string {
	gutter := "  "
	if cur {
		gutter = "● "
	}
	desc := ""
	if o.description != "" {
		desc = "  " + o.description
	}
	gap := w - runeWidth(gutter+o.title+desc) - runeWidth(o.footer)
	if gap < 0 {
		gap = 0
	}
	pad := strings.Repeat(" ", gap)
	if active {
		bg, ok := th.Color("primary")
		if !ok {
			return cursorStyle(th).Render(gutter+o.title) + desc + pad + o.footer
		}
		sel := th.SelectedForeground()
		fg := lipgloss.Color(sel.Hex()[:7])
		bgC := lipgloss.Color(bg.Hex()[:7])
		head := lipgloss.NewStyle().Foreground(fg).Background(bgC).Bold(true)
		tail := lipgloss.NewStyle().Foreground(fg).Background(bgC)
		return lipgloss.NewStyle().Background(bgC).Width(w).Render(head.Render(gutter+o.title) + tail.Render(desc+pad+o.footer))
	}
	gutterSty := th.TextMuted()
	if cur {
		gutterSty = th.Primary()
	}
	line := gutterSty.Render(gutter) + th.Text().Render(o.title)
	if desc != "" {
		line += th.TextMuted().Render(desc)
	}
	line += th.TextMuted().Render(pad + o.footer)
	return line
}
