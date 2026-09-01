// rename.go — the session-rename dialog (S3.2): the themed huh input seeded
// with the session's current title; confirm patches the title (PatchSession),
// an empty value is a no-op (the upstream guard), and success toasts nothing
// (upstream parity: update({title}) + clear).

package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// openSessionRenameDialog pushes the rename form modal (S3.2): the themed
// huh input seeded with the session's current title (store.Sessions, falling
// back to Current), the dlgMedium form (the upstream DialogPrompt default).
func (a *App) openSessionRenameDialog(id string) []tea.Cmd {
	title := ""
	for _, se := range a.store.Sessions {
		if se.ID == id {
			title = se.Title
			break
		}
	}
	if title == "" && a.store.Current != nil && a.store.Current.ID == id {
		title = a.store.Current.Title
	}
	form := buildInputForm(a.theme, "Rename Session", "", "Title", title)
	return a.openFormModal(form, dlgMedium, func(app *App, f *huh.Form) {
		value := f.GetString("value")
		if value == "" {
			return // the upstream guard: the dialog already closed (cascade)
		}
		app.emit(app.renameCmd(id, value))
	}, nil)
}

// renameCmd patches the session title (the S3.2 confirm).
func (a *App) renameCmd(id, title string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := a.PatchSession(ctx, id, map[string]any{"title": title})
		return renameMsg{id: id, title: title, err: err}
	}
}

// renameMsg reports the rename patch result.
type renameMsg struct {
	id    string
	title string
	err   error
}

// applyRename lands the rename: an error toasts the error string; success
// updates the matching store.Sessions entry's Title + Current.Title when it
// is the current session — with no toast (upstream parity).
func (a *App) applyRename(m renameMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	for i := range a.store.Sessions {
		if a.store.Sessions[i].ID == m.id {
			a.store.Sessions[i].Title = m.title
			break
		}
	}
	if a.store.Current != nil && a.store.Current.ID == m.id {
		a.store.Current.Title = m.title
	}
	return nil
}
