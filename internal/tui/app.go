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
	"charm.land/bubbles/v2/textinput"
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

// toast is a transient one-shot line (T25 lands the busy-send and command
// error toasts; T28 replaces this with the proper stack and 4s auto-clear).
type toast struct{ msg string }

// maxToasts caps the visible toast stack (matches the T28 locked queue ≤3;
// T28 refines timing and dismiss).
const maxToasts = 3

// toast records a transient message; the newest stays within the cap.
func (a *App) toast(msg string) {
	a.toasts = append(a.toasts, toast{msg: msg})
	if len(a.toasts) > maxToasts {
		a.toasts = a.toasts[len(a.toasts)-maxToasts:]
	}
}

func (a *App) toastsView() string {
	if len(a.toasts) == 0 {
		return ""
	}
	var b strings.Builder
	for i, t := range a.toasts {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(errRed.Render("\u2022 " + t.msg))
	}
	return b.String()
}

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
// session (resume); empty starts at home. The prompt is always focused with a
// static (non-blinking) cursor.
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
	in := textinput.New()
	in.SetWidth(78)
	st := in.Styles()
	st.Cursor.Blink = false
	in.SetStyles(st)
	in.Focus()
	a.prompt.input = in
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
		if m.Width > 2 {
			a.prompt.input.SetWidth(m.Width - 2)
		}
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
	case sendMsg:
		return a, a.applySend(m)
	case commandExecMsg:
		return a, a.applyCommandExec(m)
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

// hydratedMsg payloads are delivered by the hydrate cmd: home lists, session
// details, or the resume not-found case. Fetch failures that don't invalidate
// the payload (ListMessages, ListCommands) degrade the corresponding slice.
type hydratedMsg struct {
	id       string
	list     []protocol.Session
	sess     *protocol.Session
	msgs     []protocol.MessageWithParts
	cmds     []protocol.Command
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
			cmds, _ := a.ListCommands(ctx)
			msgs, merr := a.ListMessages(ctx, id)
			if merr != nil {
				return hydratedMsg{id: id, sess: &ses, cmds: cmds, err: merr}
			}
			return hydratedMsg{id: id, sess: &ses, msgs: msgs, cmds: cmds}
		}
	}
	return func() tea.Msg {
		list, err := a.ListSessions(context.Background())
		cmds, _ := a.ListCommands(context.Background())
		return hydratedMsg{list: list, cmds: cmds, err: err}
	}
}

func (a *App) applyHydrate(m hydratedMsg) tea.Cmd {
	if m.cmds != nil {
		a.store.Commands = m.cmds
	}
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
}

var (
	escBinding = key.NewBinding(key.WithKeys("esc"))
	dlgYes     = key.NewBinding(key.WithKeys("y", "enter", "ctrl+c"))
	dlgNo      = key.NewBinding(key.WithKeys("n", "esc"))
)

// handleKey is the app key dispatcher: dialog > slash menu > route > prompt.
// While the menu is open it owns the keys; routes handle their navigation
// keys; everything else falls through to the always-focused prompt input.
func (a *App) handleKey(k tea.KeyPressMsg) []tea.Cmd {
	if d, ok := a.dlg.top(); ok {
		return a.handleDialogKey(d, k)
	}
	if a.prompt.slashActive() {
		return a.handleMenuKey(k)
	}
	switch a.route {
	case routeSession:
		if cmds, done := a.handleSessionKey(k); done {
			return cmds
		}
	default:
		if cmds, done := a.handleHomeKey(k); done {
			return cmds
		}
	}
	return a.handlePromptKey(k)
}

// handleMenuKey dispatches keys while the slash menu is open: arrows move
// the selection with wraparound, enter executes the selection (or clears the
// input on no match), esc closes the menu; everything else keeps filtering
// through the live input.
func (a *App) handleMenuKey(k tea.KeyPressMsg) []tea.Cmd {
	items := a.prompt.menuItems(a.store.Commands)
	switch {
	case key.Matches(k, homeKeyMap.Up):
		a.prompt.moveMenuSel(len(items), -1)
		return nil
	case key.Matches(k, homeKeyMap.Down):
		a.prompt.moveMenuSel(len(items), 1)
		return nil
	case key.Matches(k, promptEnter):
		if len(items) > 0 && a.prompt.sel < len(items) {
			return a.runCommand(items[a.prompt.sel].Name)
		}
		a.prompt.input.SetValue("")
		return nil
	case key.Matches(k, escBinding):
		a.prompt.input.SetValue("")
		return nil
	}
	return a.inputUpdate(k)
}

// handlePromptKey is the prompt fallback: enter sends (or soft-enters a
// trailing backslash), everything else feeds the input.
func (a *App) handlePromptKey(k tea.KeyPressMsg) []tea.Cmd {
	if key.Matches(k, promptEnter) {
		return a.promptEnter()
	}
	return a.inputUpdate(k)
}

