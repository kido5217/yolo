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
	modelDlg  *modelDlg
	agentDlg  *agentDlg
	toasts    []toast
	toastSeq  int
	toastCmds []tea.Cmd
	lastErr   string
	spinIdx   int // footer spinner frame
	// tea plumbing
	size     tea.WindowSizeMsg
	eventCh  chan protocol.Event
	stop     context.CancelFunc
	emitSink func(cmds ...tea.Cmd) // test seam, set from _test.go only
}

// NewApp builds the root model. A non-empty startSessionID starts on that
// session (resume); empty starts at home. The prompt is always focused with a
// static (non-blinking) cursor.
func NewApp(c *client.Client, s store.Store, startSessionID string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		Client:  c,
		store:   s,
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
// Update dispatches a message, then drains the toast ticks armed during the
// update and merges them into the returned cmd (each toast owns its 4s
// auto-clear tick).
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.updateMsg(msg)
	if c := a.drainToastCmds(); c != nil {
		if cmd == nil {
			cmd = c
		} else {
			cmd = tea.Batch(cmd, c)
		}
	}
	return a, cmd
}

func (a *App) updateMsg(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.size = m
		if m.Width > 2 {
			a.prompt.input.SetWidth(m.Width - 2)
		}
		return nil
	case EventMsg:
		a.store.Conn = true
		a.store.Apply(m.Ev)
		// Any applied event may have changed the transcript (message/part
		// family); re-render once instead of on every frame.
		a.sess.dirty = true
		return a.afterApply(a.eventPump())
	case connLostMsg:
		a.store.Conn = false
		return nil
	case spinMsg:
		a.spinIdx++
		if a.statusSeg() != "" {
			return a.spinTick()
		}
		return nil
	case permReplyMsg:
		return a.applyPermReply(m)
	case HydrateMsg:
		return a.hydrateCmd()
	case hydratedMsg:
		return a.applyHydrate(m)
	case catalogMsg:
		return a.applyCatalog(m)
	case dlgPatchMsg:
		return a.applyDlgPatch(m)
	case sessionCreatedMsg:
		return a.applySessionCreated(m)
	case toastExpireMsg:
		a.removeToast(m.id)
		return nil
	case abortedMsg:
		if m.err != nil {
			a.lastErr = "abort: " + m.err.Error()
		}
		return nil
	case sendMsg:
		return a.applySend(m)
	case commandExecMsg:
		return a.applyCommandExec(m)
	case tea.KeyPressMsg:
		cmds := a.handleKey(m)
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	return nil
}

// connLostMsg signals the SSE channel closed (ctx done); the client already
// handles reconnects with its internal backoff loop.
type connLostMsg struct{}

// afterApply arms the footer spinner when a just-applied event left the
// session non-idle.
func (a *App) afterApply(cmd tea.Cmd) tea.Cmd {
	if a.statusSeg() == "" {
		return cmd
	}
	if cmd == nil {
		return a.spinTick()
	}
	return tea.Batch(cmd, a.spinTick())
}

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
	cfg      map[string]any
	err      error
	notFound bool
}

func (a *App) hydrateCmd() tea.Cmd {
	if a.route == routeSession && a.cur != "" {
		id := a.cur
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		list, err := a.ListSessions(ctx)
		cmds, _ := a.ListCommands(ctx)
		cfg, _ := a.GetConfig(ctx)
		return hydratedMsg{list: list, cmds: cmds, cfg: cfg, err: err}
	}
}

