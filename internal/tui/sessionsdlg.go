// sessionsdlg.go — the /sessions picker (S3.1): the select over
// store.Sessions (updated-desc), the client-side title filter (deviation
// 190), the one-shot status snapshot gutter and the two-step delete.

package tui

import (
	"context"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// sessionDeleteArmTitle is the armed-row title (the S3.1 two-step confirm).
const sessionDeleteArmTitle = "Press ctrl+d again to confirm"

// sessionDeleteKey is the delete action binding (the upstream session_delete
// default).
var sessionDeleteKey = key.NewBinding(key.WithKeys("ctrl+d"))

// sessionRenameKey is the rename action binding (the upstream session_rename
// default, S3.2).
var sessionRenameKey = key.NewBinding(key.WithKeys("ctrl+r"))

// sessionsDlg is the session picker payload: the select + the status
// snapshot + the armed delete id. th is the theme at open (the pinned view
// takes no theme arg — deviation 197).
type sessionsDlg struct {
	sel      *selectModel
	th       theme.Theme
	status   map[string]string
	toDelete string
}

// handleKey drives the select (the modal stack consumes esc/ctrl+c first).
func (d *sessionsDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if d.sel == nil {
		return nil
	}
	return d.sel.handleKey(a, k)
}

// view renders the select (the modal stack draws the panel chrome).
func (d *sessionsDlg) view(w, h int) string {
	if d.sel == nil {
		return title.Render("Sessions") + "\n" + d.th.TextMuted().Render("  loading…")
	}
	return d.sel.view(w, h, d.th)
}

// sessionCategory is the date bucket: "Today" when the updated date is
// today (the local day), else the JS toDateString port.
func sessionCategory(updated, now time.Time) string {
	if updated.Format("2006-01-02") == now.Format("2006-01-02") {
		return "Today"
	}
	return updated.Format("Mon Jan 2 2006")
}

// sessionOptions is the session-list option list: store.Sessions in
// updated-desc (upstream orderByRecency), title = the title, description =
// the directory (the row tail), value = the id, category = the date bucket,
// gutter = the spinner frame when the snapshot is busy/retry.
func sessionOptions(a *App, status map[string]string) []selectOption {
	sessions := make([]protocol.Session, len(a.store.Sessions))
	copy(sessions, a.store.Sessions)
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].Time.Updated > sessions[j].Time.Updated
	})
	now := time.Now()
	opts := make([]selectOption, 0, len(sessions))
	for _, se := range sessions {
		gutter := ""
		switch status[se.ID] {
		case protocol.SessionStatusBusy, protocol.SessionStatusRetry:
			gutter = a.spinFrame() + " "
		}
		opts = append(opts, selectOption{
			title:       se.Title,
			description: se.Directory,
			category:    sessionCategory(time.UnixMilli(se.Time.Updated), now),
			value:       se.ID,
			gutter:      gutter,
		})
	}
	return opts
}

// sessionFilteredOptions is the client-side title filter (deviation 190 —
// the yolo wire has no search endpoint): the needle is a substring of the
// lowercased title; an empty needle returns the full list.
func sessionFilteredOptions(a *App, status map[string]string, needle string) []selectOption {
	opts := sessionOptions(a, status)
	if needle == "" {
		return opts
	}
	kept := make([]selectOption, 0, len(opts))
	for _, o := range opts {
		if strings.Contains(strings.ToLower(o.title), needle) {
			kept = append(kept, o)
		}
	}
	return kept
}

// rebuildSessionOptions (re)builds the dialog's option list on the live
// store (keeping the typed filter) and re-applies the armed toDelete row
// (the title + the error bg).
func (a *App) rebuildSessionOptions(d *sessionsDlg) {
	opts := sessionFilteredOptions(a, d.status, d.sel.input.Value())
	if d.toDelete != "" {
		for i := range opts {
			if v, _ := opts[i].value.(string); v == d.toDelete {
				opts[i].title = sessionDeleteArmTitle
				opts[i].bg = "error"
			}
		}
	}
	d.sel.options = opts
}

