package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

var escBinding = key.NewBinding(key.WithKeys("esc"))

// dlgCtrlC is the modal's second close binding (upstream dialog.tsx: esc AND
// ctrl+c are both "Close dialog").
var dlgCtrlC = key.NewBinding(key.WithKeys("ctrl+c"))

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
		a.prompt.draft.WriteString(strings.TrimSuffix(val, "\\") + "\n")
		a.prompt.input.SetValue("")
		return nil
	}
	text := a.prompt.draft.String() + strings.TrimSpace(val)
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
