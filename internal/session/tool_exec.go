package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

// executeTool runs one model-issued tool call through the LOCKED permission
// gates, then the tool itself:
//
//  1. doom check (sliding 3-identical window on the turn's call history;
//     the doom ask fires BEFORE the part goes "running");
//  2. hidden guard (a tool denied by a wildcard rule is not offered to the
//     model; if it is called anyway, the part errors "tool not available");
//  3. external-directory gate on tool.External paths outside the session
//     dir (the part is "running" first so the TUI shows the pending state);
//  4. core ask with Resources/Always from tool.Patterns.
//
// Every path finalizes the part. Deny -> "permission rejected"; a ctx
// cancel while parked (Abort) -> "aborted"; the model continues either way.
func (e *Engine) executeTool(ctx context.Context, t *turn, r *round, p llm.Part) {
	name := p.Name
	callID := p.CallID
	if callID == "" {
		callID = protocol.NewID("prt")
	}
	raw := p.Args
	if len(raw) == 0 && p.Text != "" {
		// scripted drivers carry the args JSON in Text (locked convention)
		raw = json.RawMessage(p.Text)
	}
	fail := func(stage int64, msg string) {
		e.saveToolPart(ctx, t, r, toolPart{
			callID: callID,
			name:   name,
			state: protocol.ToolState{
				Status: "error",
				Input:  map[string]any{},
				Error:  msg,
				Time:   protocol.PartTime{Start: e.clock(), End: stage},
			},
		})
	}
	// gateFail finalizes the part for a failed permission gate (service
	// error, deny, or ctx cancel while parked).
	gateFail := func() {
		msg := "permission rejected"
		if ctx.Err() != nil {
			msg = "aborted"
		}
		fail(e.clock(), msg)
	}

	input := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &input); err != nil {
			fail(e.clock(), "invalid tool arguments: "+err.Error())
			return
		}
	}
	tl, ok := e.tools[name]
	if !ok {
		start := e.clock()
		fail(start, "unknown tool "+name)
		return
	}
	rules, err := e.rulesetForRow(ctx, t.row)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}
	if hidden := permission.Hidden(rules, []string{name})[name]; hidden {
		start := e.clock()
		fail(start, "tool not available")
		return
	}
	resources, always, err := tl.Patterns(raw)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}
	external, err := tl.External(raw)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}

	// (1) doom check: the third identical call of the turn asks before it
	// runs (sliding window; a "once" reply does not extend the exemption).
	key := permission.CallKey{Tool: name, Hash: callKeyHash(raw)}
	if permission.DoomLoopDue(t.doomHist, key) {
		e.lg.Info("doom loop trigger", "session_id", t.sessionID, "tool", name)
		d := e.perm.EvaluateRules(t.agent, t.cfgRules, "doom_loop", []string{name})
		doomReq := permission.Request{
			RequestID: protocol.NewID("perm"), SessionID: t.sessionID, Agent: t.agent,
			Permission: "doom_loop", Tool: tl.ID(),
			Resources: []string{name},
			CallID:    callID, MessageID: r.id,
			PreDecision: d, CreatedAt: e.clock(),
			CfgRules: t.cfgRules,
		}
		decision, err := e.perm.Ask(ctx, doomReq)
		if err != nil {
			fail(e.clock(), err.Error())
			return
		}
		if decision != permission.Allow {
			msg := "permission rejected"
			if ctx.Err() != nil {
				msg = "aborted"
			}
			now := e.clock()
			e.saveToolPart(ctx, t, r, toolPart{
				callID: callID,
				name:   name,
				state: protocol.ToolState{
					Status:   "error",
					Input:    input,
					Error:    msg,
					Metadata: map[string]any{"reason": "doom_loop"},
					Time:     protocol.PartTime{Start: now, End: now},
				},
			})
			t.doomHist = append(t.doomHist, key)
			return
		}
	}
	t.doomHist = append(t.doomHist, key)

	start := e.clock()
	e.saveToolPart(ctx, t, r, toolPart{
		callID: callID,
		name:   name,
		state:  protocol.ToolState{Status: "running", Input: input, Time: protocol.PartTime{Start: start}},
	})

	// (3) external-directory gate.
	for _, ext := range external {
		abs := ext
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(t.row.ProjectDir, abs)
		}
		abs = filepath.Clean(abs)
		if inside, _ := withinDir(t.row.ProjectDir, abs); inside {
			continue
		}
		pattern := filepath.Dir(abs) + "/*"
		d := e.perm.EvaluateRules(t.agent, t.cfgRules, "external_directory", []string{pattern})
		extReq := permission.Request{
			RequestID: protocol.NewID("perm"), SessionID: t.sessionID, Agent: t.agent,
			Permission: "external_directory", Tool: tl.ID(),
			Resources: []string{pattern},
			CallID:    callID, MessageID: r.id,
			PreDecision: d, CreatedAt: e.clock(),
			CfgRules: t.cfgRules,
		}
		decision, aerr := e.perm.Ask(ctx, extReq)
		if aerr != nil || decision != permission.Allow {
			gateFail()
			return
		}
	}

	// (4) core permission.
	d := e.perm.EvaluateRules(t.agent, t.cfgRules, tl.Permission(), resources)
	preq := permission.Request{
		RequestID: protocol.NewID("perm"), SessionID: t.sessionID, Agent: t.agent,
		Permission: tl.Permission(), Tool: tl.ID(),
		Resources: resources, Always: always,
		CallID: callID, MessageID: r.id,
		PreDecision: d, CreatedAt: e.clock(),
		CfgRules: t.cfgRules,
	}
	decision, err := e.perm.Ask(ctx, preq)
	if err != nil {
		fail(e.clock(), err.Error())
		return
	}
	if decision != permission.Allow {
		gateFail()
		return
	}

	env := &tool.Env{
		Dir:       t.row.ProjectDir,
		Shell:     e.shellFor(t.sessionID, t.row.ProjectDir),
		Limits:    e.limitsFor(t.cfg),
		OutputDir: e.outputDir,
		Storage:   e.db,
		SessionID: t.sessionID,
		Log:       e.lg,
	}
	e.lg.Info("tool start", "session_id", t.sessionID, "tool", name)
	toolStart := e.clock()
	out, runErr := tl.Run(ctx, raw, env)
	if runErr != nil {
		msg := runErr.Error()
		if ctx.Err() != nil {
			// Abort while the tool ran: label it plainly; the process was
			// already force-killed via ctx.
			msg = "aborted"
		}
		e.lg.Error("tool failed", "session_id", t.sessionID, "tool", name, "error", msg)
		e.saveToolPart(ctx, t, r, toolPart{
			callID: callID,
			name:   name,
			state: protocol.ToolState{
				Status:   "error",
				Input:    input,
				Title:    out.Title,
				Output:   out.Text,
				Error:    msg,
				Metadata: out.Meta,
				Time:     protocol.PartTime{Start: start, End: e.clock()},
			},
		})
		return
	}
	{
		args := []any{"session_id", t.sessionID, "tool", name, "latency_ms", e.clock() - toolStart}
		if v, ok := out.Meta["exit"]; ok {
			args = append(args, "exit_code", v)
		}
		if v, ok := out.Meta["truncated"]; ok {
			args = append(args, "truncated", v)
		}
		e.lg.Info("tool end", args...)
	}
	e.saveToolPart(ctx, t, r, toolPart{
		callID: callID,
		name:   name,
		state: protocol.ToolState{
			Status:   "completed",
			Input:    input,
			Title:    out.Title,
			Output:   out.Text,
			Metadata: out.Meta,
			Time:     protocol.PartTime{Start: start, End: e.clock()},
		},
	})
}

