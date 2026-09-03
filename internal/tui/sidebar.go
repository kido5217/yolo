// sidebar.go — the todo sidebar (S7.1): the latest todowrite part → the
// status-glyph list. The upstream feature-plugins/sidebar/todo.tsx +
// component/todo-item.tsx @ v1.18.18 port: the show gate (todo.tsx:12),
// the [✓]/[•]/[ ] glyph runs + the warning/muted fg (todo-item.tsx), the
// ▼-only-when-len>2 header. The block is always expanded (the upstream
// mouse collapse has no yolo referent — no mouse; deviation 246). The list
// referent is the last todowrite part's State.Input["todos"] (the
// wire-decoded shape) or State.Metadata["todos"] (the in-memory shape) —
// both decode via a marshal/unmarshal round-trip (the S7 detail finding:
// the wire shape is sufficient, no wire change).

package tui

import (
	"encoding/json"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// latestTodos returns the last todowrite part's todo list (the upstream
// session.todo() referent over the yolo part walk): the LAST part of type
// tool / tool id "todowrite" with a decodable todos list; nil when none
// (an undecodable part is skipped — it does not shadow an earlier one).
func latestTodos(st *store.State) []protocol.Todo {
	var latest []protocol.Todo
	found := false
	for _, m := range st.Messages {
		for _, p := range m.Parts {
			if p.Type != "tool" || p.Tool != "todowrite" || p.State == nil {
				continue
			}
			if todos, ok := todosFromPart(p); ok {
				latest, found = todos, true
			}
		}
	}
	if !found {
		return nil
	}
	return latest
}

// todosFromPart decodes the part's todos list: State.Input["todos"] (the
// wire-decoded []any of map[string]any) first, else
// State.Metadata["todos"] (the in-memory []protocol.Todo — []any after a
// wire round-trip). A marshal/unmarshal round-trip normalizes both shapes
// into []protocol.Todo.
func todosFromPart(p protocol.Part) ([]protocol.Todo, bool) {
	v, ok := p.State.Input["todos"]
	if !ok {
		v, ok = p.State.Metadata["todos"]
	}
	if !ok {
		return nil, false
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var todos []protocol.Todo
	if json.Unmarshal(raw, &todos) != nil {
		return nil, false
	}
	return todos, true
}

// todoBlockVisible ports the upstream show gate (todo.tsx:12): the block
// is hidden for an empty list and an all-completed list (a cancelled item
// counts as open — status != "completed").
func todoBlockVisible(todos []protocol.Todo) bool {
	if len(todos) == 0 {
		return false
	}
	for _, td := range todos {
		if td.Status != "completed" {
			return true
		}
	}
	return false
}

// todoGlyphRun is the upstream todo-item glyph run (bracket + glyph +
// bracket + trailing space, 4 columns): [✓] completed, [•] in_progress,
// [ ] every other status (pending, cancelled).
func todoGlyphRun(status string) string {
	switch status {
	case "completed":
		return "[✓] "
	case "in_progress":
		return "[•] "
	default:
		return "[ ] "
	}
}

// todoRow is the plain (unstyled) todo-block line: the display text + the
// item status (the fg source) + the header flag. The styled render (the
// S7.1 pin) and the S7.2 panel padding both derive from this layout.
type todoRow struct {
	plain      string
	inProgress bool
	header     bool
}

// todoSidebarRows is the plain todo-block layout: the header row (the ▼
// collapse glyph only when len > 2 — the block is always expanded,
// deviation 246) + one line per item (the glyph run + the content
// word-wrapped at w-4, the 4 continuation columns).
func todoSidebarRows(todos []protocol.Todo, w int) []todoRow {
	contentW := w - 4 // the glyph run width
	if contentW < 1 {
		contentW = 1
	}
	header := "Todo"
	if len(todos) > 2 {
		header = "▼ " + header
	}
	rows := []todoRow{{plain: header, header: true}}
	for _, td := range todos {
		ip := td.Status == "in_progress"
		cols := strings.Split(wrapLine(td.Content, contentW), "\n")
		for i, col := range cols {
			if i == 0 {
				rows = append(rows, todoRow{plain: todoGlyphRun(td.Status) + col, inProgress: ip})
			} else {
				rows = append(rows, todoRow{plain: "    " + col, inProgress: ip})
			}
		}
	}
	return rows
}

// todoSidebarLines renders the todo block with the upstream fg (the glyph
// + content share the fg — warning for in_progress, else textMuted; the
// header is the bold text token).
func todoSidebarLines(todos []protocol.Todo, w int, th theme.Theme) []string {
	rows := todoSidebarRows(todos, w)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		switch {
		case r.header:
			out = append(out, th.Text().Render(r.plain))
		case r.inProgress:
			out = append(out, th.Warning().Render(r.plain))
		default:
			out = append(out, th.TextMuted().Render(r.plain))
		}
	}
	return out
}

// sidebarWidth is the upstream sidebar shell width (sidebar.tsx).
const sidebarWidth = 42

// sidebarInnerW is the panel content width (the sidebarWidth minus the
// paddingLeft 2 + paddingRight 2; the upstream content box's paddingRight
// 1 is absorbed into the panel padding — deviation 246).
const sidebarInnerW = 38

// sideRow is the plain content line of the sidebar panel: the display text
// + the fg style (the panel bg is applied to the pad tail — the
// deletefailed.go:77 fg+bg composition idiom).
type sideRow struct {
	plain string
	st    lipgloss.Style
}

// sidebarLines renders the sidebar panel as exactly vh lines (each padded
// to sidebarWidth with the backgroundPanel fill — the upstream shell:
// backgroundPanel, paddingTop 1, paddingBottom 1, paddingLeft 2,
// paddingRight 2): the paddingTop blank line (fully panel-painted), the
// session-title block (the bold theme.text, wrapped at sidebarInnerW —
// the upstream sidebar_title minus the sessionID / workspace / share
// spans, no yolo referents), the blank gap line, then the todo block
// (S7.1, hidden by the todoBlockVisible gate), then blank lines to the
// bottom. The fg applies to the content; the pad tail carries the panel
// bg (an outer-width pad over an inner-styled line would not paint).
func (a *App) sidebarLines(vh int) []string {
	panel := a.theme.BackgroundPanel()
	var rows []sideRow
	title := "session"
	if a.store.Current != nil {
		title = a.store.Current.Title
	}
	for _, l := range strings.Split(wrapLine(title, sidebarInnerW), "\n") {
		rows = append(rows, sideRow{plain: l, st: a.theme.Text()})
	}
	if todos := latestTodos(&a.store); todoBlockVisible(todos) {
		rows = append(rows, sideRow{}) // the blank gap line (the panel paint only)
		for _, r := range todoSidebarRows(todos, sidebarInnerW) {
			switch {
			case r.header:
				rows = append(rows, sideRow{plain: r.plain, st: a.theme.Text()})
			case r.inProgress:
				rows = append(rows, sideRow{plain: r.plain, st: a.theme.Warning()})
			default:
				rows = append(rows, sideRow{plain: r.plain, st: a.theme.TextMuted()})
			}
		}
	}
	lines := make([]string, 0, vh)
	for i := 0; i < vh; i++ {
		plain := ""
		var st lipgloss.Style
		if i > 0 && i-1 < len(rows) { // i == 0: the paddingTop blank line
			plain, st = rows[i-1].plain, rows[i-1].st
		}
		body := ""
		if plain != "" {
			body = st.Render(plain)
		}
		if pad := sidebarWidth - runeWidth(plain); pad > 0 {
			body += panel.Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, body)
	}
	return lines
}
