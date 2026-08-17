package permission

import (
	"fmt"

	"github.com/kido5217/yolo/internal/protocol"
)

// base is the shared rule matrix. Order is significant (findLast): broad
// rules first, narrow rules later.
//
// PLAN FIX (Task 10 owns this decision): upstream's third plan "edit" rule
// is worktree-relative (rel(worktree, <dataDir>/plans/*.md)), which is
// session-dependent. LoadBuiltins only knows dataDir, so the plan matrix
// carries the two absolute plan rules; the engine adds the worktree-relative
// rule at session start: {edit, path.Rel(sessionDir, dataDir)/"plans/*.md", allow}.
var base = []protocol.Rule{
	{Permission: "*", Pattern: "*", Action: RuleAllow},
	{Permission: "doom_loop", Pattern: "*", Action: RuleAsk},
	{Permission: "external_directory", Pattern: "*", Action: RuleAsk},
	{Permission: "question", Pattern: "*", Action: RuleDeny},
	{Permission: "plan_enter", Pattern: "*", Action: RuleDeny},
	{Permission: "plan_exit", Pattern: "*", Action: RuleDeny},
	{Permission: "read", Pattern: "*", Action: RuleAllow},
	{Permission: "read", Pattern: "*.env", Action: RuleAsk},
	{Permission: "read", Pattern: "*.env.*", Action: RuleAsk},
	{Permission: "read", Pattern: "*.env.example", Action: RuleAllow},
}

// LoadBuiltins returns the ruleset for a built-in agent. "yolo" allows
// everything with a single catch-all rule.
func LoadBuiltins(agent, dataDir string) ([]protocol.Rule, error) {
	switch agent {
	case "build":
		return cloneBase(base,
			protocol.Rule{Permission: "question", Pattern: "*", Action: RuleAllow},
			protocol.Rule{Permission: "plan_enter", Pattern: "*", Action: RuleAllow},
		), nil
	case "plan":
		return cloneBase(base,
			protocol.Rule{Permission: "question", Pattern: "*", Action: RuleAllow},
			protocol.Rule{Permission: "plan_exit", Pattern: "*", Action: RuleAllow},
			protocol.Rule{Permission: "external_directory", Pattern: dataDir + "/plans/*", Action: RuleAllow},
			protocol.Rule{Permission: "edit", Pattern: "*", Action: RuleDeny},
			protocol.Rule{Permission: "edit", Pattern: dataDir + "/plans/*.md", Action: RuleAllow},
		), nil
	case "yolo":
		return []protocol.Rule{{Permission: "*", Pattern: "*", Action: RuleAllow}}, nil
	default:
		return nil, fmt.Errorf("permission: unknown agent %q", agent)
	}
}

func cloneBase(src []protocol.Rule, extra ...protocol.Rule) []protocol.Rule {
	out := make([]protocol.Rule, 0, len(src)+len(extra))
	out = append(out, src...)
	out = append(out, extra...)
	return out
}

// corePermissions are the permission actions the engine can emit (tool
// permissions plus the special ones). They define the scope of the catch-all
// "*" permission rule in the DECISION path (DecisionFor): unknown actions
// (e.g. custom/test actions) have no rule and default to ask, matching
// upstream's behavior where a ruleset without a matching rule asks.
var corePermissions = map[string]bool{
	"read": true, "write": true, "edit": true, "bash": true, "glob": true,
	"grep": true, "webfetch": true, "task": true, "todo": true, "question": true,
	"doom_loop": true, "external_directory": true, "plan_enter": true, "plan_exit": true,
}
