// Package tui is the bubbletea v2 frontend for yolo.
//
// The TUI is a pure client: it talks to the core server only through the wire
// contract (internal/protocol) via internal/tui/client. Non-test files import
// only internal/protocol, internal/tui/*, the standard library, and the charm
// deps.
package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// EventMsg carries one server SSE event. It is exported so the test harness
// can drive the app with it.
type EventMsg struct{ Ev protocol.Event }

// HydrateMsg asks the app to re-hydrate its current route over REST. It is
// exported so the test harness can drive the app with it.
type HydrateMsg struct{}

type route int

const (
	routeHome route = iota
	routeSession
)

// promptModel and toast are placeholders for T25 (prompt) and T29 (toasts).
type promptModel struct{}

type toast struct{ msg string }

// App is the root bubbletea model: routes, store, dialog stack and the SSE
// event pump.
type App struct {
	*client.Client
	store     store.Store
	route     route
	cur       string
	home      homeModel
	sess      sessionModel
	prompt    promptModel
	dlg       dialogStack
	toasts    []toast
	lastErr   string
	// tea plumbing
	size    tea.WindowSizeMsg
	eventCh chan protocol.Event
	stop    context.CancelFunc
	record  bool
	Cmds    []tea.Cmd // test hook: emitted cmds are captured here when record
}

// NewApp builds the root model. A non-empty startSessionID starts on that
// session (resume); empty starts at home.
func NewApp(c *client.Client, s *store.Store, startSessionID string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		Client:  c,
		route:   routeHome,
		home:    homeModel{now: nowMillis},
		sess:    newSessionModel(80, 21),
		size:    tea.WindowSizeMsg{Width: 80, Height: 24},
		eventCh: c.Events(ctx),
		stop:    cancel,
	}
	if s != nil {
		a.store = *s
	}
	if startSessionID != "" {
		a.route = routeSession
		a.cur = startSessionID
	}
	return a
}

// Close stops the SSE pump. Call it once the program exits.
func (a *App) Close() { a.stop() }

// Init hydrates the starting route and arms the SSE pump.
func (a *App) Init() tea.Cmd {
	return tea.Batch(a.hydrateCmd(), a.eventPump())
}

// Update dispatches one message; every state change re-renders on return
// (bubbletea default).
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.size = m
		return a, nil
	case EventMsg:
		a.store.Conn = true
		a.store.Apply(m.Ev)
		return a, a.eventPump()
	case connLostMsg:
		a.store.Conn = false
		return a, nil
	case HydrateMsg:
		return a, a.hydrateCmd()
	case hydratedMsg:
		return a, a.applyHydrate(m)
	case sessionCreatedMsg:
		return a, a.applySessionCreated(m)
	case abortedMsg:
		if m.err != nil {
			a.lastErr = "abort: " + m.err.Error()
		}
		return a, nil
	case tea.KeyPressMsg:
		cmds := a.handleKey(m)
		if len(cmds) == 0 {
			return a, nil
		}
		return a, tea.Batch(cmds...)
	}
	return a, nil
}

// connLostMsg signals the SSE channel closed (ctx done); the client already
// handles reconnects with its internal backoff loop.
type connLostMsg struct{}

// eventPump blocks on the SSE channel and delivers the next event. It re-arms
// itself on every event; on channel close it delivers connLostMsg and stops.
func (a *App) eventPump() tea.Cmd {
	ch := a.eventCh
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return connLostMsg{}
		}
		return EventMsg{Ev: ev}
	}
}

// hydrateMsg payloads are delivered by the hydrate cmd: home lists, session
// details, or the resume not-found case.
type hydratedMsg struct {
	id       string
	list     []protocol.Session
	sess     *protocol.Session
	msgs     []protocol.MessageWithParts
	err      error
	notFound bool
}

func (a *App) hydrateCmd() tea.Cmd {
	if a.route == routeSession && a.cur != "" {
		id := a.cur
		return func() tea.Msg {
			ctx := context.Background()
			ses, err := a.GetSession(ctx, id)
			if errors.Is(err, client.ErrNotFound) {
				return hydratedMsg{id: id, notFound: true}
			}
			if err != nil {
				return hydratedMsg{id: id, err: err}
			}
			msgs, merr := a.ListMessages(ctx, id)
			if merr != nil {
				return hydratedMsg{id: id, sess: &ses, err: merr}
			}
			return hydratedMsg{id: id, sess: &ses, msgs: msgs}
		}
	}
	return func() tea.Msg {
		list, err := a.ListSessions(context.Background())
		return hydratedMsg{list: list, err: err}
	}
}

func (a *App) applyHydrate(m hydratedMsg) tea.Cmd {
	switch {
	case m.notFound:
		// Resume hit a missing session: visible error line, exit to the
		// cmd layer, which maps this Quit to exit code 2 (T30).
		a.lastErr = "session not found: " + m.id
		a.route = routeHome
		a.cur = ""
		return quitCmd()
	case m.err != nil:
		a.lastErr = m.err.Error()
		return nil
	case a.route == routeSession:
		if m.sess != nil {
			cp := *m.sess
			a.store.Current = &cp
		}
		a.store.Messages = m.msgs
		a.store.LastHydrate = time.Now().UnixMilli()
		return nil
	default:
		a.store.Sessions = m.list
		a.store.LastHydrate = time.Now().UnixMilli()
	}
	return nil
}

type sessionCreatedMsg struct {
	ses protocol.Session
	err error
}

