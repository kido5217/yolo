package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// homeModel holds home-route state: the cursor line and the relative-time
// clock. (The "/<cmd>" buffer was the T23 prototype; T25 replaces it with the
// always-focused prompt input.)
type homeModel struct {
	cursor int
	now    func() int64
}

var homeKeyMap = struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	NewSess key.Binding
	Quit    key.Binding
}{
	Up:      key.NewBinding(key.WithKeys("up")),
	Down:    key.NewBinding(key.WithKeys("down")),
	Enter:   key.NewBinding(key.WithKeys("enter")),
	NewSess: key.NewBinding(key.WithKeys("n")),
	Quit:    key.NewBinding(key.WithKeys("ctrl+c")),
}

func nowMillis() int64 { return time.Now().UnixMilli() }

// maxHomeSessions locks home to the newest 50 sessions; the server already
// lists them updated-desc.
const maxHomeSessions = 50

// relTime renders locked relative time: <60s "12s", <60m "5m", <24h "3h",
// else "4d".
func relTime(then, now int64) string {
	d := now - then
	if d < 0 {
		d = 0
	}
	s := d / 1000
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm", s/60)
	case s < 86400:
		return fmt.Sprintf("%dh", s/3600)
	default:
		return fmt.Sprintf("%dd", s/86400)
	}
}

// lineParts splits a session row into its title and the metadata tail
// (" · provider/model · relTime", dimmed like upstream's per-row footer);
// title+meta is byte-identical to the old lineContent output.
func lineParts(s protocol.Session, now int64) (title, meta string) {
	title = s.Title
	if s.Model != nil {
		meta += " \u00B7 " + s.Model.ProviderID + "/" + s.Model.ID
	}
	return title, meta + " \u00B7 " + relTime(s.Time.Updated, now)
}

// visible returns the sessions home renders.
func (h *homeModel) visible(s *store.State) []protocol.Session {
	ses := s.Sessions
	if len(ses) > maxHomeSessions {
		ses = ses[:maxHomeSessions]
	}
	return ses
}

func (h *homeModel) lineCount(s *store.State) int { return 1 + len(h.visible(s)) }

func (h *homeModel) clampCursor(s *store.State) {
	if h.cursor >= h.lineCount(s) {
		h.cursor = h.lineCount(s) - 1
	}
	if h.cursor < 0 {
		h.cursor = 0
	}
}

func (h *homeModel) moveCursor(s *store.State, d int) {
	h.clampCursor(s)
	n := h.lineCount(s)
	h.cursor = ((h.cursor+d)%n + n) % n
}

const helpText = "\u2191/\u2193 move \u00B7 enter open \u00B7 n new \u00B7 /help"

func (h *homeModel) render(s *store.State, w int, th theme.Theme) string {
	return h.renderClamped(s, w, th, -1)
}

// renderClamped is render with the recent-session row count capped (maxRows
// -1 = all; the modal stack, S2.2, clamps the chrome so the panel fits).
// It produces the locked home layout for the store: the 4-line upstream
// logo (S0.8), the session rows word-wrapped at the terminal width (the
// cursor stays one stop per session — continuation lines align under the
// content), the theme borderSubtle divider and the dimmed help line.
func (h *homeModel) renderClamped(s *store.State, w int, th theme.Theme, maxRows int) string {
	h.clampCursor(s)
	rows := h.visible(s)
	if maxRows >= 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	var b strings.Builder
	b.WriteString(renderLogo(th))
	b.WriteByte('\n')
	b.WriteString(h.renderRow(0, "New session", "", w, th))
	b.WriteByte('\n')
	for i, se := range rows {
		title, meta := lineParts(se, h.now())
		b.WriteString(h.renderRow(i+1, title, meta, w, th))
		b.WriteByte('\n')
	}
	b.WriteString(th.BorderSubtle().Render(dividerLine()))
	b.WriteByte('\n')
	b.WriteString(dimWrapped(th, helpText, w))
	return b.String()
}

// rowLead splits the row prefix into its leading-space lead (rendered
// plain) and the styled body ("  ▸ " is two plain spaces + the ▸ run).
func rowLead(prefix string) (lead, body string) {
	body = strings.TrimLeft(prefix, " \t")
	return prefix[:len(prefix)-len(body)], body
}

// rowLine is one visual line of a wrapped home row, split into its styled
// runs: cur the "▸" run (cursor rows, first line only), title the title run
// (its trailing join space when the line continues into the meta), meta the
// " · provider/model · relTime" tail.
type rowLine struct {
	cur   string
	title string
	meta  string
}

// wTag is one word of a home row tagged with its run: 0 prefix (the "▸"
// body), 1 title, 2 meta.
type wTag struct {
	word string
	seg  int
}

