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
func (sm *sessionModel) sync(st *store.State, w, h int, th theme.Theme) {
	if sm.vm.Width() != w || sm.vm.Height() != h {
		sm.vm.SetWidth(w)
		sm.vm.SetHeight(h)
	}
	if sm.isDirty {
		sm.isDirty = false
		content := renderMessages(st, sm.expanded, w, th)
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
// content: user messages verbatim, assistant parts in arrival order (reasoning
// collapsed as "▸ think", tool rows "✓/▶/✗ <tool> <title>"), a divider before
// every message after the first, and message errors as a red "! message" line.
// expanded maps partID to the parts whose I/O block or reasoning text is
// shown.
func renderMessages(st *store.State, expanded map[string]bool, w int, th theme.Theme) string {
	blocks := make([]string, 0, len(st.Messages))
	for _, m := range st.Messages {
		if m.Info.Role == "user" {
			blocks = append(blocks, renderUser(m, w))
		} else {
			blocks = append(blocks, renderAssistant(m, expanded, w, th))
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

func renderAssistant(m protocol.MessageWithParts, expanded map[string]bool, w int, th theme.Theme) string {
	muted := th.TextMuted()
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
			for _, l := range strings.Split(p.Text, "\n") {
				writePlain(l)
			}
		case "reasoning":
			if expanded[p.ID] && p.Text != "" {
				writeStyled(muted, "\u25BE think")
				for _, l := range strings.Split(p.Text, "\n") {
					writeStyled(muted, "  "+l)
				}
			} else {
				writeStyled(muted, "\u25B8 think")
			}
		case "tool":
			sty, row, ok := toolRowLine(p)
			if !ok {
				continue
			}
			writeStyled(sty, row)
			switch {
			case expanded[p.ID] && p.State != nil:
				block := tailLines(p.State.Output, 40)
				if p.State.Status == "error" {
					block = p.State.Error
				}
				if block == "" {
					continue
				}
				for _, l := range strings.Split(block, "\n") {
					writePlain("  " + l)
				}
			case p.Tool == "bash" && p.State != nil && p.State.Status == "completed":
				// Inline preview (upstream parity): a completed bash part
				// shows the 10-line head of its output without alt+e.
				if block := headPreview(p.State.Output, 10); block != "" {
					for _, l := range strings.Split(block, "\n") {
						writePlain("  " + l)
					}
				}
			}
		}
	}
	if m.Info.Error != nil {
		writeStyled(errRed, "! "+m.Info.Error.Message)
	}
	return b.String()
}

// toolRowLine renders the locked tool row: "✓ <tool> <title>" completed,
// "▶ <tool> <title>" running, "✗ <tool> <error>" error (first error line).
// The caller applies the style per wrapped line (the row may wrap).
func toolRowLine(p protocol.Part) (lipgloss.Style, string, bool) {
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
	switch status {
	case "completed":
		return okGreen, "\u2713 " + p.Tool + " " + title, true
	case "error":
		errText := ""
		if st != nil {
			errText = st.Error
		}
		if i := strings.IndexByte(errText, '\n'); i >= 0 {
			errText = errText[:i]
		}
		if errText == "" {
			errText = title
		}
		return errRed, "\u2717 " + p.Tool + " " + errText, true
	default:
		return toolRow, "\u25B6 " + p.Tool + " " + title, true
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