func (a *App) applyHydrate(m hydratedMsg) tea.Cmd {
	if m.cmds != nil {
		a.store.Commands = m.cmds
	}
	if m.cfg != nil {
		a.store.Config = m.cfg
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
		a.store.ForgetParts()
		a.sess.dirty = true
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ses, err := a.CreateSession(ctx, "")
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

// handleKey is the app key dispatcher: permission > dialog > model/agent
// openers > slash menu > route > prompt. A pending permission ask owns every
// key (1/2/3/esc only); while the slash menu is open it owns the keys; routes
// handle their navigation keys; everything else falls through to the
// always-focused prompt input.
func (a *App) handleKey(k tea.KeyPressMsg) []tea.Cmd {
	if len(a.store.Pending) > 0 {
		return a.handlePermKey(k)
	}
	if d, ok := a.dlg.top(); ok {
		return a.handleDialogKey(d, k)
	}
	switch {
	case key.Matches(k, dlgModelKey):
		return a.openModelDialog()
	case key.Matches(k, dlgAgentsKey):
		return a.openAgentDialog()
	case key.Matches(k, homeKeyMap.Quit):
		a.dlg.push(dialog{kind: dlgQuit})
		return nil
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
	case "/quit", "/exit": // /exit is the alias of /quit
		a.dlg.push(dialog{kind: dlgQuit})
	case "/model":
		return a.openModelDialog()
	case "/agents":
		return a.openAgentDialog()
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
		return title.Render("quit? [y/n]")
	case dlgHelp:
		return title.Render("Help") +
			"\n" + dim.Render("  | Key | Action |") +
			"\n" + dim.Render("  |---|---|") +
			"\n" + dim.Render("  | enter | send prompt |") +
			"\n" + dim.Render("  | esc | abort turn (busy) / close dialog |") +
			"\n" + dim.Render("  | ctrl+c | quit (confirm) |") +
			"\n" + dim.Render("  | ctrl+p | model dialog |") +
			"\n" + dim.Render("  | ctrl+a | agent dialog |") +
			"\n" + dim.Render("  | / | command menu |") +
			"\n" + dim.Render("  | pgup/pgdn | viewport scroll |") +
			"\n" + dim.Render("  | 1/2/3 | permission reply |") +
			"\n" + dim.Render("  | alt+e / alt+t | expand tool part / toggle reasoning |") +
			"\n" + dim.Render("  pgup/pgdn scroll \u00B7 \\+enter newline")
	}
	return ""
}

// dlgView renders the top dialog: the model/agent pickers carry their state
// on the app, the rest render from the stack alone.
func (a *App) dlgView() string {
	switch d, ok := a.dlg.top(); {
	case !ok:
		return ""
	case d.kind == dlgModel && a.modelDlg != nil:
		return a.modelDlg.view(&a.store)
	case d.kind == dlgAgents && a.agentDlg != nil:
		return a.agentDlg.view(&a.store)
	}
	return a.dlg.view()
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
	switch d.kind {
	case dlgModel:
		if a.modelDlg == nil {
			a.dlg.pop()
			return nil
		}
		return a.modelDlg.handleKey(a, k)
	case dlgAgents:
		if a.agentDlg == nil {
			a.dlg.pop()
			return nil
		}
		return a.agentDlg.handleKey(a, k)
	}
	a.dlg.pop() // dlgHelp: any key closes
	return nil
}

// catalogMsg reports the provider + agent catalog fetched when the model or
// agent dialog opens.
type catalogMsg struct {
	provs  []protocol.Provider
	agents []protocol.Agent
	err    error
}

// fetchCatalogCmd lists providers and agents in one cmd.
func (a *App) fetchCatalogCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		provs, perr := a.ListProviders(ctx)
		agents, aerr := a.ListAgents(ctx)
		var err error
		if perr != nil {
			err = perr
		} else if aerr != nil {
			err = aerr
		}
		return catalogMsg{provs: provs, agents: agents, err: err}
	}
}

func (a *App) applyCatalog(m catalogMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	if m.provs != nil {
		a.store.Providers = m.provs
	}
	if m.agents != nil {
		a.store.Agents = m.agents
	}
	a.syncModelSel()
	a.syncAgentSel()
	return nil
}

// dlgPatchMsg reports the result of a model/agent dialog apply; sess is set
// for the "this session" scope, cfg for "set default".
type dlgPatchMsg struct {
	field string // "model" | "agent"
	value string
	sess  *protocol.Session
	cfg   map[string]any
	err   error
}