// inputUpdate feeds a key to the prompt input and collects any emitted cmds.
func (a *App) inputUpdate(k tea.KeyPressMsg) []tea.Cmd {
	var cmds []tea.Cmd
	var c tea.Cmd
	a.prompt.input, c = a.prompt.input.Update(k)
	if c != nil {
		cmds = append(cmds, c)
	}
	return cmds
}

// promptEnter implements the LOCKED send semantics: a trailing backslash
// soft-enters a draft line; empty input is ignored; a busy store toasts;
// otherwise draft+line is sent and the input clears only on success.
func (a *App) promptEnter() []tea.Cmd {
	val := a.prompt.input.Value()
	if strings.HasSuffix(val, "\\") {
		a.prompt.draft += strings.TrimSuffix(val, "\\") + "\n"
		a.prompt.input.SetValue("")
		return nil
	}
	text := a.prompt.draft + strings.TrimSpace(val)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if sessionBusy(&a.store) {
		a.toast(busyToast)
		return nil
	}
	// The line stays until the success msg lands (applySend clears it), so a
	// server-side busy error leaves it for retry.
	return a.emit(a.sendMessageCmd(text))
}

// sendMessageCmd posts the composed line as a user message for the current
// session.
func (a *App) sendMessageCmd(text string) tea.Cmd {
	id := a.cur
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := a.SendMessage(ctx, id, text)
		return sendMsg{err: err}
	}
}

// sendMsg reports the result of a prompt send. On success the input clears;
// on error the line is kept for retry.
type sendMsg struct{ err error }

func (a *App) applySend(m sendMsg) tea.Cmd {
	if m.err != nil {
		if errors.Is(m.err, client.ErrBusy) {
			a.toast(busyToast)
		} else {
			a.lastErr = m.err.Error()
		}
		return nil
	}
	a.prompt.input.SetValue("")
	a.prompt.draft = ""
	return nil
}

// runCommand executes a slash command from the menu. /new without a current
// session issues CreateSession directly (LOCKED: the command endpoint needs a
// session id); other commands open their dialogs.
func (a *App) runCommand(name string) []tea.Cmd {
	a.prompt.input.SetValue("")
	switch name {
	case "/help":
		a.dlg.push(dialog{kind: dlgHelp})
	case "/exit":
		a.dlg.push(dialog{kind: dlgQuit})
	case "/model":
		a.dlg.push(dialog{kind: dlgModel})
	case "/agents":
		a.dlg.push(dialog{kind: dlgAgents})
	case "/new":
		if a.cur == "" {
			return a.emit(a.createSessionCmd())
		}
		return a.emit(a.commandCmd("/new"))
	}
	return nil
}

// commandCmd posts a slash command to the server for the current session.
func (a *App) commandCmd(cmd string) tea.Cmd {
	id := a.cur
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := a.Command(ctx, id, cmd)
		return commandExecMsg{resp: resp, err: err}
	}
}

// commandExecMsg reports the result of POST /session/{id}/command; a response
// carrying a session_id (server-side /new) switches to it.
type commandExecMsg struct {
	resp protocol.CommandResponse
	err  error
}

func (a *App) applyCommandExec(m commandExecMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	if m.resp.SessionID != "" {
		a.openSession(m.resp.SessionID)
		return a.emit(a.hydrateCmd())[0]
	}
	return nil
}

type dialogKind int

const (
	dlgQuit dialogKind = iota
	dlgHelp
	// dlgModel and dlgAgents are T25 placeholders opened by the slash menu;
	// the real pickers land in Task 27.
	dlgModel
	dlgAgents
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
			"\n" + dim.Render("  /help help \u00B7 ctrl+c quit \u00B7 esc back") +
			"\n" + dim.Render("  / commands: /new /model /agents /help /exit") +
			"\n" + dim.Render("  \\+enter newline in the prompt")
	case dlgModel:
		return title.Render("Model") +
			"\n" + dim.Render("  select a model")
	case dlgAgents:
		return title.Render("Agents") +
			"\n" + dim.Render("  select an agent")
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

// view composes the on-screen string: the active route, the slash menu and
// prompt lines, toasts, the dialog overlay and the last error line.
func (a *App) view() string {
	var b strings.Builder
	if a.route == routeSession {
		b.WriteString(a.viewSession())
	} else {
		b.WriteString(a.home.render(&a.store))
	}
	if v := a.prompt.menuView(a.store.Commands); v != "" {
		b.WriteString("\n" + v)
	}
	b.WriteString("\n" + a.prompt.view())
	if v := a.toastsView(); v != "" {
		b.WriteString("\n" + v)
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
// the locked help line. The viewport reserves a line for the prompt plus the
// open slash menu.
func (a *App) viewSession() string {
	w := a.size.Width
	if w < 1 {
		w = 80
	}
	h := a.size.Height - 3 - 1 - a.prompt.menuLines(a.store.Commands)
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
