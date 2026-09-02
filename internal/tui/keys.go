package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

var escBinding = key.NewBinding(key.WithKeys("esc"))

// dlgCtrlC is the modal's second close binding (upstream dialog.tsx: esc AND
// ctrl+c are both "Close dialog").
var dlgCtrlC = key.NewBinding(key.WithKeys("ctrl+c"))

// handleKey is the app key dispatcher: permission > dialog > the keymap
// registry (app-level bindings, S4.2) > slash menu > @-picker > route >
// prompt. A pending permission ask owns every key; while a dialog is open it
// owns the keys; otherwise the keymap registry owns the app-level bindings
// (the leader + the base context group); while the slash menu is open it
// owns the keys; while the @-picker is open it owns the keys; routes handle
// their navigation keys; everything else falls through to the always-focused
// prompt input.
func (a *App) handleKey(k tea.KeyPressMsg) []tea.Cmd {
	if len(a.store.Pending) > 0 {
		if d, ok := a.dlg.top(); ok && d.kind == dlgPerm && d.perm != nil {
			return d.perm.handleKey(a, k)
		}
		return (&permDlg{}).handleKey(a, k)
	}
	if d, ok := a.dlg.top(); ok {
		return a.handleDialogKey(d, k)
	}
	// S4.2: the keymap registry owns the app-level bindings (any route, no
	// dialog). The leader mechanism first, then the base context group.
	if cmds, done := a.handleAppKeys(k); done {
		return cmds
	}
	if a.prompt.slashActive() {
		return a.handleMenuKey(k)
	}
	if a.prompt.mentionActive() {
		return a.handleAcKey(k)
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

// handleAppKeys dispatches the keymap registry's app-level bindings (any
// route, no dialog, no pending permission): the leader mechanism first, then
// the base context group in order. It reports whether the key was consumed;
// unhandled keys fall through to the slash menu / route / prompt.
func (a *App) handleAppKeys(k tea.KeyPressMsg) ([]tea.Cmd, bool) {
	km := a.keymap
	// The leader binding arms (or re-arms, while pending) the pending state
	// and consumes the key (a leader keypress while pending re-arms).
	if km.Match("leader", k) {
		a.pendingLeader = true
		return []tea.Cmd{leaderTick()}, true
	}
	if a.pendingLeader {
		// A second key: match a <leader> continuation (base group order); a
		// match dispatches; a miss clears and re-dispatches (the key is not
		// lost — deviation 211).
		a.pendingLeader = false
		if name, ok := a.matchLeaderContinuation(k); ok {
			return a.dispatchCommand(name), true
		}
		if name, done := a.matchBase(k); done {
			return a.dispatchCommand(name), true
		}
		return nil, false // fall through to the slash menu / route / prompt
	}
	if name, done := a.matchBase(k); done {
		return a.dispatchCommand(name), true
	}
	return nil, false
}

// leaderTick arms the leader timeout (the ported registerTimedLeader tick).
func leaderTick() tea.Cmd {
	return tea.Tick(LeaderTimeout, func(time.Time) tea.Msg { return leaderTimeoutMsg{} })
}

// matchLeaderContinuation matches a second key against the base group's
// <leader> continuations (base group order); it returns the matched binding
// name.
func (a *App) matchLeaderContinuation(k tea.KeyPressMsg) (string, bool) {
	for _, name := range contextGroups[BaseMode] {
		if a.keymap.MatchPending(name, k) {
			return name, true
		}
	}
	return "", false
}

// matchBase matches the base context group in order (app_exit's ctrl+d seq is
// prompt-owned — skipped, the upstream input-layer-wins semantics; the prompt
// is always focused in yolo). It returns the matched binding name.
func (a *App) matchBase(k tea.KeyPressMsg) (string, bool) {
	for _, name := range contextGroups[BaseMode] {
		skipCtrlD := name == "app_exit"
		for _, seq := range a.keymap.Seqs(name) {
			if skipCtrlD && seq == "ctrl+d" {
				continue
			}
			if keyMatchesSeq(k, seq) {
				return name, true
			}
		}
	}
	return "", false
}

// dispatchCommand runs a referent-bearing registry command. The which_key_*
// (S4.6) case is consumed but inert (the case lands in that task).
func (a *App) dispatchCommand(name string) []tea.Cmd {
	switch name {
	case "command_list":
		return a.openPaletteDialog()
	case "app_exit":
		a.dlg.push(dialog{kind: dlgQuit})
	case "model_list":
		return a.openModelDialog()
	case "agent_list":
		return a.openAgentDialog()
	case "status_view":
		return a.openStatusDialog()
	case "theme_list":
		return a.openThemeListDialog()
	case "session_new":
		return a.emit(a.createSessionCmd())
	case "session_list":
		return a.openSessionListDialog()
	case "provider_connect":
		return a.openProviderDialog()
	case "help_show":
		a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
		// which_key_* (S4.6) is consumed but inert here.
	}
	return nil
}

// handleMenuKey dispatches keys while the slash menu is open: arrows move
// the selection with wraparound, enter executes the selection (or clears the
// input on no match), esc closes the menu; everything else keeps filtering
// through the live input. The menu items and the view (view.go) use the same
// merged command list (local + server — spec §10) so the rendered row and
// the executed row stay in step.
func (a *App) handleMenuKey(k tea.KeyPressMsg) []tea.Cmd {
	items := a.prompt.menuItems(a.mergedCommands())
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
		a.clearPrompt()
		return nil
	case key.Matches(k, escBinding):
		a.clearPrompt()
		return nil
	}
	return a.inputUpdate(k)
}

// handleAcKey dispatches keys while the @-picker is open: arrows move the
// selection with wraparound, enter inserts the selected path (a no-op on no
// selection — the upstream if (!selected) return), esc removes the @-trigger
// keeping the prefix; everything else keeps filtering through the live input
// (re-filtering the options).
func (a *App) handleAcKey(k tea.KeyPressMsg) []tea.Cmd {
	opts := a.mentionOptions()
	switch {
	case key.Matches(k, homeKeyMap.Up):
		a.prompt.moveMenuSel(len(opts), -1)
		return nil
	case key.Matches(k, homeKeyMap.Down):
		a.prompt.moveMenuSel(len(opts), 1)
		return nil
	case key.Matches(k, promptEnter):
		if len(opts) > 0 && a.prompt.sel < len(opts) {
			if p, ok := opts[a.prompt.sel].value.(string); ok {
				a.acInsert(p)
			}
		}
		return nil
	case key.Matches(k, escBinding):
		v := a.prompt.input.Value()
		if idx, ok := mentionTriggerIndex(v); ok {
			a.prompt.input.SetValue(v[:idx])
			a.prompt.input.SetCursor(idx)
		}
		a.prompt.sel = 0
		return nil
	}
	return a.inputUpdate(k)
}

// handlePromptKey is the prompt fallback: up/down recall the prompt history
// (S5.1 — the session-route prompt behavior: the home route's up/down is
// consumed by handleHomeKey and the slash menu owns up/down while open),
// enter sends (or soft-enters a trailing backslash), everything else feeds
// the input.
func (a *App) handlePromptKey(k tea.KeyPressMsg) []tea.Cmd {
	if key.Matches(k, homeKeyMap.Up) {
		a.recallHistory(-1)
		return nil
	}
	if key.Matches(k, homeKeyMap.Down) {
		a.recallHistory(1)
		return nil
	}
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
