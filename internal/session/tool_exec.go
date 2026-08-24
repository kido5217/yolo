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

// toolCall bundles the per-call context of one in-flight tool call: the
// session identity, the turn config + permission rules, the assistant
// message id, the turn's doom history (aliased so gate appends land on the
// turn), and the part itself.
type toolCall struct {
	ctx       context.Context
	sessionID string
	agent     string
	row       storage.SessionRow
	cfg       *protocol.Config
	cfgRules  []protocol.Rule
	asstID    string
	doomHist  *[]permission.CallKey
	part      llm.Part
}

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
	if p.CallID == "" {
		p.CallID = protocol.NewID("prt")
	}
	raw := p.Args
	if len(raw) == 0 && p.Text != "" {
		// scripted drivers carry the args JSON in Text (locked convention)
		raw = json.RawMessage(p.Text)
	}
	tc := &toolCall{
		ctx: ctx, sessionID: t.sessionID, agent: t.agent,
		row: t.row, cfg: t.cfg, cfgRules: t.cfgRules,
		asstID: r.id, doomHist: &t.doomHist, part: p,
	}
	input := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &input); err != nil {
			e.failToolPart(tc, e.clock(), "invalid tool arguments: "+err.Error())
			return
		}
	}
	tl, ok := e.tools[p.Name]
	if !ok {
		start := e.clock()
		e.failToolPart(tc, start, "unknown tool "+p.Name)
		return
	}
	rules, err := e.rulesetForRow(ctx, t.row)
	if err != nil {
		e.failToolPart(tc, e.clock(), err.Error())
		return
	}
	if hidden := permission.Hidden(rules, []string{p.Name})[p.Name]; hidden {
		start := e.clock()
		e.failToolPart(tc, start, "tool not available")
		return
	}
	resources, always, err := tl.Patterns(raw)
	if err != nil {
		e.failToolPart(tc, e.clock(), err.Error())
		return
	}
	external, err := tl.External(raw)
	if err != nil {
		e.failToolPart(tc, e.clock(), err.Error())
		return
	}

	key := permission.CallKey{Tool: p.Name, Hash: callKeyHash(raw)}
	if !e.checkDoom(tc, tl, key, input) {
		return
	}

	start := e.clock()
	e.saveToolPart(tc, toolPart{
		callID: p.CallID,
		name:   p.Name,
		state:  protocol.ToolState{Status: "running", Input: input, Time: protocol.PartTime{Start: start}},
	})

	if !e.gateExternal(tc, tl, external) {
		return
	}

	if !e.coreAsk(tc, tl, resources, always) {
		return
	}

	env := &tool.Env{
		Dir:       tc.row.ProjectDir,
		Shell:     e.shellFor(tc.sessionID, tc.row.ProjectDir),
		Limits:    e.limitsFor(tc.cfg),
		OutputDir: e.outputDir,
		Storage:   e.db,
		SessionID: tc.sessionID,
		Log:       e.lg,
	}
	e.lg.Info("tool start", "session_id", tc.sessionID, "tool", p.Name)
	toolStart := e.clock()
	out, runErr := tl.Run(ctx, raw, env)
	if runErr != nil {
		msg := runErr.Error()
		if ctx.Err() != nil {
			// Abort while the tool ran: label it plainly; the process was
			// already force-killed via ctx.
			msg = "aborted"
		}
		e.lg.Error("tool failed", "session_id", tc.sessionID, "tool", p.Name, "error", msg)
		e.saveToolPart(tc, toolPart{
			callID: p.CallID,
			name:   p.Name,
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
		args := []any{"session_id", tc.sessionID, "tool", p.Name, "latency_ms", e.clock() - toolStart}
		if v, ok := out.Meta["exit"]; ok {
			args = append(args, "exit_code", v)
		}
		if v, ok := out.Meta["truncated"]; ok {
			args = append(args, "truncated", v)
		}
		e.lg.Info("tool end", args...)
	}
	e.saveToolPart(tc, toolPart{
		callID: p.CallID,
		name:   p.Name,
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

// checkDoom runs the doom check (sliding 3-identical window on the turn's
// call history): the third identical call of the turn asks before it runs
// (the ask fires BEFORE the part goes "running"); a "once" reply does not
// extend the exemption. Every pass records the call in the turn's doom
// history; the gate reports whether the call may proceed (an Ask error or
// a non-Allow decision finalizes the part and stops the call).
func (e *Engine) checkDoom(tc *toolCall, tl tool.Tool, key permission.CallKey, input map[string]any) bool {
	if !permission.DoomLoopDue(*tc.doomHist, key) {
		*tc.doomHist = append(*tc.doomHist, key)
		return true
	}
	e.lg.Info("doom loop trigger", "session_id", tc.sessionID, "tool", tc.part.Name)
	d := e.perm.EvaluateRules(tc.agent, tc.cfgRules, "doom_loop", []string{tc.part.Name})
	doomReq := permission.Request{
		RequestID: protocol.NewID("perm"), SessionID: tc.sessionID, Agent: tc.agent,
		Permission: "doom_loop", Tool: tl.ID(),
		Resources: []string{tc.part.Name},
		CallID:    tc.part.CallID, MessageID: tc.asstID,
		PreDecision: d, CreatedAt: e.clock(),
		CfgRules: tc.cfgRules,
	}
	decision, err := e.perm.Ask(tc.ctx, doomReq)
	if err != nil {
		e.failToolPart(tc, e.clock(), err.Error())
		return false
	}
	if decision != permission.Allow {
		msg := "permission rejected"
		if tc.ctx.Err() != nil {
			msg = "aborted"
		}
		now := e.clock()
		e.saveToolPart(tc, toolPart{
			callID: tc.part.CallID,
			name:   tc.part.Name,
			state: protocol.ToolState{
				Status:   "error",
				Input:    input,
				Error:    msg,
				Metadata: map[string]any{"reason": "doom_loop"},
				Time:     protocol.PartTime{Start: now, End: now},
			},
		})
		*tc.doomHist = append(*tc.doomHist, key)
		return false
	}
	*tc.doomHist = append(*tc.doomHist, key)
	return true
}

// gateExternal runs the external-directory gate on the tool's External
// paths outside the session dir (the part is "running" first so the TUI
// shows the pending state): one ask per outside directory pattern; an Ask
// error or a non-Allow decision finalizes the part via gateFail and stops
// the call.
func (e *Engine) gateExternal(tc *toolCall, tl tool.Tool, external []string) bool {
	for _, ext := range external {
		abs := ext
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(tc.row.ProjectDir, abs)
		}
		abs = filepath.Clean(abs)
		if inside, _ := withinDir(tc.row.ProjectDir, abs); inside {
			continue
		}
		pattern := filepath.Dir(abs) + "/*"
		d := e.perm.EvaluateRules(tc.agent, tc.cfgRules, "external_directory", []string{pattern})
		extReq := permission.Request{
			RequestID: protocol.NewID("perm"), SessionID: tc.sessionID, Agent: tc.agent,
			Permission: "external_directory", Tool: tl.ID(),
			Resources: []string{pattern},
			CallID:    tc.part.CallID, MessageID: tc.asstID,
			PreDecision: d, CreatedAt: e.clock(),
			CfgRules: tc.cfgRules,
		}
		decision, aerr := e.perm.Ask(tc.ctx, extReq)
		if aerr != nil || decision != permission.Allow {
			e.gateFail(tc)
			return false
		}
	}
	return true
}

// coreAsk runs the core permission ask with Resources/Always from
// tool.Patterns: an Ask error fails the part with the error text; a
// non-Allow decision finalizes the part via gateFail; both stop the call.
func (e *Engine) coreAsk(tc *toolCall, tl tool.Tool, resources, always []string) bool {
	d := e.perm.EvaluateRules(tc.agent, tc.cfgRules, tl.Permission(), resources)
	preq := permission.Request{
		RequestID: protocol.NewID("perm"), SessionID: tc.sessionID, Agent: tc.agent,
		Permission: tl.Permission(), Tool: tl.ID(),
		Resources: resources, Always: always,
		CallID: tc.part.CallID, MessageID: tc.asstID,
		PreDecision: d, CreatedAt: e.clock(),
		CfgRules: tc.cfgRules,
	}
	decision, err := e.perm.Ask(tc.ctx, preq)
	if err != nil {
		e.failToolPart(tc, e.clock(), err.Error())
		return false
	}
	if decision != permission.Allow {
		e.gateFail(tc)
		return false
	}
	return true
}

// gateFail finalizes the part for a failed permission gate (service
// error, deny, or ctx cancel while parked).
func (e *Engine) gateFail(tc *toolCall) {
	msg := "permission rejected"
	if tc.ctx.Err() != nil {
		msg = "aborted"
	}
	e.failToolPart(tc, e.clock(), msg)
}

// failToolPart finalizes the part as a hard error (invalid args, unknown
// tool, or a service failure): status "error" with the empty input, End
// stamped at the caller's stage.
func (e *Engine) failToolPart(tc *toolCall, stage int64, msg string) {
	e.saveToolPart(tc, toolPart{
		callID: tc.part.CallID,
		name:   tc.part.Name,
		state: protocol.ToolState{
			Status: "error",
			Input:  map[string]any{},
			Error:  msg,
			Time:   protocol.PartTime{Start: e.clock(), End: stage},
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

func (e *Engine) saveToolPart(tc *toolCall, tp toolPart) {
	p := protocol.Part{
		ID: tp.callID, SessionID: tc.sessionID, MessageID: tc.asstID,
		Type: "tool", Tool: tp.name, CallID: tp.callID, State: &tp.state,
	}
	row, perr := storage.ProtocolToPart(p)
	if perr != nil {
		e.lg.Error("persist part marshal failed", "part_id", p.ID, "session_id", tc.sessionID, "error", perr)
		return
	}
	// Finalization must land even when the turn ctx is cancelled (abort):
	// a cancelled ctx would drop the terminal tool-part write and leave the
	// part "running" in the store.
	if err := e.db.UpsertPart(context.WithoutCancel(tc.ctx), row); err != nil {
		e.lg.Error("persist part failed", "part_id", p.ID, "session_id", tc.sessionID, "error", err)
	}
	e.publish(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: tc.sessionID, Part: p, Time: e.clock(),
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
