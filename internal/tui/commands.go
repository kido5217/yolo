package tui

import (
	"context"
	"errors"
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
	return nil
}

// localCommands is the TUI-local slash commands merged client-side into the
// slash menu (the server catalog is frozen at 5 — spec §10; the S3.4/S3.5/
// S3.8 openers append their entries as they land).
func localCommands() []protocol.Command {
	return []protocol.Command{
		{Name: "/sessions", Description: "List all sessions"},
	}
}

// mergedCommands is the slash menu's command list: the local ones first,
// then the server catalog.
func (a *App) mergedCommands() []protocol.Command {
	return append(localCommands(), a.store.Commands...)
}

// runCommand executes a slash command from the menu. /new without a current
// session issues CreateSession directly (LOCKED: the command endpoint needs a
// session id); other commands open their dialogs.
func (a *App) runCommand(name string) []tea.Cmd {
	a.prompt.input.SetValue("")
	switch name {
	case "/help":
		a.dlg.push(dialog{kind: dlgHelp})
	case "/quit", "/exit": // /exit is the alias of /quit
		a.dlg.push(dialog{kind: dlgQuit})
	case "/model":
		return a.openModelDialog()
	case "/agents":
		return a.openAgentDialog()
	case "/sessions":
		return a.openSessionListDialog()
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
