package session

import (
	"context"
	"slices"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

// RulesetFor assembles the session's permission ruleset in LOCKED order:
// agent builtins (unknown agents fall back to the build matrix) + config
// permission rules + the session's persisted always rules. The ruleset is
// re-read per round (tool visibility) and per tool call (hidden guard), so
// "always" replies apply from the very next evaluation. A config load error
// degrades to no config rules (the turn continues; config load is
// non-fatal, as in history replay).
func (e *Engine) RulesetFor(ctx context.Context, sessionID string) ([]protocol.Rule, error) {
	row, err := e.db.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return e.rulesetForRow(ctx, row)
}

// rulesetForRow is RulesetFor for an already-loaded session row.
func (e *Engine) rulesetForRow(ctx context.Context, row storage.SessionRow) ([]protocol.Rule, error) {
	agent := row.Agent
	if agent == "" {
		agent = "build"
	}
	// unknown (custom) agent: fall back to the build matrix (v1
	// custom-agent behavior) — the same fallback the service's decision
	// path uses (permission.BuiltinsFor).
	builtins := permission.BuiltinsFor(agent, e.dataDir)
	ruleset := make([]protocol.Rule, 0, len(builtins)+4)
	ruleset = append(ruleset, builtins...)
	if cfg, err := e.loadCfg(row.ProjectDir); err == nil && cfg != nil {
		// Invalid permission entries degrade to no config rules (config
		// load is non-fatal per turn).
		if perms, perr := protocol.ParsePerms(cfg.Permission); perr == nil {
			ruleset = append(ruleset, perms...)
		}
	}
	always, err := e.db.AlwaysRules(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return append(ruleset, always...), nil
}

// VisibleToolsFor returns the tools visible to the session's model under
// its ruleset (upstream disabled() semantics: a wildcard deny on "edit"
// hides both edit and write).
func (e *Engine) VisibleToolsFor(ctx context.Context, sessionID string) (map[string]tool.Tool, error) {
	rules, err := e.RulesetFor(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return tool.Visible(rules, e.tools), nil
}

// toolSchemaList renders the LLM tool schemas for the visible tools in
// stable (sorted) id order. Parameter bytes come from the schemas
// marshalled once at engine construction (encoding is deterministic, so
// wire bytes are identical to per-round marshalling).
func (e *Engine) toolSchemaList(ctx context.Context, sessionID string) ([]llm.ToolDef, error) {
	visible, err := e.VisibleToolsFor(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(visible))
	for id := range visible {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	tools := make([]llm.ToolDef, 0, len(ids))
	for _, id := range ids {
		t := visible[id]
		tools = append(tools, llm.ToolDef{Name: t.ID(), Description: t.Desc(), Parameters: e.schemas[id]})
	}
	return tools, nil
}
