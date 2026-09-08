package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
)

// sendMessageCmd posts the composed line as a user message for the current
// session.
func (a *App) sendMessageCmd(text string) tea.Cmd {
	id := a.curSessionID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := a.SendMessage(ctx, id, protocol.SendMessageRequest{Text: text})
		return sendMsg{text: text, err: err}
	}
}

// sendMsg reports the result of a prompt send. On success the input clears
// and the sent text appends to the prompt history (S5.1); on error the line
// is kept for retry.
type sendMsg struct {
	text string
	err  error
}

func (a *App) applySend(m sendMsg) tea.Cmd {
	if m.err != nil {
		if errors.Is(m.err, client.ErrBusy) {
			a.toast(busyToast)
		} else {
			a.lastErr = m.err.Error()
		}
		return nil
	}
	a.prompt.input.SetValue("")
	a.prompt.draft.Reset()
	// A successful send appends to the prompt history (S5.1 — the ported
	// history.append after every send).
	a.appendHistory(m.text)
	// The next send re-arms the S3.7 retry-action per-run gate (deviation
	// 194): the suppression for this session clears on a successful send.
	delete(a.retrySuppressed, a.curSessionID)
	return nil
}

// homeSubmitMsg reports the home submit's mint+send result (decision 2):
// ses is the minted session (zero on a mint failure), text the typed line
// (kept for retry on error).
type homeSubmitMsg struct {
	ses  protocol.Session
	text string
	err  error
}

// configModel is the home submit's model seed: the store.Config["model"]
// string or "" (the server applies the catalog default on blank, matching
// newSession's blank-model branch).
func (a *App) configModel() string {
	if s, ok := a.store.Config["model"].(string); ok {
		return s
	}
	return ""
}

// homeSubmitCmd mints the session (title "" -> "New session", the seeded
// agent+model) and then sends the typed text as its first message —
// two sequential wire calls under per-stage 5s timeouts (the
// createSessionCmd/sendMessageCmd convention).
func (a *App) homeSubmitCmd(text string) tea.Cmd {
	agent := a.pendingAgentName()
	model := a.configModel()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ses, err := a.CreateSessionWith(ctx, "", agent, model)
		cancel()
		if err != nil {
			return homeSubmitMsg{text: text, err: err}
		}
		// the send stage gets a FRESH 5s window (per-stage timeouts): the
		// mint's deadline must not eat the send's budget.
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = a.SendMessage(ctx, ses.ID, protocol.SendMessageRequest{Text: text})
		return homeSubmitMsg{ses: ses, text: text, err: err}
	}
}

// homeShellCmd is the shell-mode twin: mint (the same seed), then POST
// /session/{id}/shell {command: text} (Task 9's client method).
func (a *App) homeShellCmd(text string) tea.Cmd {
	agent := a.pendingAgentName()
	model := a.configModel()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ses, err := a.CreateSessionWith(ctx, "", agent, model)
		cancel()
		if err != nil {
			return homeSubmitMsg{text: text, err: err}
		}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _, err = a.Shell(ctx, ses.ID, text)
		return homeSubmitMsg{ses: ses, text: text, err: err}
	}
}

// shellMsg reports the session-route shell post result; the transcript
// updates via SSE (the msg only drives the post-send state).
type shellMsg struct {
	text string
	err  error
}

// shellCmd posts the composed line to the CURRENT session's shell (the
// session route — the home route mints first, homeShellCmd). No busy
// gate: the shell is serialized by the per-session shell mutex (a shell
// submit during a turn waits for the turn's bash exec — documented
// behavior, decision-silent). Uses the client Shell method (Task 9).
func (a *App) shellCmd(text string) tea.Cmd {
	id := a.curSessionID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _, err := a.Shell(ctx, id, text)
		return shellMsg{text: text, err: err}
	}
}

// applyShell: success -> clear input + draft + appendHistory(text) + clear
// the retry suppression (the applySend post-send state — the transcript
// updates via SSE, isDirty on the applied event). Error -> lastErr
// (ErrBusy -> the busy toast; the session-route convention).
func (a *App) applyShell(m shellMsg) tea.Cmd {
	if m.err != nil {
		if errors.Is(m.err, client.ErrBusy) {
			a.toast(busyToast)
		} else {
			a.lastErr = m.err.Error()
		}
		return nil
	}
	a.prompt.input.SetValue("")
	a.prompt.draft.Reset()
	a.appendHistory(m.text)
	delete(a.retrySuppressed, a.curSessionID)
	return nil
}

// localCommands is the TUI-local slash commands merged client-side into the
// slash menu (the server catalog is frozen at 5 — spec §10).
func localCommands() []protocol.Command {
	return []protocol.Command{
		{Name: "/sessions", Description: "List all sessions"},
		{Name: "/connect", Description: "Connect a provider"},
		{Name: "/status", Description: "View status"},
		{Name: "/themes", Description: "List available themes"},
	}
}

// mergedCommands is the slash menu's command list: the local ones first,
// then the server catalog.
func (a *App) mergedCommands() []protocol.Command {
	return append(localCommands(), a.store.Commands...)
}

// commandBindings maps the yolo command names to the registry binding names
// (the referent subset — the commands with a registry default; the palette
// footer shows the registry's Format for each, blank when "none").
var commandBindings = map[string]string{
	"/help":     "help_show",
	"/new":      "session_new",
	"/model":    "model_list",
	"/agents":   "agent_list",
	"/quit":     "app_exit",
	"/sessions": "session_list",
	"/connect":  "provider_connect",
	"/status":   "status_view",
	"/themes":   "theme_list",
}

