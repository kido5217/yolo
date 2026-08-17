package session

import (
	"encoding/json"
	"sort"

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
func (e *Engine) RulesetFor(sessionID string) ([]protocol.Rule, error) {
	row, err := e.db.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	return e.rulesetForRow(row)
}

// rulesetForRow is RulesetFor for an already-loaded session row.
func (e *Engine) rulesetForRow(row storage.SessionRow) ([]protocol.Rule, error) {
	agent := row.Agent
	if agent == "" {
		agent = "build"
	}
	builtins, err := permission.LoadBuiltins(agent, e.dataDir)
	if err != nil {
		// unknown (custom) agent: fall back to the build matrix (v1
		// custom-agent behavior)
		builtins, err = permission.LoadBuiltins("build", e.dataDir)
		if err != nil {
			return nil, err
		}
	}
	ruleset := make([]protocol.Rule, 0, len(builtins)+4)
	ruleset = append(ruleset, builtins...)
	if cfg, err := e.loadCfg(row.ProjectDir); err == nil && cfg != nil {
		ruleset = append(ruleset, protocol.ParsePerms(cfg.Permission)...)
	}
	always, err := e.db.AlwaysRules(row.ID)
	if err != nil {
		return nil, err
	}
	return append(ruleset, always...), nil
}

// VisibleToolsFor returns the tools visible to the session's model under
// its ruleset (upstream disabled() semantics: a wildcard deny on "edit"
// hides both edit and write).
func (e *Engine) VisibleToolsFor(sessionID string) (map[string]tool.Tool, error) {
	rules, err := e.RulesetFor(sessionID)
	if err != nil {
		return nil, err
	}
	return tool.Visible(rules, e.tools), nil
}

// toolSchemaList renders the LLM tool schemas for the visible tools in
// stable (sorted) id order.
func (e *Engine) toolSchemaList(sessionID string) ([]llm.ToolDef, error) {
	visible, err := e.VisibleToolsFor(sessionID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(visible))
	for id := range visible {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	tools := make([]llm.ToolDef, 0, len(ids))
	for _, id := range ids {
		t := visible[id]
		params, err := json.Marshal(t.Schema())
		if err != nil {
			return nil, err
		}
		tools = append(tools, llm.ToolDef{Name: t.ID(), Description: t.Desc(), Parameters: params})
	}
	return tools, nil
}