// openSessionListDialog pushes the /sessions picker: the select over
// store.Sessions (updated-desc) with the current session pre-selected, the
// client-side title filter (skipFilter + onFilter), the two-step delete
// (ctrl+d) and the one-shot status snapshot gutter.
func (a *App) openSessionListDialog() []tea.Cmd {
	d := &sessionsDlg{th: a.theme, status: map[string]string{}}
	sel := selectNew("Sessions", "Search", sessionOptions(a, d.status),
		func(o selectOption) bool {
			v, _ := o.value.(string)
			return v == a.curSessionID
		},
		func(app *App, o selectOption) {
			id, _ := o.value.(string)
			if id == "" {
				return
			}
			app.closeTopModal()
			app.openSession(id)
			app.emit(app.hydrateCmd())
		},
		func(selectOption) {
			if d.toDelete == "" {
				return
			}
			d.toDelete = ""
			a.rebuildSessionOptions(d)
		})
	d.sel = sel
	sel.skipFilter = true
	sel.onFilter = func(needle string) {
		sel.options = sessionFilteredOptions(a, d.status, needle)
	}
	sel.WithActions([]selectAction{{
		key:   sessionDeleteKey,
		title: "delete",
		run: func(app *App) {
			l := sel.filtered()
			if sel.sel >= len(l) {
				return
			}
			id, _ := l[sel.sel].value.(string)
			if id == "" {
				return
			}
			if d.toDelete == id {
				app.emit(app.sessionDeleteCmd(id))
				return
			}
			d.toDelete = id
			app.rebuildSessionOptions(d)
		},
	}, {
		// S3.2: the rename action (the upstream action label "rename") —
		// closes the list and opens the rename form for the selected row.
		key:   sessionRenameKey,
		title: "rename",
		run: func(app *App) {
			l := sel.filtered()
			if sel.sel >= len(l) {
				return
			}
			id, _ := l[sel.sel].value.(string)
			if id == "" {
				return
			}
			app.closeTopModal()
			app.openSessionRenameDialog(id)
		},
	}})
	for i, o := range sel.options {
		if v, _ := o.value.(string); v == a.curSessionID {
			sel.sel = i
			break
		}
	}
	a.pushModal(dialog{kind: dlgSessions, sessions: d}, dlgMedium, nil)
	return a.emit(a.fetchSessionStatusCmd())
}

// syncSessionSel re-anchors the open session picker at the selected session
// id across the store events that mutate Sessions (session.updated/deleted —
// the EventMsg hook, preserveSelection); the id gone clamps to the last row.
func (a *App) syncSessionSel() {
	d := a.dlg.sessions()
	if d == nil || d.sel == nil {
		return
	}
	id := ""
	if l := d.sel.filtered(); d.sel.sel >= 0 && d.sel.sel < len(l) {
		id, _ = l[d.sel.sel].value.(string)
	}
	a.rebuildSessionOptions(d)
	l := d.sel.filtered()
	if len(l) == 0 {
		d.sel.sel = 0
		return
	}
	for i, o := range l {
		if v, _ := o.value.(string); v == id {
			d.sel.sel = i
			return
		}
	}
	d.sel.sel = len(l) - 1
}

// fetchSessionStatusCmd fetches the per-session status snapshot at open
// (deviation 190: store.Status is current-session-only).
func (a *App) fetchSessionStatusCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		st, err := a.Status(ctx)
		return statusSnapshotMsg{status: st, err: err}
	}
}

// statusSnapshotMsg reports the GET /session/status snapshot.
type statusSnapshotMsg struct {
	status map[string]string
	err    error
}

// applySessionStatusSnapshot stores the snapshot and rebuilds the open
// dialog's gutter (the spinner on the busy/retry rows).
func (a *App) applySessionStatusSnapshot(m statusSnapshotMsg) tea.Cmd {
	if m.err != nil {
		return nil
	}
	d := a.dlg.sessions()
	if d == nil || d.sel == nil {
		return nil
	}
	d.status = m.status
	a.rebuildSessionOptions(d)
	return nil
}

// sessionDeleteCmd deletes the session (the S3.1 two-step confirm).
func (a *App) sessionDeleteCmd(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := a.DeleteSession(ctx, id)
		return sessionDeleteMsg{err: err}
	}
}

// sessionDeleteMsg reports the delete result (the deleted id is the dialog's
// armed toDelete — the pinned shape carries no id).
type sessionDeleteMsg struct{ err error }

// sessionTitle is the store session's title (before the delete), falling
// back to Current (the rename.go convention).
func (a *App) sessionTitle(id string) string {
	for _, se := range a.store.Sessions {
		if se.ID == id {
			return se.Title
		}
	}
	if a.store.Current != nil && a.store.Current.ID == id {
		return a.store.Current.Title
	}
	return ""
}

// applySessionDelete lands the delete: success closes the dialog (and
// routes home + hydrates when the current session died); an error opens the
// S3.3 delete-failed dialog (the title from the store session before the
// delete) — or, on a failed retry with that dialog on top, refreshes its
// payload's errMsg in place (it stays open; a success closes it).
func (a *App) applySessionDelete(m sessionDeleteMsg) tea.Cmd {
	if failed := a.dlg.deleteFailed(); failed != nil {
		if m.err != nil {
			failed.errMsg = m.err.Error()
			return nil
		}
		deleted := failed.id
		a.closeTopModal()
		if a.curSessionID == deleted {
			a.route = routeHome
			a.repickTip()
			a.curSessionID = ""
			return a.hydrateCmd()
		}
		return nil
	}
	if m.err != nil {
		d := a.dlg.sessions()
		if d != nil && d.toDelete != "" {
			a.openDeleteFailedDialog(d.toDelete, a.sessionTitle(d.toDelete), m.err.Error())
			return nil
		}
		a.toast(m.err.Error())
		return nil
	}
	deleted := ""
	if d := a.dlg.sessions(); d != nil {
		deleted = d.toDelete
		a.closeTopModal()
	}
	if a.curSessionID == deleted {
		a.route = routeHome
		a.repickTip()
		a.curSessionID = ""
		return a.hydrateCmd()
	}
	return nil
}
