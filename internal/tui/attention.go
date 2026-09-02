package tui

import (
	"encoding/json"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

// attentionState is the S5.6 terminal-bell state (deviation 227): the ported
// notifications.ts conditions, current-session-scoped. active = the current
// session went busy/retry since its last idle; errored = a turn error has
// surfaced since the busy; lastPermID dedupes the permission-ask bell by ask
// id.
type attentionState struct {
	active     bool
	errored    bool
	lastPermID string
}

// bell is the terminal bell. tea.Raw is the alt-screen-independent execute
// path (tea.Printf is inert on the alt screen).
func bell() tea.Cmd {
	return tea.Raw("\a")
}

// onAttention is the S5.6 attention hook (deviation 227): it runs in the
// EventMsg case after store.Apply and returns the bell cmd (batched into the
// applied event's cmd) or nil. The ported conditions, current-session-scoped:
// a permission.asked rings (deduped by the ask id); session.status busy|retry
// sets active and clears errored (no bell); session.status idle rings ONLY
// when active and not errored (the done bell); a message.updated carrying a
// non-nil Message.Error sets errored and rings (the turn-error bell). The
// upstream question.asked + SSE-timeout conditions are dropped (no yolo
// referent — deviation 227).
func (a *App) onAttention(ev protocol.Event) tea.Cmd {
	switch ev.Type {
	case protocol.EventTypePermissionAsked:
		var p protocol.PermissionAskedProps
		if json.Unmarshal(ev.Properties, &p) != nil {
			return nil
		}
		if p.SessionID != a.curSessionID || p.ID == a.attention.lastPermID {
			return nil
		}
		a.attention.lastPermID = p.ID
		return bell()
	case protocol.EventTypeSessionStatus:
		var p protocol.SessionStatusProps
		if json.Unmarshal(ev.Properties, &p) != nil {
			return nil
		}
		if p.SessionID != a.curSessionID {
			return nil
		}
		switch p.Status.Type {
		case protocol.SessionStatusBusy, protocol.SessionStatusRetry:
			a.attention.active = true
			a.attention.errored = false
			return nil
		case protocol.SessionStatusIdle:
			if a.attention.active && !a.attention.errored {
				return bell()
			}
			a.attention.active = false
			return nil
		}
	case protocol.EventTypeMessageUpdated:
		var p protocol.MessageUpdatedProps
		if json.Unmarshal(ev.Properties, &p) != nil {
			return nil
		}
		if p.SessionID != a.curSessionID {
			return nil
		}
		if p.Info.Error != nil {
			a.attention.errored = true
			return bell()
		}
	}
	return nil
}