// patchDlgCmd patches the session or config with the chosen value.
func (a *App) patchDlgCmd(field, value string, thisSession bool) tea.Cmd {
	if thisSession {
		id := a.cur
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ses, err := a.PatchSession(ctx, id, map[string]any{field: value})
			return dlgPatchMsg{field: field, value: value, sess: &ses, err: err}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cfg, err := a.PatchConfig(ctx, map[string]any{field: value})
		return dlgPatchMsg{field: field, value: value, cfg: cfg, err: err}
	}
}

// applyDlgPatch lands a successful apply (store + toast + close); an error
// toasts and keeps the dialog open.
func (a *App) applyDlgPatch(m dlgPatchMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	if m.sess != nil {
		cp := *m.sess
		a.store.Current = &cp
		a.putSessionFirst(cp)
	}
	if m.cfg != nil {
		a.store.Config = m.cfg
	}
	switch m.field {
	case "agent":
		a.toast("agent set: " + m.value)
		a.closeAgentDialog()
	default:
		a.toast("model set: " + m.value)
		a.closeModelDialog()
	}
	return nil
}

// quitCmd is a Cmd that tells the program to exit.
func quitCmd() tea.Cmd {
	return tea.Quit
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

// emit returns cmds unchanged; when a test sink is installed (emitSink), it
// also captures the non-nil cmds there.
func (a *App) emit(cmds ...tea.Cmd) []tea.Cmd {
	if a.emitSink != nil {
		nonNil := make([]tea.Cmd, 0, len(cmds))
		for _, c := range cmds {
			if c != nil {
				nonNil = append(nonNil, c)
			}
		}
		if len(nonNil) > 0 {
			a.emitSink(nonNil...)
		}
	}
	return cmds
}

// View renders the active route, the dialog overlay and the last error line
// into a tea.View (bubbletea v2's Model interface returns tea.View, not
// string). The plain-string composition lives in a.view() for unit testing.
// AltScreen keeps the TUI in the alternate screen buffer (v2 expresses this
// on the View, not as a program option).
func (a *App) View() tea.View {
	v := tea.NewView(a.view())
	v.AltScreen = true
	return v
}

// overlayLines counts the lines the below-viewport overlays (slash menu is
// reserved separately by viewSession) occupy: the permission ask, the
// toasts, the open dialog and the last error line. The viewport uses this
// to shrink so the composed frame always fits the terminal height —
// mandatory under the alt screen, whose frame (unlike the normal-screen
// frame, which grows with content) is the fixed terminal size.
func (a *App) overlayLines() int {
	n := 0
	for _, v := range []string{a.permissionView(), a.toastsView(), a.dlgView()} {
		if v != "" {
			n += 1 + strings.Count(v, "\n")
		}
	}
	if a.lastErr != "" {
		n++
	}
	return n
}

// view composes the on-screen string: the active route, the slash menu, the
// permission overlay above the prompt, the prompt line, toasts, the dialog
// overlay, the last error line and the status footer (both routes).
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
	if v := a.permissionView(); v != "" {
		b.WriteString("\n" + v)
	}
	b.WriteString("\n" + a.prompt.view())
	if v := a.toastsView(); v != "" {
		b.WriteString("\n" + v)
	}
	if v := a.dlgView(); v != "" {
		b.WriteString("\n" + v)
	}
	if a.lastErr != "" {
		b.WriteString("\n" + errRed.Render("! "+a.lastErr))
	}
	b.WriteString("\n" + a.footerView())
	return b.String()
}

// viewSession renders the session route: title, the transcript viewport and
// the locked help line. The viewport reserves a line for the prompt, one for
// the footer, the open slash menu and every below-viewport overlay (see
// overlayLines), so the frame fits the terminal height.
func (a *App) viewSession() string {
	w := a.size.Width
	if w < 1 {
		w = 80
	}
	h := a.size.Height - 3 - 1 - 1 - a.prompt.menuLines(a.store.Commands) - a.overlayLines()
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
