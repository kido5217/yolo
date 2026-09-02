// retrydlg.go — the retry-action dialog (S3.7): the two-pill choice on the
// current session's idle->retry transition (the server's retry attempts
// before exhaustion). The upstream dialog-retry-action shape + keys port
// verbatim (title bold + "esc" muted, the muted message, the pills left
// "don't show again" / right the action, starts selected on the action,
// left/right/tab toggle, enter confirms, esc dismisses); the trigger, the
// per-run gate and the action adapt — deviation 194.

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// retryToggleKey / retryEnterKey drive the two pills (left/right/tab toggle
// the selection, enter confirms the active; the modal stack consumes
// esc/ctrl+c first).
var (
	retryToggleKey = key.NewBinding(key.WithKeys("left", "right", "tab"))
	retryEnterKey  = key.NewBinding(key.WithKeys("enter"))
)

// retryDlg is the retry-action payload: the title/message/actionLabel and the
// active pill (0 = dismiss / "don't show again", 1 = the action). th is the
// theme at open (the pinned view takes no theme arg — the 197(c) convention).
type retryDlg struct {
	title       string
	message     string
	actionLabel string
	selected    int
	th          theme.Theme
}

// openRetryActionDialog opens the retry-action dialog (S3.7): the modal
// starts selected on the action (the upstream selected="action").
func (a *App) openRetryActionDialog(title, message, actionLabel string) []tea.Cmd {
	d := &retryDlg{title: title, message: message, actionLabel: actionLabel, selected: 1, th: a.theme}
	a.pushModal(dialog{kind: dlgRetryAction, retry: d}, dlgMedium, nil)
	return nil
}

// headerRow is the dialog header: the bold title left, the muted "esc"
// right, space-between at the panel width.
func (d *retryDlg) headerRow(w int) string {
	pad := w - runeWidth(d.title) - runeWidth("esc")
	if pad < 0 {
		pad = 0
	}
	return title.Render(d.title) + strings.Repeat(" ", pad) + d.th.TextMuted().Render("esc")
}

// pill renders one pills-row pill (pad 0 3, the help "ok" pill precedent):
// the active pill is the primary bg with the SelectedForeground-on-primary
// fg (the select active-row chain); the inactive pill is the muted text.
func (d *retryDlg) pill(label string, active bool) string {
	if !active {
		return d.th.TextMuted().Padding(0, 3).Render(label)
	}
	bg, ok := d.th.Color("primary")
	if !ok {
		return cursorStyle(d.th).Padding(0, 3).Render(label)
	}
	fg := lipgloss.Color(d.th.SelectedForeground(bg).Hex()[:7])
	return lipgloss.NewStyle().Foreground(fg).
		Background(lipgloss.Color(bg.Hex()[:7])).Padding(0, 3).Render(label)
}

// pillsRow is the two pills space-between at the panel width: "don't show
// again" left, the actionLabel right; the active pill paints the selection.
func (d *retryDlg) pillsRow(w int) string {
	left := d.pill("don't show again", d.selected == 0)
	right := d.pill(d.actionLabel, d.selected == 1)
	pad := w - ansiWidth(left) - ansiWidth(right)
	if pad < 0 {
		pad = 0
	}
	return left + strings.Repeat(" ", pad) + right
}

// view renders the dialog (the modal stack draws the panel chrome): the
// header row, the muted message (wrapped at w-4) and the pills row. The link
// line is unused (no BgPulse dep — deviation 194).
func (d *retryDlg) view(w, h int) string {
	var b strings.Builder
	b.WriteString(d.headerRow(w))
	for _, l := range strings.Split(wrapLine(d.message, w-4), "\n") {
		b.WriteString("\n" + d.th.TextMuted().Render(l))
	}
	b.WriteString("\n\n" + d.pillsRow(w))
	return b.String()
}

// handleKey drives the pills (the modal stack consumes esc/ctrl+c first):
// left/right/tab toggle the selection (0/1), enter confirms the active —
// the action closes + aborts (the wire Abort; the turn lands on the
// existing aborted flow), the dismiss closes silently.
func (d *retryDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	switch {
	case key.Matches(k, retryToggleKey):
		if d.selected == 1 {
			d.selected = 0
		} else {
			d.selected = 1
		}
	case key.Matches(k, retryEnterKey):
		if d.selected == 1 {
			a.closeTopModal()
			return a.emit(a.abortCmd())
		}
		a.closeTopModal()
	}
	return nil
}
