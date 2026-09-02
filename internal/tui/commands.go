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
		_, err := a.SendMessage(ctx, id, text)
		return sendMsg{err: err}
	}
}

// sendMsg reports the result of a prompt send. On success the input clears;
// on error the line is kept for retry.
type sendMsg struct{ err error }

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
	// The next send re-arms the S3.7 retry-action per-run gate (deviation
	// 194): the suppression for this session clears on a successful send.
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

// openPaletteDialog pushes the command palette select modal (S4.4): the
// options = the merged command list (the 4 local commands first, then the
// GET /command catalog — the slash-menu convention; an empty pre-hydrate
// catalog degrades to the locals). Each option's footer = the registry
// binding's Format (the commandBindings referent subset; blank when "none").
// The onSelect is wired in S4.5 (the run-on-enter) — at S4.4 the palette
// opens and filters (the S2.5 fuzzy) but enter is inert.
func (a *App) openPaletteDialog() []tea.Cmd {
	m := selectNew("Commands", "Filter commands", paletteOptions(a), nil, nil, nil)
	a.pushModal(dialog{kind: dlgPalette, sel: m}, dlgMedium, nil)
	return nil
}

// paletteOptions builds the palette select options from the merged command
// list (the 4 local commands first, then the GET /command catalog).
func paletteOptions(a *App) []selectOption {
	var opts []selectOption
	for _, c := range a.mergedCommands() {
		footer := ""
		if bn, ok := commandBindings[c.Name]; ok {
			if f := a.keymap.Format(bn); f != "none" {
				footer = f
			}
		}
		opts = append(opts, selectOption{
			title:       strings.TrimPrefix(c.Name, "/"),
			description: c.Description,
			footer:      footer,
			value:       c.Name,
		})
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
		a.dlg.push(dialog{kind: dlgQuit})
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