// toolPart is one tool part save: the call identity plus the persisted tool
// state (the start/end times live in state.Time).
type toolPart struct {
	callID string
	name   string
	state  protocol.ToolState
}

func (e *Engine) saveToolPart(ctx context.Context, t *turn, r *round, tp toolPart) {
	p := protocol.Part{
		ID: tp.callID, SessionID: t.sessionID, MessageID: r.id,
		Type: "tool", Tool: tp.name, CallID: tp.callID, State: &tp.state,
	}
	row, perr := storage.ProtocolToPart(p)
	if perr != nil {
		e.lg.Error("persist part marshal failed", "part_id", p.ID, "session_id", t.sessionID, "error", perr)
		return
	}
	// Finalization must land even when the turn ctx is cancelled (abort):
	// a cancelled ctx would drop the terminal tool-part write and leave the
	// part "running" in the store.
	if err := e.db.UpsertPart(context.WithoutCancel(ctx), row); err != nil {
		e.lg.Error("persist part failed", "part_id", p.ID, "session_id", t.sessionID, "error", err)
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: t.sessionID, Part: p, Time: e.clock(),
	})
}

// callKeyHash returns the sha256 hex of the canonical (sorted-key) JSON form
// of the tool args, so deep-equal inputs hash equal regardless of key order.
func callKeyHash(raw json.RawMessage) string {
	var v any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &v); err != nil {
			// unparseable args fall through to the raw bytes; the doom window
			// only ever sees well-formed calls (validated before use).
			v = string(raw)
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		b = raw
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// withinDir reports whether p is inside (or is) root.
func withinDir(root, p string) (bool, error) {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}
