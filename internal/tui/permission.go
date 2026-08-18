package tui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

var (
	permOnce   = key.NewBinding(key.WithKeys("1"))
	permAlways = key.NewBinding(key.WithKeys("2"))
	permReject = key.NewBinding(key.WithKeys("3", "esc"))
)

// permissionView renders the pending-ask overlay from the store (shown while
// Store.Pending is non-empty, above the prompt): the first parked ask only.
func (a *App) permissionView() string {
	if len(a.store.Pending) == 0 {
		return ""
	}
	p := a.store.Pending[0]
	lines := []string{
		title.Render("permission · " + p.Permission),
		dim.Render("  patterns: " + strings.Join(p.Patterns, ", ")),
	}
	if len(p.Always) > 0 {
		lines = append(lines, dim.Render("  Always: "+strings.Join(p.Always, ", ")))
	}
	if p.Tool != nil && p.Tool.CallID != "" {
		lines = append(lines, dim.Render("  tool call: "+p.Tool.MessageID+"/"+short6(p.Tool.CallID)))
	}
	lines = append(lines, dim.Render("  [1] once  [2] always  [3] reject"))
	return strings.Join(lines, "\n")
}

// short6 truncates an ID to its first 6 runes for the tool-call ref line.
func short6(s string) string {
	r := []rune(s)
	if len(r) <= 6 {
		return s
	}
	return string(r[:6])
}

// handlePermKey owns every key while an ask is pending: 1/2/3 reply
// once/always/reject, esc rejects (locked); everything else is ignored.
func (a *App) handlePermKey(k tea.KeyPressMsg) []tea.Cmd {
	switch {
	case key.Matches(k, permOnce):
		return a.emit(a.replyPermCmd("once")...)
	case key.Matches(k, permAlways):
		return a.emit(a.replyPermCmd("always")...)
	case key.Matches(k, permReject):
		return a.emit(a.replyPermCmd("reject")...)
	}
	return nil
}

// replyPermCmd posts the reply for the first parked ask.
func (a *App) replyPermCmd(reply string) []tea.Cmd {
	id := a.store.Pending[0].ID
	return []tea.Cmd{func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := a.ReplyPermission(ctx, id, reply)
		return permReplyMsg{id: id, reply: reply, err: err}
	}}
}

// permReplyMsg reports the result of a permission reply.
type permReplyMsg struct {
	id    string
	reply string
	err   error
}

// applyPermReply drops the answered ask on success (idempotent with the
// permission.replied event) or toasts and keeps the dialog on failure.
func (a *App) applyPermReply(m permReplyMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	kept := make([]protocol.PermissionAskedProps, 0, len(a.store.Pending))
	for _, p := range a.store.Pending {
		if p.ID != m.id {
			kept = append(kept, p)
		}
	}
	a.store.Pending = kept
	return nil
}
