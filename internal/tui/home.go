package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// homeModel holds home-route state: cursor line, the "/<cmd>" input buffer,
// and the relative-time clock.
type homeModel struct {
	cursor int
	buf    string
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

func lineContent(s protocol.Session, now int64) string {
	c := s.Title
	if s.Model != nil {
		c += " \u00B7 " + s.Model.ProviderID + "/" + s.Model.ID
	}
	return c + " \u00B7 " + relTime(s.Time.Updated, now)
}

// visible returns the sessions home renders.
func (h *homeModel) visible(s *store.Store) []protocol.Session {
	ses := s.Sessions
	if len(ses) > maxHomeSessions {
		ses = ses[:maxHomeSessions]
	}
	return ses
}

func (h *homeModel) lineCount(s *store.Store) int { return 1 + len(h.visible(s)) }

func (h *homeModel) clampCursor(s *store.Store) {
	if h.cursor >= h.lineCount(s) {
		h.cursor = h.lineCount(s) - 1
	}
	if h.cursor < 0 {
		h.cursor = 0
	}
}

func (h *homeModel) moveCursor(s *store.Store, d int) {
	h.clampCursor(s)
	n := h.lineCount(s)
	h.cursor = ((h.cursor+d)%n + n) % n
}

const helpText = "\u2191/\u2193 move \u00B7 enter open \u00B7 n new \u00B7 /help"

// render produces the locked home layout for the store.
func (h *homeModel) render(s *store.Store) string {
	h.clampCursor(s)
	rows := h.visible(s)
	var b strings.Builder
	b.WriteString(title.Render("Yolo"))
	b.WriteByte('\n')
	b.WriteString(divider.Render(dividerLine()))
	b.WriteByte('\n')
	b.WriteString(h.renderRow(0, "New session"))
	b.WriteByte('\n')
	for i, se := range rows {
		b.WriteString(h.renderRow(i+1, lineContent(se, h.now())))
		b.WriteByte('\n')
	}
	b.WriteString(divider.Render(dividerLine()))
	b.WriteByte('\n')
	b.WriteString(dim.Render(helpText))
	return b.String()
}

func (h *homeModel) renderRow(line int, content string) string {
	if line == h.cursor {
		return cursor.Render("  \u25B8 " + content)
	}
	return "  " + content
}

// handleHomeKey dispatches home-route keys: up/down wrap, enter opens or
// creates, n creates, ctrl+c asks to quit, printable input feeds the command
// buffer.
func (a *App) handleHomeKey(k tea.KeyPressMsg) []tea.Cmd {
	switch {
	case key.Matches(k, homeKeyMap.Up):
		a.home.moveCursor(&a.store, -1)
		return nil
	case key.Matches(k, homeKeyMap.Down):
		a.home.moveCursor(&a.store, 1)
		return nil
	case key.Matches(k, homeKeyMap.Enter):
		return a.homeEnter()
	case key.Matches(k, homeKeyMap.NewSess):
		a.home.buf = ""
		return a.emit(a.createSessionCmd())
	case key.Matches(k, homeKeyMap.Quit):
		a.dlg.push(dialog{kind: dlgQuit})
		return nil
	default:
		return a.homeCommand(k)
	}
}

func (a *App) homeEnter() []tea.Cmd {
	if a.home.cursor == 0 {
		a.home.buf = ""
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

// homeCommand accumulates the "/<cmd>" buffer; "/help" opens the help dialog,
// any other input clears the buffer. Only /help exists at T23.
func (a *App) homeCommand(k tea.KeyPressMsg) []tea.Cmd {
	t := k.Text
	if t == "" {
		return nil
	}
	a.home.buf += t
	if a.home.buf == "/help" {
		a.home.buf = ""
		a.dlg.push(dialog{kind: dlgHelp})
		return nil
	}
	if !strings.HasPrefix("/help", a.home.buf) {
		a.home.buf = ""
	}
	return nil
}
