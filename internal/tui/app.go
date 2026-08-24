// Package tui is the bubbletea v2 frontend for yolo.
//
// The TUI is a pure client: it talks to the core server only through the wire
// contract (internal/protocol) via internal/tui/client. Non-test files import
// only internal/protocol, internal/tui/*, the standard library, and the charm
// deps.
package tui

import (
	"context"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// EventMsg carries one server SSE event. It is exported so the test harness
// can drive the app with it.
type EventMsg struct{ Event protocol.Event }

type route int

const (
	routeHome route = iota
	routeSession
)

// App is the root bubbletea model: routes, store, dialog stack and the SSE
// event pump.
type App struct {
	*client.Service
	store        store.State
	route        route
	curSessionID string
	home         homeModel
	sess         sessionModel
	prompt       promptModel
	dlg          dialogStack
	toasts       []toast
	toastSeq     int
	toastCmds    []tea.Cmd
	lastErr      string
	spinIdx      int // footer spinner frame
	// tea plumbing
	size      tea.WindowSizeMsg
	eventCh   chan protocol.Event
	resyncCh  chan struct{} // SSE drop pings from the client
	resyncing bool          // a transient SSE drop's re-hydrate is in flight
	stop      context.CancelFunc
	emitSink  func(cmds ...tea.Cmd) // test seam, set from _test.go only
}

// NewApp builds the root model. A non-empty startSessionID starts on that
// session (resume); empty starts at home. The prompt is always focused with a
// static (non-blinking) cursor.
func NewApp(c *client.Service, s store.State, startSessionID string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	eventCh, resyncCh := c.Events(ctx)
	a := &App{
		Service:  c,
		store:    s,
		route:    routeHome,
		home:     homeModel{now: nowMillis},
		sess:     newSessionModel(80, 21),
		size:     tea.WindowSizeMsg{Width: 80, Height: 24},
		eventCh:  eventCh,
		resyncCh: resyncCh,
		stop:     cancel,
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
		a.curSessionID = startSessionID
	}
	return a
}

// Close stops the SSE pump. Call it once the program exits.
func (a *App) Close() { a.stop() }

// Init hydrates the starting route and arms the SSE + resync pumps.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.hydrateCmd(), a.eventPump()}
	if c := a.resyncPump(); c != nil {
		cmds = append(cmds, c)
	}
	return tea.Batch(cmds...)
}

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
		a.store.Live = true
		a.store.Apply(m.Event)
		// Any applied event may have changed the transcript (message/part
		// family); re-render once instead of on every frame.
		a.sess.isDirty = true
		return a.afterApply(a.eventPump())
	case connLostMsg:
		a.store.Live = false
		return nil
	case resyncMsg:
		// The SSE stream dropped (the client is reconnecting): events
		// published in the gap are unrecoverable — re-hydrate the current
		// route over REST and re-arm the resync pump. The footer shows the
		// outage window until the re-hydrate completes (concurrency-4).
		a.resyncing = true
		a.sess.isDirty = true
		return tea.Batch(a.hydrateCmd(), a.resyncPump())
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
		if a.resyncing {
			a.resyncing = false
			a.store.Live = true
		}
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
	case tea.InterruptMsg:
		// SIGINT during Run: the same as the ctrl+c keystroke (cli-2) —
		// route it through the full key ladder so a pending permission
		// ask or an open dialog still owns the keys.
		cmds := a.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	return nil
}

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
		return EventMsg{Event: ev}
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
