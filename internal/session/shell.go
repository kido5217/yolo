package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

// Shell runs command in the session's persistent shell: it persists the
// user message (row.Agent), the synthetic text part, the assistant
// message (ParentID = the user message on the wire; the storage row has
// no parent column — the wire-only referent, the round convention), and
// the RUNNING bash tool part (input {command}, start time), publishes
// each (message.updated / message.part.updated, the Send publish
// pattern), then spawns the exec goroutine and returns the ids. Errors:
// storage.ErrNotFound (unknown session), ErrShellClosed (deleted), a
// persistence failure. A shell run is independent of turns — no
// busy-map interaction; the shell mutex serializes concurrent execs. The
// exec runs on a session-scoped cancel that Close/Shutdown invoke to kill
// a running command (the turn-Abort referent, yolo-i84): a killed run
// finalizes as `error` with the bash tool's pinned "command aborted"
// message (finalize-must-land, the terminal publishes suppressed).
func (e *Engine) Shell(ctx context.Context, sessionID, command string) (ShellResult, error) {
	row, err := e.db.GetSession(ctx, sessionID)
	if err != nil {
		return ShellResult{}, err
	}
	info, model, err := e.prov.Resolve(row.Model)
	if err != nil {
		return ShellResult{}, err
	}
	sh := e.shellFor(sessionID, row.ProjectDir)
	if sh == nil {
		return ShellResult{}, ErrShellClosed
	}
	// A miswired Deps without the bash tool would nil-panic in the
	// detached exec goroutine (no recover): surface it up front, the
	// tool.Registry's fixed surface always carries it.
	if _, ok := e.tools["bash"]; !ok {
		return ShellResult{}, errors.New("session: bash tool not wired")
	}

	now := e.clock()
	userMsgID := protocol.NewID("msg")
	synthID := protocol.NewID("prt")
	asstID := protocol.NewID("msg")
	partID := protocol.NewID("prt")

	if err := e.db.CreateMessage(ctx, storage.MessageRow{
		ID: userMsgID, SessionID: sessionID, Role: "user", Agent: row.Agent, TimeCreated: now,
	}); err != nil {
		return ShellResult{}, err
	}
	// The CreateMessage row carries no model, the wire does (the Send
	// publish pattern).
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: sessionID,
		Info: protocol.Message{
			ID: userMsgID, SessionID: sessionID, Role: "user", Agent: row.Agent,
			Time:  protocol.MessageTime{Created: now},
			Model: &protocol.MessageModel{ProviderID: info.ID, ModelID: model.ID},
		},
	})

	synthetic := true
	synth := protocol.Part{
		ID: synthID, SessionID: sessionID, MessageID: userMsgID,
		Type: "text", Text: "The following tool was executed by the user",
		IsSynthetic: &synthetic, Time: protocol.PartTime{Start: now},
	}
	synthRow, err := storage.ProtocolToPart(synth)
	if err != nil {
		return ShellResult{}, fmt.Errorf("session: persist synthetic part: %w", err)
	}
	if err := e.db.UpsertPart(ctx, synthRow); err != nil {
		return ShellResult{}, err
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: sessionID, Part: synth, Time: e.clock(),
	})

	if err := e.db.CreateMessage(ctx, storage.MessageRow{
		ID: asstID, SessionID: sessionID, Role: "assistant", Agent: row.Agent, TimeCreated: now,
	}); err != nil {
		return ShellResult{}, err
	}
	asstMsg := protocol.Message{
		ID: asstID, SessionID: sessionID, Role: "assistant", Agent: row.Agent,
		ParentID: userMsgID,
		Time:     protocol.MessageTime{Created: now},
		Model:    &protocol.MessageModel{ProviderID: info.ID, ModelID: model.ID},
	}
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: sessionID, Info: asstMsg,
	})

	running := protocol.Part{
		ID: partID, SessionID: sessionID, MessageID: asstID,
		Type: "tool", Tool: "bash", CallID: partID,
		State: &protocol.ToolState{
			Status: "running",
			Input:  map[string]any{"command": command},
			Time:   protocol.PartTime{Start: now},
		},
	}
	runningRow, err := storage.ProtocolToPart(running)
	if err != nil {
		return ShellResult{}, fmt.Errorf("session: persist running part: %w", err)
	}
	if err := e.db.UpsertPart(ctx, runningRow); err != nil {
		return ShellResult{}, err
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: sessionID, Part: running, Time: e.clock(),
	})

	// The user's shell command is NOT tied to a turn (no busy-map
	// interaction) and outlives the POST that spawned it, so the exec's
	// ctx is a session-scoped cancel rather than the request ctx:
	// Close/Shutdown cancel it to KILL a running command (the turn-Abort
	// referent, yolo-i84 — pre-fix this ctx was context.Background with
	// no abort surface, and Close blocked on the shell mutex for the
	// command's full timeout). Stored before the spawn so a fast command
	// can never finish before the entry exists; the exec removes it on
	// exit (the endTurn referent).
	shellCtx, cancelShell := context.WithCancel(context.Background())
	e.mu.Lock()
	e.shellAbort[sessionID] = cancelShell
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.shellAbort, sessionID)
			e.mu.Unlock()
		}()
		tl := e.tools["bash"]
		raw, _ := json.Marshal(map[string]any{
			"command": command,
			"timeout": int(e.shellTimeout / time.Millisecond),
		})
		cfg, _ := e.loadCfg(row.ProjectDir) // a load failure degrades to
		// the tool.Limits{} defaults via limitsFor's nil branch
		env := &tool.Env{
			Dir:       row.ProjectDir,
			Shell:     sh,
			Limits:    e.limitsFor(cfg),
			OutputDir: e.outputDir,
			Storage:   e.db,
			SessionID: sessionID,
			Log:       e.lg,
		}
		out, runErr := tl.Run(shellCtx, raw, env)
		e.finalizeShellPart(sessionID, asstMsg, partID, now, out, runErr)
	}()
	return ShellResult{MessageID: userMsgID, PartID: partID}, nil
}

