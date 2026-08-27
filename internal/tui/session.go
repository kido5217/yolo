package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// sessionModel holds session-route state: the transcript viewport, the
// expanded part set (tool I/O blocks and reasoning text) and the auto-follow
// flag.
type sessionModel struct {
	vm        viewport.Model
	expanded  map[string]bool
	following bool
	isDirty   bool // transcript needs a re-render (store mutation, expand toggle)
	content   string
}

var sessKeyMap = struct {
	PageUp   key.Binding
	PageDown key.Binding
	Expand   key.Binding
	Think    key.Binding
}{
	PageUp:   key.NewBinding(key.WithKeys("pgup")),
	PageDown: key.NewBinding(key.WithKeys("pgdown")),
	// T25 (deviation 51): the always-focused prompt needs plain e/t for
	// typing, so the toggles moved to alt+e / alt+t (both unbound by
	// textinput's DefaultKeyMap).
	Expand: key.NewBinding(key.WithKeys("alt+e")),
	Think:  key.NewBinding(key.WithKeys("alt+t")),
}

func newSessionModel(w, h int) sessionModel {
	return sessionModel{
		vm:        viewport.New(viewport.WithWidth(w), viewport.WithHeight(h)),
		expanded:  map[string]bool{},
		following: true,
		isDirty:   true,
	}
}

func sessionBusy(st *store.State) bool {
	switch st.Status.Type {
	case protocol.SessionStatusBusy, protocol.SessionStatusRetry:
		return true
	}
	return false
}

// const sessionHelp locks the session-route footer help line.
const sessionHelp = "pgup/pgdn scroll \u00B7 alt+e expand \u00B7 alt+t think \u00B7 esc abort/back"

// sync updates viewport size/content and applies auto-follow: while the
// session is busy and follow is on, the viewport stays pinned to the bottom;
// a user scroll-up (pgup) pauses follow until pgdn reaches the bottom again.
// The transcript re-renders only when dirty (store mutation or expand
// toggle); frames that only advance the footer spinner or report a status
// tick reuse the existing viewport content instead of rebuilding it.
func (sm *sessionModel) sync(st *store.State, w, h int, th theme.Theme, spin string) {
	if sm.vm.Width() != w || sm.vm.Height() != h {
		sm.vm.SetWidth(w)
		sm.vm.SetHeight(h)
	}
	if sm.isDirty {
		sm.isDirty = false
		content := renderMessages(st, sm.expanded, w, th, spin)
		if content != sm.content {
			sm.content = content
			sm.vm.SetContent(content)
		}
	}
	if sm.following && sessionBusy(st) {
		sm.vm.GotoBottom()
	}
}

// renderMessages renders the current session's transcript as viewport
// content: user messages verbatim, assistant parts in arrival order
// (reasoning as the spinner/Thought row, tool rows in the S1.7 glyph form —
// "~ <pending>" running, "<icon> <title>" completed, "<icon>
// <failure ?? title>" error), a divider before every message after the
// first, and message errors as a red "! message" line. expanded maps partID
// to the parts whose I/O block or reasoning text is shown; spin is the
// footer spinner frame for the running reasoning + read rows ("" in unit
// runs).
func renderMessages(st *store.State, expanded map[string]bool, w int, th theme.Theme, spin string) string {
	var tr, rr *theme.Renderer
	if !th.Zero() {
		if built, err := theme.NewTranscriptRenderer(th, w-3); err == nil {
			tr = built
		}
		if built, err := theme.NewReasoningRenderer(th, w-5); err == nil {
			rr = built
		}
	}
	blocks := make([]string, 0, len(st.Messages))
	for _, m := range st.Messages {
		if m.Info.Role == "user" {
			blocks = append(blocks, renderUser(m, w))
		} else {
			blocks = append(blocks, renderAssistant(m, expanded, w, th, tr, rr, spin))
		}
	}
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(blocks[0])
	for _, blk := range blocks[1:] {
		b.WriteByte('\n')
		b.WriteString(divider.Render(dividerLine()))
		b.WriteByte('\n')
		b.WriteString(blk)
	}
	return b.String()
}