// createSessionCmd creates a session with the server-side defaults (title
// "New session", model from the provider seam).
func (a *App) createSessionCmd() tea.Cmd {
	return func() tea.Msg {
		ses, err := a.CreateSession(context.Background(), "")
		return sessionCreatedMsg{ses: ses, err: err}
	}
}

func (a *App) applySessionCreated(m sessionCreatedMsg) tea.Cmd {
	if m.err != nil {
		a.lastErr = m.err.Error()
		return nil
	}
	a.putSessionFirst(m.ses)
	a.openSession(m.ses.ID)
	return a.hydrateCmd()
}

// putSessionFirst upserts s at the head of the home list (newest-first); a
// later SSE session.updated replaces it in place via store.Apply.
func (a *App) putSessionFirst(s protocol.Session) {
	for i := range a.store.Sessions {
		if a.store.Sessions[i].ID == s.ID {
			a.store.Sessions[i] = s
			return
		}
	}
	a.store.Sessions = append([]protocol.Session{s}, a.store.Sessions...)
}

func (a *App) openSession(id string) {
	a.route = routeSession
	a.cur = id
	a.home.buf = ""
}

var (
	escBinding = key.NewBinding(key.WithKeys("esc"))
	dlgYes     = key.NewBinding(key.WithKeys("y", "enter", "ctrl+c"))
	dlgNo      = key.NewBinding(key.WithKeys("n", "esc"))
)

// handleKey is the app key dispatcher: dialog > session route > home route.
func (a *App) handleKey(k tea.KeyPressMsg) []tea.Cmd {
	if d, ok := a.dlg.top(); ok {
		return a.handleDialogKey(d, k)
	}
	if a.route == routeSession {
		return a.handleSessionKey(k)
	}
	return a.handleHomeKey(k)
}

type dialogKind int

const (
	dlgQuit dialogKind = iota
	dlgHelp
)

type dialog struct{ kind dialogKind }

type dialogStack struct{ items []dialog }

func (s *dialogStack) push(d dialog) { s.items = append(s.items, d) }

func (s *dialogStack) pop() {
	if n := len(s.items); n > 0 {
		s.items = s.items[:n-1]
	}
}

func (s *dialogStack) top() (dialog, bool) {
	n := len(s.items)
	if n == 0 {
		return dialog{}, false
	}
	return s.items[n-1], true
}

func (s dialogStack) has() bool { return len(s.items) > 0 }

func (s dialogStack) view() string {
	d, ok := s.top()
	if !ok {
		return ""
	}
	switch d.kind {
	case dlgQuit:
		return title.Render("Quit yolo?") +
			"\n" + dim.Render("  y yes \u00B7 n/esc no")
	case dlgHelp:
		return title.Render("Help") +
			"\n" + dim.Render("  \u2191/\u2193 move \u00B7 enter open \u00B7 n new") +
			"\n" + dim.Render("  /help help \u00B7 ctrl+c quit \u00B7 esc back")
	}
	return ""
}

func (a *App) handleDialogKey(d dialog, k tea.KeyPressMsg) []tea.Cmd {
	if d.kind == dlgQuit {
		if key.Matches(k, dlgYes) {
			return a.emit(quitCmd())
		}
		if key.Matches(k, dlgNo) {
			a.dlg.pop()
		}
		return nil
	}
	a.dlg.pop() // dlgHelp: any key closes
	return nil
}

// quitCmd is a Cmd that tells the program to exit (bubbletea v2's Quit() is a
// Msg, so it is wrapped for use as a Cmd).
func quitCmd() tea.Cmd {
	return func() tea.Msg { return tea.Quit() }
}

// abortedMsg reports the result of the esc-while-busy abort.
type abortedMsg struct{ err error }

// abortCmd posts the server abort for the current session.
func (a *App) abortCmd() tea.Cmd {
	id := a.cur
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := a.Abort(ctx, id)
		return abortedMsg{err: err}
	}
}

// emit returns cmds unchanged; when record is set (tests) it also captures
// them in a.Cmds.
func (a *App) emit(cmds ...tea.Cmd) []tea.Cmd {
	if a.record {
		for _, c := range cmds {
			if c != nil {
				a.Cmds = append(a.Cmds, c)
			}
		}
	}
	return cmds
}

// View renders the active route, the dialog overlay and the last error line
// into a tea.View (bubbletea v2's Model interface returns tea.View, not
// string). The plain-string composition lives in a.view() for unit testing.
func (a *App) View() tea.View {
	return tea.NewView(a.view())
}

// view composes the on-screen string for the active route, dialogs and the
// last error line.
func (a *App) view() string {
	var b strings.Builder
	if a.route == routeSession {
		b.WriteString(a.viewSession())
	} else {
		b.WriteString(a.home.render(&a.store))
	}
	if v := a.dlg.view(); v != "" {
		b.WriteString("\n" + v)
	}
	if a.lastErr != "" {
		b.WriteString("\n" + errRed.Render("! "+a.lastErr))
	}
	return b.String()
}

// viewSession renders the session route: title, the transcript viewport and
// the locked help line.
func (a *App) viewSession() string {
	w := a.size.Width
	if w < 1 {
		w = 80
	}
	h := a.size.Height - 3
	if h < 1 {
		h = 1
	}
	a.sess.sync(&a.store, w, h)
	t := "session"
	if a.store.Current != nil {
		t = a.store.Current.Title
	}
	return title.Render(t) +
		"\n" + a.sess.vm.View() +
		"\n" + divider.Render(dividerLine()) +
		"\n" + dim.Render(sessionHelp)
}
