// deletefailed.go — the session-delete-failed dialog (S3.3): the two-option
// retry/keep choice after a failed delete. The upstream
// dialog-session-delete-failed shape + keys port verbatim (return confirms
// the active option, left/up = the first option, right/down = the second,
// esc close); the options + body adapt — deviation 191.

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// dfLeftKey / dfRightKey / dfEnterKey drive the two option rows (the
// two-row clamp — no wrap: left/up = the retry row, right/down = the keep
// row, enter confirms the active).
var (
	dfLeftKey  = key.NewBinding(key.WithKeys("left", "up"))
	dfRightKey = key.NewBinding(key.WithKeys("right", "down"))
	dfEnterKey = key.NewBinding(key.WithKeys("enter"))
)

// deleteFailedDlg is the delete-failed payload: the deleted session's id +
// title, the wire error, and the active option (0 = retry, 1 = keep).
type deleteFailedDlg struct {
	id     string
	title  string
	errMsg string
	active int
}

// openDeleteFailedDialog opens the delete-failed dialog (S3.3): it replaces
// the session list when it is on top (the upstream dialog.replace
// semantics), else pushes; the active row starts on the retry.
func (a *App) openDeleteFailedDialog(id, title, errMsg string) []tea.Cmd {
	d := &deleteFailedDlg{id: id, title: title, errMsg: errMsg}
	item := dialog{kind: dlgDeleteFailed, deleteFailed: d}
	if top, ok := a.dlg.top(); ok && top.kind == dlgSessions {
		a.replaceModal(item, dlgMedium, nil)
		return nil
	}
	a.pushModal(item, dlgMedium, nil)
	return nil
}

// headerRow is the dialog header: the bold title left, the muted "esc"
// right, space-between at the panel width.
func (d *deleteFailedDlg) headerRow(w int, th theme.Theme) string {
	const t = "Failed to Delete Session"
	pad := w - runeWidth(t) - runeWidth("esc")
	if pad < 0 {
		pad = 0
	}
	return title.Render(t) + strings.Repeat(" ", pad) + th.TextMuted().Render("esc")
}

// optionRow renders one option row: the active row is the full-row paint in
// the primary bg with the SelectedForeground-on-primary fg (the select
// active-row chain — bold title + the description tail); the inactive rows
// are the text title + the muted description.
func (d *deleteFailedDlg) optionRow(label, desc string, active bool, w int, th theme.Theme) string {
	tail := "  " + desc
	if active {
		bg, ok := th.Color("primary")
		if !ok {
			return cursorStyle(th).Render(label) + th.TextMuted().Render(tail)
		}
		fg := lipgloss.Color(th.SelectedForeground(bg).Hex()[:7])
		bgC := lipgloss.Color(bg.Hex()[:7])
		head := lipgloss.NewStyle().Foreground(fg).Background(bgC).Bold(true)
		tailSty := lipgloss.NewStyle().Foreground(fg).Background(bgC)
		return lipgloss.NewStyle().Background(bgC).Width(w).Render(head.Render(label) + tailSty.Render(tail))
	}
	return th.Text().Render(label) + th.TextMuted().Render(tail)
}

// view renders the dialog (the modal stack draws the panel chrome): the
// header row, the muted body (the session title + the wire error + the
// proceed hint, wrapped at w-4) and the two option rows.
func (d *deleteFailedDlg) view(w, h int, th theme.Theme) string {
	const (
		retryLabel  = "Retry delete"
		retryDesc   = "Try to delete the session again."
		keepLabel   = "Keep session"
		keepDesc    = "Cancel the delete and keep the session."
		proceedHint = "Choose how to proceed."
	)
	var b strings.Builder
	b.WriteString(d.headerRow(w, th))
	body := "The session \"" + d.title + "\" could not be deleted: " + d.errMsg
	for _, l := range strings.Split(wrapLine(body, w-4), "\n") {
		b.WriteString("\n" + th.TextMuted().Render(l))
	}
	b.WriteString("\n\n" + th.TextMuted().Render(wrapLine(proceedHint, w-4)))
	b.WriteString("\n" + d.optionRow(retryLabel, retryDesc, d.active == 0, w, th))
	b.WriteString("\n" + d.optionRow(keepLabel, keepDesc, d.active == 1, w, th))
	return b.String()
}

// handleKey drives the options (the modal stack consumes esc/ctrl+c
// first): left/up → the retry row, right/down → the keep row (the two-row
// clamp — no wrap), enter confirms the active row.
func (d *deleteFailedDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	switch {
	case key.Matches(k, dfLeftKey):
		d.active = 0
	case key.Matches(k, dfRightKey):
		d.active = 1
	case key.Matches(k, dfEnterKey):
		if d.active == 1 {
			a.closeTopModal()
			return a.emit(a.hydrateCmd())
		}
		// The retry re-issue stays open: a failed retry re-enters through
		// applySessionDelete (the payload errMsg refreshes); a success
		// closes the dialog.
		return a.emit(a.sessionDeleteCmd(d.id))
	}
	return nil
}
