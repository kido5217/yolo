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

// selectAction is a footer action (upstream DialogSelectAction): its key
// triggers it, tab/shift+tab focus it, enter on the focus runs it.
type selectAction struct {
	key   key.Binding
	title string
	run   func(*App)
}

// footerHint is a right-footer hint (upstream: key + desc).
type footerHint struct {
	key  string
	desc string
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
	lastSel     int // the selection row the window was last anchored at; 0 is the initial anchor (the window starts at the top, the selection's first row) so the first view re-anchors only when the selection actually moves (S2.7)
	pageDelta   int // queued window shift from pgup/pgdn, consumed by view (S2.7)
	filter      string
	input       textinput.Model
	actions     []selectAction
	hints       []footerHint
	focAct      int // focused action index, -1 = none (S2.7)
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
		focAct:      -1,
	}
	m.input = textinput.New()
	m.input.Prompt = ""
	m.input.Placeholder = placeholder
	m.input.SetWidth(40)
	_ = m.input.Focus()
	return m
}

// WithActions attaches the left-footer actions (the focused one highlights).
func (m *selectModel) WithActions(actions []selectAction) *selectModel {
	m.actions = actions
	m.focAct = -1
	return m
}

// WithHints attaches the right-footer hints.
func (m *selectModel) WithHints(hints []footerHint) *selectModel {
	m.hints = hints
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

// handleKey drives the select while the modal stack owns the frame (S2.7):
// an action's own key runs it; pgup/pgdn page-scroll the window;
// tab/shift+tab cycle the action focus; arrows move with wraparound,
// home/end jump, enter runs the focused action (or submits the selection);
// every other key feeds the fuzzy filter input (esc/ctrl+c are consumed
// by the stack first — S2.2).
func (m *selectModel) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	for i := range m.actions {
		if key.Matches(k, m.actions[i].key) {
			m.actions[i].run(a)
			return nil
		}
	}
	switch {
	case key.Matches(k, selPgUpKey):
		m.pageScroll(-10)
	case key.Matches(k, selPgDnKey):
		m.pageScroll(10)
	case key.Matches(k, selTabKey):
		m.focusAction(+1)
	case key.Matches(k, selShiftTabKey):
		m.focusAction(-1)
	case key.Matches(k, homeKeyMap.Up):
		m.move(-1)
	case key.Matches(k, homeKeyMap.Down):
		m.move(1)
	case key.Matches(k, selHomeKey):
		m.jump(0)
	case key.Matches(k, selEndKey):
		m.jump(-1)
	case key.Matches(k, homeKeyMap.Enter):
		if m.focAct >= 0 {
			m.actions[m.focAct].run(a)
			return nil
		}
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
	selHomeKey     = key.NewBinding(key.WithKeys("home"))
	selEndKey      = key.NewBinding(key.WithKeys("end"))
	selTabKey      = key.NewBinding(key.WithKeys("tab"))
	selShiftTabKey = key.NewBinding(key.WithKeys("shift+tab"))
	selPgUpKey     = key.NewBinding(key.WithKeys("pgup"))
	selPgDnKey     = key.NewBinding(key.WithKeys("pgdown"))
)

// pageScroll queues a window shift (deviation 176: ±10 rows pinned; the
// upstream env-machined getScrollAcceleration is not ported). The WINDOW
// moves (the selection stays); view() consumes the delta and re-anchors
// the window to the selection only when the selection itself changed
// (selectModel.lastSel — added in this task; S2.6's every-call re-anchor
// becomes the selection-change re-anchor):
//
//	if selRow >= 0 && selRow != m.lastSel { m.lastSel = selRow; <S2.6 anchor> }
//	else if m.pageDelta != 0 { m.top += m.pageDelta; m.pageDelta = 0; if m.top < 0 { m.top = 0 } }
//	// then the existing clamp (top > len(visible) → max(0, len-visible))
func (m *selectModel) pageScroll(rows int) {
	m.pageDelta += rows
}

// focusAction cycles the action focus (tab/shift+tab; no actions = no-op).
func (m *selectModel) focusAction(d int) {
	n := len(m.actions)
	if n == 0 {
		return
	}
	m.focAct = ((m.focAct+d)%n + n) % n
}

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
// upstream height = min(rows, floor(h/2)-6)) and the footer row (the S2.7
// actions/hints, the S2.5 keymap hint when neither exists).
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
	// the selection's FIRST row anchors the window only when the selection
	// row changed (S2.7: S2.6's every-call re-anchor becomes the
	// selection-change re-anchor); otherwise a queued page delta shifts the
	// window (the window moves, the selection stays)
	selRow := -1
	for i, l := range lines {
		if l.opt == m.sel {
			selRow = i
			break
		}
	}
	if selRow >= 0 && selRow != m.lastSel {
		m.lastSel = selRow
		if selRow < m.top {
			m.top = selRow
		}
		if selRow >= m.top+visible {
			m.top = selRow - visible + 1
		}
	} else if m.pageDelta != 0 {
		m.top += m.pageDelta
		m.pageDelta = 0
		if m.top < 0 {
			m.top = 0
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
	// footer: actions left (focused highlighted), hints right (muted),
	// the S2.5 keymap hint when neither exists
	if len(m.actions) > 0 || len(m.hints) > 0 {
		var leftParts, leftPlain []string
		for i, ac := range m.actions {
			label := ac.key.Keys()[0] + " " + ac.title
			leftPlain = append(leftPlain, label)
			if i == m.focAct {
				leftParts = append(leftParts, cursorStyle(th).Render(label))
			} else {
				leftParts = append(leftParts, th.TextMuted().Render(label))
			}
		}
		rightPlain := strings.Join(hintTexts(m.hints), " \u00B7 ")
		line := strings.Join(leftParts, "   ")
		if rightPlain != "" {
			gap := w - 2 - plainJoinWidth(leftPlain, "   ") - runeWidth(rightPlain)
			if gap < 1 {
				gap = 1
			}
			line += strings.Repeat(" ", gap) + th.TextMuted().Render(rightPlain)
		}
		b.WriteString(line)
	} else {
		b.WriteString(th.TextMuted().Render("  \u2191/\u2193 move \u00B7 enter select \u00B7 esc close"))
	}
	return b.String()
}

// hintTexts is the right-tail text of the hints (key + desc pairs).
func hintTexts(hints []footerHint) []string {
	out := make([]string, len(hints))
	for i, h := range hints {
		out[i] = h.key + " " + h.desc
	}
	return out
}

// plainJoinWidth is the rendered width of the joined parts (rune columns).
func plainJoinWidth(parts []string, sep string) int {
	w := 0
	for i, p := range parts {
		if i > 0 {
			w += runeWidth(sep)
		}
		w += runeWidth(p)
	}
	return w
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