// commandCategories maps the yolo command names to the palette's client-side
// category buckets (decision B — the wire protocol.Command has no category
// field; the bucket set is yolo-chosen, not upstream's wire-derived
// categories). The "Suggested" bucket (decision A) overlays these: paletteOptions
// re-categorises the seeded commands "Suggested".
var commandCategories = map[string]string{
	"/help":     "General",
	"/status":   "General",
	"/themes":   "General",
	"/quit":     "General",
	"/new":      "Session",
	"/sessions": "Session",
	"/model":    "Model",
	"/connect":  "Provider",
	"/agents":   "Agent",
}

// openPaletteDialog pushes the command palette select modal (S4.4): the
// options = the merged command list (the 4 local commands first, then the
// GET /command catalog — the slash-menu convention; an empty pre-hydrate
// catalog degrades to the locals). Each option's footer = the registry
// binding's Format (the commandBindings referent subset; blank when "none").
// The onSelect (S4.5) runs the selected command (the run-on-enter contract).
// The 0.10.0 palette S1 inner-line parity: the title row's esc hint
// (palette-scoped — the model/agent dialogs keep the plain title row) and
// the command_list keymap footer hint (right-aligned, replacing the
// generic nav hint for the palette select only).
func (a *App) openPaletteDialog() []tea.Cmd {
	m := selectNew("Commands", "Filter commands", paletteOptions(a), nil,
		func(app *App, o selectOption) { app.paletteSelectPick(o) }, nil).
		WithEscHint().
		WithHints([]footerHint{{key: a.keymap.Format("command_list"), desc: "commands"}})
	a.pushModal(dialog{kind: dlgPalette, sel: m}, dlgMedium, nil)
	return nil
}

// paletteSelectPick is the palette's onSelect (S4.5): it closes the palette
// and runs the selected command (the run-on-enter contract — the port of the
// upstream dialog.clear() + dispatchCommand).
func (a *App) paletteSelectPick(o selectOption) {
	a.closeTopModal()
	if v, ok := o.value.(string); ok {
		a.runCommand(v)
	}
}

// isSuggested reports whether the command belongs in the palette's
// empty-filter-only "Suggested" bucket (decision A — the yolo seed, mapped
// from upstream's per-command seeds onto yolo's 9-command catalog): /model
// always, /new on the session route, /sessions when a session is stored,
// /connect when a provider is not connected.
func (a *App) isSuggested(name string) bool {
	switch name {
	case "/model":
		return true
	case "/new":
		return a.route == routeSession
	case "/sessions":
		return len(a.store.Sessions) > 0
	case "/connect":
		return !a.tipsConnected()
	}
	return false
}

// paletteOptions builds the palette select options from the merged command
// list (the 4 local commands first, then the GET /command catalog). The
// Suggested bucket (decision A) leads: the seeded commands, each
// re-categorised "Suggested" but keeping the plain command name as the value
// (decision B — run-on-enter-ready, no suggested: prefix), so the selection /
// mouse / pick paths are unchanged. Every plain option carries its
// client-side category (decision B).
func paletteOptions(a *App) []selectOption {
	cmds := a.mergedCommands()
	build := func(c protocol.Command) selectOption {
		footer := ""
		if bn, ok := commandBindings[c.Name]; ok {
			if f := a.keymap.Format(bn); f != "none" {
				footer = f
			}
		}
		return selectOption{
			title:       strings.TrimPrefix(c.Name, "/"),
			description: c.Description,
			footer:      footer,
			category:    commandCategories[c.Name],
			value:       c.Name,
		}
	}
	var opts []selectOption
	for _, c := range cmds {
		if a.isSuggested(c.Name) {
			o := build(c)
			o.category = "Suggested"
			opts = append(opts, o)
		}
	}
	for _, c := range cmds {
		opts = append(opts, build(c))
	}
	return opts
}

// runCommand executes a slash command from the menu. /new without a current
// session issues CreateSession directly (LOCKED: the command endpoint needs a
// session id); other commands open their dialogs.
func (a *App) runCommand(name string) []tea.Cmd {
	a.prompt.input.SetValue("")
	switch name {
	case "/help":
		a.pushModal(dialog{kind: dlgHelp}, dlgMedium, nil)
	case "/quit", "/exit": // /exit is the alias of /quit
		return a.emit(quitCmd())
	case "/model":
		return a.openModelDialog()
	case "/agents":
		return a.openAgentDialog()
	case "/sessions":
		return a.openSessionListDialog()
	case "/connect":
		return a.openProviderDialog()
	case "/status":
		return a.openStatusDialog()
	case "/themes":
		return a.openThemeListDialog()
	case "/new":
		if a.curSessionID == "" {
			return a.emit(a.createSessionCmd())
		}
		return a.emit(a.commandCmd("/new"))
	}
	return nil
}

// commandCmd posts a slash command to the server for the current session.
func (a *App) commandCmd(cmd string) tea.Cmd {
	id := a.curSessionID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := a.Command(ctx, id, cmd)
		return commandExecMsg{resp: resp, err: err}
	}
}

// commandExecMsg reports the result of POST /session/{id}/command; a response
// carrying a session_id (server-side /new) switches to it.
type commandExecMsg struct {
	resp protocol.CommandResponse
	err  error
}

func (a *App) applyCommandExec(m commandExecMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	if m.resp.SessionID != "" {
		a.openSession(m.resp.SessionID)
		return a.emit(a.hydrateCmd())[0]
	}
	return nil
}

// quitCmd is a Cmd that tells the program to exit.
func quitCmd() tea.Cmd {
	return tea.Quit
}

// abortedMsg reports the result of the esc-while-busy abort.
type abortedMsg struct{ err error }

// abortCmd posts the server abort for the current session.
func (a *App) abortCmd() tea.Cmd {
	id := a.curSessionID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := a.Abort(ctx, id)
		return abortedMsg{err: err}
	}
}