// finalizeShellPart persists the shell part's terminal state and
// re-publishes it, then re-publishes the assistant message with
// Time.Completed set (the upstream finish finalizes the message's
// completed time). runErr == nil maps to "completed" with the bash
// tool's output + exit meta; runErr != nil maps to "error" with the
// tool's pinned message (the timeout string, command aborted, …). The
// full assistant info is carried — the TUI store's upsertMessage
// REPLACES the whole Info, so a minimal publish would wipe
// role/agent/model/parent from the row (the surfaceTurnError idiom).
func (e *Engine) finalizeShellPart(sessionID string, asst protocol.Message, partID string, start int64, out tool.Output, runErr error) {
	end := e.clock()
	state := protocol.ToolState{
		Status:   "completed",
		Title:    out.Title,
		Output:   out.Text,
		Metadata: out.Meta,
		Time:     protocol.PartTime{Start: start, End: end},
	}
	if runErr != nil {
		state = protocol.ToolState{
			Status: "error",
			Error:  runErr.Error(),
			Output: "",
			Time:   protocol.PartTime{Start: start, End: end},
		}
	}
	p := protocol.Part{
		ID: partID, SessionID: sessionID, MessageID: asst.ID,
		Type: "tool", Tool: "bash", CallID: partID, State: &state,
	}
	row, perr := storage.ProtocolToPart(p)
	if perr != nil {
		e.lg.Error("persist part marshal failed", "part_id", p.ID, "session_id", sessionID, "error", perr)
		return
	}
	// Finalization must land even for a deleted session: the exec's own
	// ctx is cancelled by Close (the kill that ended this run), so the
	// terminal write rides a fresh uncancellable ctx — the saveToolPart
	// finalize-must-land pattern made explicit — and the publishes below
	// are then suppressed by eventSuppressed.
	ectx := context.Background()
	if err := e.db.UpsertPart(context.WithoutCancel(ectx), row); err != nil {
		e.lg.Error("persist part failed", "part_id", p.ID, "session_id", sessionID, "error", err)
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: sessionID, Part: p, Time: e.clock(),
	})
	asst.Time.Completed = e.clock()
	e.publish(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: sessionID, Info: asst,
	})
}