func renderUser(m protocol.MessageWithParts, w int) string {
	var texts []string
	for _, p := range m.Parts {
		if p.Type == "text" && p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	if len(texts) == 0 {
		return "User:"
	}
	var b strings.Builder
	lines := strings.Split(strings.Join(texts, "\n"), "\n")
	for i, l := range lines {
		if i == 0 {
			l = "User: " + l
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(wrapLine(l, w))
	}
	return b.String()
}

func renderAssistant(m protocol.MessageWithParts, expanded map[string]bool, w int, th theme.Theme, tr, rr *theme.Renderer, spin string) string {
	var b strings.Builder
	first := true
	writeRaw := func(s string) {
		if !first {
			b.WriteByte('\n')
		}
		b.WriteString(s)
		first = false
	}
	// writePlain word-wraps plain text at the viewport width (styled strings
	// must never reach wrapLine: it does not parse ANSI escapes).
	writePlain := func(s string) {
		for _, l := range strings.Split(wrapLine(s, w), "\n") {
			writeRaw(l)
		}
	}
	writeStyled := func(sty lipgloss.Style, s string) {
		for _, l := range strings.Split(wrapLine(s, w), "\n") {
			writeRaw(sty.Render(l))
		}
	}
	for _, p := range m.Parts {
		switch p.Type {
		case "text":
			if p.Text == "" {
				continue
			}
			if tr == nil {
				for _, l := range strings.Split(p.Text, "\n") {
					writePlain(l)
				}
				continue
			}
			// The upstream TextPart is a 3-column-indented markdown block
			// (index.tsx:1700-1707). The renderer word-wraps at w-3
			// (WithWordWrap), so the indented lines already fit w — the
			// styled output never reaches wrapLine.
			if out, err := tr.Render(p.Text); err == nil {
				for _, l := range strings.Split(strings.Trim(out, "\n"), "\n") {
					writeRaw("   " + l)
				}
			} else {
				for _, l := range strings.Split(p.Text, "\n") {
					writePlain(l)
				}
			}
		case "reasoning":
			text := strings.TrimSpace(strings.ReplaceAll(p.Text, "[REDACTED]", ""))
			if text == "" {
				continue
			}
			done := p.Time.End != 0
			dur := int64(0)
			if done {
				dur = p.Time.End - p.Time.Start
				if dur < 0 {
					dur = 0
				}
			}
			title, body := reasoningSummary(text)
			open := expanded[p.ID]
			// The upstream header fg: warning PRE-BLENDED at ThinkingOpacity
			// while running (the Spinner color, index.tsx:1660) and when
			// open (1657-1659); full warning when done+closed.
			var hdr lipgloss.Style
			if !done || open {
				hdr = th.WarningSubtle()
			} else {
				hdr = th.Warning()
			}
			row := ""
			switch {
			case !done:
				label := "Thinking"
				if title != "" {
					label = "Thinking: " + title
				}
				if spin != "" {
					row = spin + " " + label
				} else {
					row = label // zero-theme/unit runs pass "" — no leading space
				}
			case title != "" && dur > 0:
				row = openMark(open) + " Thought: " + title + " · " + durationText(dur)
			case title != "":
				row = openMark(open) + " Thought: " + title
			case dur > 0:
				row = openMark(open) + " Thought: " + durationText(dur)
			default:
				row = openMark(open) + " Thought"
			}
			writeStyled(hdr, row)
			if open && body != "" && rr != nil {
				if out, err := rr.Render(body); err == nil {
					for _, l := range strings.Split(strings.Trim(out, "\n"), "\n") {
						writeRaw("     " + l) // 3 (part box) + 2 (body box)
					}
				}
			}
		case "tool":
			sty, row, ok := toolRow(p, th, spin)
			if !ok {
				continue
			}
			writeStyled(sty, row)
			switch {
			case expanded[p.ID] && p.State != nil && p.State.Status == "error":
				// The upstream expanded error (InlineToolRow 1992-1999):
				// the FULL error at the icon width (2), fg=error.
				if p.State.Error != "" {
					for _, l := range strings.Split(p.State.Error, "\n") {
						writeStyled(th.Error(), "  "+l)
					}
				}
			case expanded[p.ID] && p.State != nil:
				block := tailLines(p.State.Output, 40)
				if block == "" {
					continue
				}
				for _, l := range strings.Split(block, "\n") {
					writePlain("  " + l)
				}
			case p.Tool == "bash" && p.State != nil && p.State.Status == "completed":
				// Inline preview (S0 lock): the 10-line head.
				if block := headPreview(p.State.Output, 10); block != "" {
					for _, l := range strings.Split(block, "\n") {
						writePlain("  " + l)
					}
				}
			}
		}
	}
	if m.Info.Error != nil {
		writeStyled(th.Error(), "! "+m.Info.Error.Message)
	}
	return b.String()
}

// reasoningSummary ports upstream thinking.ts:12: the leading **title**
// block is disclosure metadata; the rest is the markdown body.
func reasoningSummary(text string) (title string, body string) {
	content := strings.TrimSpace(text)
	i := strings.Index(content, "**")
	if i != 0 {
		return "", content
	}
	j := strings.Index(content[2:], "**")
	if j < 0 {
		return "", content
	}
	title = strings.TrimSpace(content[2 : 2+j])
	if title == "" {
		return "", content
	}
	rest := content[2+j+2:]
	if rest == "" {
		return title, ""
	}
	// the upstream regex requires the title block to end at a blank line
	// ((\r?\n) twice, mixed endings allowed) or the end of the content;
	// the body is what follows the blank line.
	i = 0
	for n := 0; n < 2; n++ {
		if i < len(rest) && rest[i] == '\r' {
			i++
		}
		if i < len(rest) && rest[i] == '\n' {
			i++
		} else {
			return "", content
		}
	}
	return title, strings.TrimRight(rest[i:], " \t\r\n")
}

// durationText ports upstream Locale.duration (util/locale.ts:39): ms <
// 1s, X.Xs < 1m, Nm Ns < 1h, Nh Nm < 24h, else Nd Nh.
func durationText(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	if ms < 3600000 {
		return fmt.Sprintf("%dm %ds", ms/60000, (ms%60000)/1000)
	}
	if ms < 86400000 {
		return fmt.Sprintf("%dh %dm", ms/3600000, (ms%3600000)/60000)
	}
	return fmt.Sprintf("%dd %dh", ms/86400000, (ms%86400000)/3600000)
}

// openMark is the done-reasoning header prefix mark: "-" open, "+"
// collapsed (the single separating space lives in the row string — the
// brief's spaced mark + spaced row string built a double space; the
// binding parity note pins the single-space form, deviation 156).
func openMark(open bool) string {
	if open {
		return "-"
	}
	return "+"
}

// toolGlyph is the per-tool icon (upstream InlineTool icon props,
// index.tsx:2105-2545 — the 2-column slot is the glyph + space).
func toolGlyph(tool string) string {
	switch tool {
	case "bash":
		return "$"
	case "write", "edit":
		return "←"
	case "glob", "grep":
		return "✱"
	case "read":
		return "→"
	default:
		return "⚙"
	}
}

// toolPending is the upstream pending text (the running row, index.tsx
// pending= props).
func toolPending(tool string) string {
	switch tool {
	case "bash":
		return "Writing command..."
	case "write":
		return "Preparing write..."
	case "edit":
		return "Preparing edit..."
	case "glob":
		return "Finding files..."
	case "grep":
		return "Searching content..."
	case "read":
		return "Reading file..."
	case "todowrite":
		return "Updating todos..."
	default:
		return "Working..."
	}
}

// toolFailure is the upstream failure= prop (the error row text when the
// part has no title).
func toolFailure(tool string) string {
	if tool == "todowrite" {
		return "Todo update failed"
	}
	return ""
}

// toolRow renders the upstream InlineTool row: the running row is "~
// <pending>" at fg=text (read: "<spin> Reading file..."), the completed
// row "<icon> <title>" at fg=textMuted, the error row "<icon>
// <failure ?? title>" at fg=error (index.tsx:1882-1889, 1966-1990).
// A zero Theme degrades to plain rows (the S0.10 contract).
func toolRow(p protocol.Part, th theme.Theme, spin string) (lipgloss.Style, string, bool) {
	st := p.State
	status := "running"
	title := ""
	if st != nil {
		status = st.Status
		title = st.Title
	}
	if title == "" {
		title = toolTitleFallback(p)
	}
	icon := toolGlyph(p.Tool) + " "
	switch status {
	case "completed":
		return th.TextMuted(), icon + title, true
	case "error":
		text := toolFailure(p.Tool)
		if text == "" {
			text = title
		}
		return th.Error(), icon + text, true
	default:
		// read: the upstream spinner row (spinner={isRunning()}); a
		// zero-spin caller (zero-theme/unit runs) degrades to "~".
		if p.Tool == "read" && spin != "" {
			return th.Text(), spin + " " + toolPending("read"), true
		}
		return th.Text(), "~ " + toolPending(p.Tool), true
	}
}

// toolTitleFallback applies the locked rule for an empty state.Title: the
// first input argument stringified, else the callID prefix 8.
func toolTitleFallback(p protocol.Part) string {
	if st := p.State; st != nil && st.Input != nil {
		for _, k := range []string{"path", "command", "pattern", "input"} {
			if v, ok := st.Input[k].(string); ok {
				return v
			}
		}
		keys := make([]string, 0, len(st.Input))
		for k := range st.Input {
			if k == "input" {
				continue
			}
			keys = append(keys, k)
		}
		if len(keys) > 0 {
			sort.Strings(keys)
			return fmt.Sprintf("%v", st.Input[keys[0]])
		}
	}
	if len(p.CallID) >= 8 {
		return p.CallID[:8]
	}
	return p.CallID
}

// headPreview returns the first n lines of s for the inline output preview:
// unchanged when it fits, first n lines plus a "…" line when it overflows
// (upstream collapseToolOutput parity), "" for empty output.
func headPreview(s string, n int) string {
	if s == "" || n <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	head := lines[:n]
	head = append(head, "\u2026")
	return strings.Join(head, "\n")
}

func tailLines(s string, n int) string {
	if s == "" || n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// lastToolPartID returns the ID of the most recent tool part in the
// transcript (the part "e" toggles).
func lastToolPartID(st *store.State) string {
	id := ""
	for _, m := range st.Messages {
		for _, p := range m.Parts {
			if p.Type == "tool" {
				id = p.ID
			}
		}
	}
	return id
}

// handleSessionKey dispatches session-route keys: pgup/pgdn scroll, alt+e
// expands the most recent tool part, alt+t toggles reasoning, esc aborts
// while busy and returns to home when idle. It reports whether the key was
// consumed; unhandled keys fall through to the prompt input.
func (a *App) handleSessionKey(k tea.KeyPressMsg) ([]tea.Cmd, bool) {
	switch {
	case key.Matches(k, sessKeyMap.PageUp):
		a.sess.vm.PageUp()
		a.sess.following = false
		return nil, true
	case key.Matches(k, sessKeyMap.PageDown):
		a.sess.vm.PageDown()
		a.sess.following = a.sess.vm.AtBottom()
		return nil, true
	case key.Matches(k, sessKeyMap.Expand):
		id := lastToolPartID(&a.store)
		if id == "" {
			return nil, true
		}
		if a.sess.expanded[id] {
			delete(a.sess.expanded, id)
		} else {
			a.sess.expanded[id] = true
		}
		a.sess.isDirty = true
		return nil, true
	case key.Matches(k, sessKeyMap.Think):
		expand := false
		ids := []string{}
		for _, m := range a.store.Messages {
			for _, p := range m.Parts {
				if p.Type != "reasoning" {
					continue
				}
				ids = append(ids, p.ID)
				if !a.sess.expanded[p.ID] {
					expand = true
				}
			}
		}
		for _, id := range ids {
			if expand {
				a.sess.expanded[id] = true
			} else {
				delete(a.sess.expanded, id)
			}
		}
		if expand {
			a.sess.isDirty = true
		}
		return nil, true
	case key.Matches(k, escBinding):
		if sessionBusy(&a.store) {
			return a.emit(a.abortCmd()), true
		}
		a.route = routeHome
		a.curSessionID = ""
		return a.emit(a.hydrateCmd()), true
	}
	return nil, false
}
