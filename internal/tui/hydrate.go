package tui

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
)

// HydrateMsg asks the app to re-hydrate its current route over REST. It is
// exported so the test harness can drive the app with it.
type HydrateMsg struct{}

// connLostMsg signals the SSE channel closed (ctx done); the client already
// handles reconnects with its internal backoff loop.
type connLostMsg struct{}

// resyncMsg signals one dropped /event connection (the client is
// reconnecting with backoff); the app re-hydrates its current route to
// recover the events published in the gap.
type resyncMsg struct{}

// resyncPump blocks on the resync channel and delivers resyncMsg per ping;
// the resyncMsg case re-arms it. A closed channel (ctx done) ends the pump
// quietly — connLostMsg arrives via the event channel at the same time.
func (a *App) resyncPump() tea.Cmd {
	ch := a.resyncCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		_, ok := <-ch
		if !ok {
			return nil
		}
		return resyncMsg{}
	}
}

// hydratedMsg payloads are delivered by the hydrate cmd: home lists, session
// details, or the resume not-found case. Fetch failures that don't invalidate
// the payload (ListMessages, ListCommands) degrade the corresponding slice.
type hydratedMsg struct {
	sessID   string
	list     []protocol.Session
	sess     *protocol.Session
	msgs     []protocol.MessageWithParts
	cmds     []protocol.Command
	cfg      map[string]any
	err      error
	notFound bool
}

func (a *App) hydrateCmd() tea.Cmd {
	if a.route == routeSession && a.curSessionID != "" {
		id := a.curSessionID
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ses, err := a.GetSession(ctx, id)
			if errors.Is(err, client.ErrNotFound) {
				return hydratedMsg{sessID: id, notFound: true}
			}
			if err != nil {
				return hydratedMsg{sessID: id, err: err}
			}
			cmds, _ := a.ListCommands(ctx)
			msgs, merr := a.ListMessages(ctx, id)
			if merr != nil {
				return hydratedMsg{sessID: id, sess: &ses, cmds: cmds, err: merr}
			}
			return hydratedMsg{sessID: id, sess: &ses, msgs: msgs, cmds: cmds}
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
		a.lastErr = "session not found: " + m.sessID
		a.route = routeHome
		a.curSessionID = ""
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
		a.sess.isDirty = true
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
	a.curSessionID = id
}