// rowLines wraps the plain home row (prefix + title + meta) at w with the
// same word-wrap contract as wrapLine (word boundaries, over-long tokens
// hard-split at the width, single-space rejoin) and re-derives the
// title/meta split per visual line. A row that fits is returned verbatim as
// one line (internal spacing preserved).
func rowLines(prefix, title, meta string, w int) []rowLine {
	lead, body := rowLead(prefix)
	plain := prefix + title + meta
	if w < 1 || plain == "" {
		return []rowLine{{cur: body, title: title, meta: meta}}
	}
	var words []wTag
	add := func(s string, seg int) {
		for _, f := range strings.Fields(s) {
			words = append(words, wTag{f, seg})
		}
	}
	add(body, 0)
	add(title, 1)
	add(meta, 2)
	effW := w - runeWidth(lead)
	if effW < 1 {
		effW = 1
	}
	var (
		lines []rowLine
		cur   []wTag
		curW  int
	)
	flush := func() {
		if len(cur) == 0 {
			return
		}
		lines = append(lines, joinRowLine(cur))
		cur, curW = cur[:0], 0
	}
	for _, wd := range words {
		fw := runeWidth(wd.word)
		if fw > effW {
			flush()
			for rest := wd.word; len(rest) > 0; {
				chunk, r := cutWidth(rest, effW)
				lines = append(lines, joinRowLine([]wTag{{chunk, wd.seg}}))
				rest = r
			}
			continue
		}
		switch {
		case len(cur) == 0:
			cur, curW = append(cur, wd), fw
		case curW+1+fw <= effW:
			cur, curW = append(cur, wd), curW+1+fw
		default:
			flush()
			cur, curW = append(cur, wd), fw
		}
	}
	flush()
	return lines
}

// joinRowLine joins the tagged words of one visual line into its styled
// runs; a join space belongs to the PRECEDING word's run (a run's trailing
// space is where the next run starts on the same line; a line-boundary
// boundary drops it, as wrapLine drops leading spaces on continuation
// lines).
func joinRowLine(ws []wTag) rowLine {
	var l rowLine
	for i, wd := range ws {
		var p *string
		switch wd.seg {
		case 0:
			p = &l.cur
		case 1:
			p = &l.title
		default:
			p = &l.meta
		}
		*p += wd.word
		if i < len(ws)-1 {
			*p += " "
		}
	}
	return l
}

// renderRow renders one home row (line 0 is the "New session" row). The
// cursor row is the SELECTED row (upstream dialog-select active row): every
// rendered line is painted with the selection background (theme primary —
// upstream `option.bg ?? theme.primary`) and the text in
// SelectedForeground; the "▸" cursor run and the title run are bold
// (upstream bolds the active title), the metadata tail is not (upstream's
// dimmed description/footer runs). The background covers each rendered
// line's content only — no background on the plain indent or the empty
// tail beyond the content. Other rows: the title in the theme text token,
// the metadata tail in textMuted, no background. A zero Theme (nil-engine
// runs, S0.7) degrades: the cursor row keeps the cursorStyle bold (plain —
// a zero Theme has no text token) on the "▸" run with plain content, every
// other row plain — never a panic.
func (h *homeModel) renderRow(line int, title, meta string, w int, th theme.Theme) string {
	cursor := line == h.cursor
	prefix := "  "
	if cursor {
		prefix = "  \u25B8 "
	}
	lead, _ := rowLead(prefix)
	lines := rowLines(prefix, title, meta, w)
	ind := 2
	if cursor {
		ind = 4
	}
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			n := ind
			if ww := runeWidth(l.cur) + runeWidth(l.title) + runeWidth(l.meta); ww+n > w {
				n = w - ww
				if n < 0 {
					n = 0
				}
			}
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(" ", n))
		} else {
			b.WriteString(lead)
		}
		writeRowLine(&b, l, cursor, th)
	}
	return b.String()
}

// writeRowLine renders one visual line's styled runs (see renderRow).
func writeRowLine(b *strings.Builder, l rowLine, selected bool, th theme.Theme) {
	if !selected {
		if l.title != "" {
			b.WriteString(th.Text().Render(l.title))
		}
		if l.meta != "" {
			b.WriteString(th.TextMuted().Render(l.meta))
		}
		return
	}
	bg, ok := th.Color("primary")
	if !ok {
		// zero Theme: the cursorStyle bold (plain — a zero Theme has no
		// text token) + plain content
		if l.cur != "" {
			b.WriteString(cursorStyle(th).Render(l.cur))
		}
		b.WriteString(l.title + l.meta)
		return
	}
	sel := th.SelectedForeground()
	fg := lipgloss.Color(sel.Hex()[:7])
	bgStyle := lipgloss.Color(bg.Hex()[:7])
	head := lipgloss.NewStyle().Foreground(fg).Background(bgStyle).Bold(true)
	tail := lipgloss.NewStyle().Foreground(fg).Background(bgStyle)
	if l.cur != "" || l.title != "" {
		b.WriteString(head.Render(l.cur + l.title))
	}
	if l.meta != "" {
		b.WriteString(tail.Render(l.meta))
	}
}

// handleHomeKey dispatches home-route keys: up/down wrap, enter opens or
// creates, n creates, esc clears the prompt; unhandled keys fall through to
// the prompt input. (ctrl+c is handled app-wide in handleKey.)
func (a *App) handleHomeKey(k tea.KeyPressMsg) ([]tea.Cmd, bool) {
	switch {
	case key.Matches(k, homeKeyMap.Up):
		a.home.moveCursor(&a.store, -1)
		return nil, true
	case key.Matches(k, homeKeyMap.Down):
		a.home.moveCursor(&a.store, 1)
		return nil, true
	case key.Matches(k, homeKeyMap.Enter):
		return a.homeEnter(), true
	case key.Matches(k, homeKeyMap.NewSess):
		return a.emit(a.createSessionCmd()), true
	case key.Matches(k, escBinding):
		a.prompt.input.SetValue("")
		return nil, true
	}
	return nil, false
}

func (a *App) homeEnter() []tea.Cmd {
	if a.home.cursor == 0 {
		return a.emit(a.createSessionCmd())
	}
	rows := a.home.visible(&a.store)
	idx := a.home.cursor - 1
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	a.openSession(rows[idx].ID)
	return a.emit(a.hydrateCmd())
}
