package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

var (
	permOnce   = key.NewBinding(key.WithKeys("1"))
	permAlways = key.NewBinding(key.WithKeys("2"))
	permReject = key.NewBinding(key.WithKeys("3", "esc"))
	permLeft   = key.NewBinding(key.WithKeys("left"))
	permRight  = key.NewBinding(key.WithKeys("right"))
)

// permReplies maps the pill index to the wire reply.
var permReplies = []string{"once", "always", "reject"}

// permDlg is the permission dialog payload (S2.8: the parked ask on the
// modal stack; sel = the highlighted pill).
type permDlg struct {
	sel int
}

// moveSel steps the pill with wraparound (upstream's pill navigation).
func (p *permDlg) moveSel(d int) {
	p.sel = ((p.sel+d)%len(permReplies) + len(permReplies)) % len(permReplies)
}

// permReplyFor maps a key to the reply mode (the test seam + the handler's
// core): 1/2/3 reply directly (yolo pin), esc = reject (yolo pin), enter
// replies with the selected pill.
func permReplyFor(k tea.KeyPressMsg, sel int) (string, bool) {
	switch {
	case key.Matches(k, permOnce):
		return "once", true
	case key.Matches(k, permAlways):
		return "always", true
	case key.Matches(k, permReject):
		return "reject", true
	case key.Matches(k, homeKeyMap.Enter):
		return permReplies[sel], true
	}
	return "", false
}

// handleKey drives the dialog: the reply keys, the pill navigation, and
// everything else ignored (the modal stack owns esc/ctrl+c too — the
// stack's esc path and the permReject binding both reject; the reply
// wins the key ladder because the permission branch precedes the dialog).
func (p *permDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if reply, ok := permReplyFor(k, p.sel); ok {
		return a.emit(a.replyPermCmd(reply)...)
	}
	switch {
	case key.Matches(k, permLeft):
		p.moveSel(-1)
	case key.Matches(k, permRight):
		p.moveSel(1)
	}
	return nil
}

// permInfo is the port of the upstream info() (permission.tsx:195-380) over
// the store part's input map (deviation 179: the wire carries no request
// Meta; the part input is the data source; the EditBody diff view is
// dropped). The input keys follow the tool schemas (camelCase: filePath,
// pattern, command, url, query, path).
func permInfo(p protocol.PermissionAskedProps, input map[string]any) (icon, title, body string) {
	str := func(k string) string {
		if v, ok := input[k].(string); ok {
			return v
		}
		return ""
	}
	switch p.Permission {
	case "edit":
		return "→", "Edit " + str("filePath"), ""
	case "read":
		fp := str("filePath")
		if fp != "" {
			body = "Path: " + fp
		}
		return "→", "Read " + fp, body
	case "glob":
		pat := str("pattern")
		if pat != "" {
			body = "Pattern: " + pat
		}
		return "✱", fmt.Sprintf("Glob %q", pat), body
	case "grep":
		pat := str("pattern")
		if pat != "" {
			body = "Pattern: " + pat
		}
		return "✱", fmt.Sprintf("Grep %q", pat), body
	case "list":
		dir := str("path")
		if dir != "" {
			body = "Path: " + dir
		}
		return "→", "List " + dir, body
	case "bash":
		cmd := str("command")
		if cmd != "" {
			body = "$ " + cmd
		}
		return "#", "Shell command", body
	case "task":
		typ := str("subagent_type")
		if typ == "" {
			typ = "Unknown"
		}
		desc := str("description")
		if desc != "" {
			body = "◉ " + desc
		}
		return "#", titlecase(typ) + " Task", body
	case "webfetch":
		url := str("url")
		if url != "" {
			body = "URL: " + url
		}
		return "%", "WebFetch " + url, body
	case "websearch":
		query := str("query")
		if query != "" {
			body = "Query: " + query
		}
		return "◈", fmt.Sprintf("Search %q", query), body
	case "doom_loop":
		return "⟳", "Continue after repeated failures", "This keeps the session running despite repeated failures."
	default:
		return "⚙", "Call tool " + p.Permission, "Tool: " + p.Permission
	}
}

// partInput returns the parked ask's tool-part input map (the info() data
// source; nil when the part is not hydrated yet).
func partInput(st *store.State, p protocol.PermissionAskedProps) map[string]any {
	if p.Tool == nil {
		return nil
	}
	for _, m := range st.Messages {
		for _, pr := range m.Parts {
			if pr.CallID == p.Tool.CallID && (p.Tool.MessageID == "" || pr.MessageID == p.Tool.MessageID) && pr.State != nil {
				return pr.State.Input
			}
		}
	}
	return nil
}

// view renders the dialog content (the modal stack draws the frame —
// S2.2): the warning header, the info() icon+title (one row) + body, the
// patterns and Always lines, the reply pills (the selected pill painted in
// the warning token — deviation 182's yolo pin; unselected pills muted).
// `rows` are pre-styled lines; each wraps at w (patterns carry full
// command strings — the long line is styled as a whole then wrapped, same
// contract as the old permissionView). The body line sits at 4 columns
// (the upstream body box is paddingLeft 1 relative to the icon column).
func (p *permDlg) view(st *store.State, w int, th theme.Theme) string {
	if len(st.Pending) == 0 {
		return ""
	}
	ask := st.Pending[0]
	icon, title, body := permInfo(ask, partInput(st, ask))
	muted := th.TextMuted()
	rows := []string{
		th.Warning().Render("△ ") + th.Text().Render("Permission required"),
		"  " + muted.Render(icon) + th.Text().Render(" "+title),
	}
	if body != "" {
		rows = append(rows, "    "+muted.Render(body))
	}
	if len(ask.Patterns) > 0 {
		rows = append(rows, muted.Render("  patterns: "+strings.Join(ask.Patterns, ", ")))
	}
	if len(ask.Always) > 0 {
		rows = append(rows, muted.Render("  Always: "+strings.Join(ask.Always, ", ")))
	}
	rows = append(rows, p.pills(th))
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		for j, l := range strings.Split(wrapLine(r, w), "\n") {
			if j > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(l)
		}
	}
	return b.String()
}

// pills renders the reply pill row: the selected pill painted in the
// warning token (warning bg + the SelectedForeground-on-warning fg, bold),
// the unselected pills muted (upstream: the selected pill is accent — yolo
// pins warning; deviation 182's look note).
func (p *permDlg) pills(th theme.Theme) string {
	labels := []string{"Allow once", "Allow always", "Reject"}
	parts := make([]string, len(labels))
	for i, l := range labels {
		if i == p.sel {
			bg, ok := th.Color("warning")
			if !ok {
				parts[i] = cursorStyle(th).Render(l)
				continue
			}
			fg := lipgloss.Color(th.SelectedForeground(bg).Hex()[:7])
			parts[i] = lipgloss.NewStyle().Foreground(fg).
				Background(lipgloss.Color(bg.Hex()[:7])).Bold(true).
				Padding(0, 1).Render(l)
		} else {
			parts[i] = th.TextMuted().Render(l)
		}
	}
	return strings.Join(parts, "  ")
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
	a.syncPermDialog()
	return nil
}

// permissionView renders the top permission dialog's content (S2.8; a
// fresh selection when no perm dialog is on the stack — the unit tests).
func (a *App) permissionView(w int) string {
	p := &permDlg{}
	if d, ok := a.dlg.top(); ok && d.kind == dlgPerm && d.perm != nil {
		p = d.perm
	}
	return p.view(&a.store, w, a.theme)
}
