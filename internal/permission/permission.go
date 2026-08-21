// Package permission ports opencode's permission engine: rule evaluation
// (findLast semantics), the build/plan/yolo matrices, doom-loop detection,
// and the blocking ask/reply service.
package permission

import (
	"github.com/kido5217/yolo/internal/glob"
	"github.com/kido5217/yolo/internal/protocol"
)

type Action string

const (
	Allow Action = "allow"
	Deny  Action = "denied"
	Ask   Action = "ask"
)

// Decision is an evaluation outcome.
type Decision = Action

// Rule vocabulary: values stored on protocol.Rule.Action (config/wire form).
// Note "deny" here vs the Decision constant Deny = "denied".
const (
	RuleAllow = "allow"
	RuleDeny  = "deny"
	RuleAsk   = "ask"
)

// Evaluate applies the ruleset to (action, resources) with findLast
// semantics: for each resource, the LAST rule whose permission matches
// ("*" or exact) and whose pattern glob-matches the resource decides it.
// Any resource deciding deny -> Deny; else any deciding ask (or no matching
// rule at all) -> Ask; else Allow.
func Evaluate(rules []protocol.Rule, action string, resources []string) Decision {
	anyAsk := false
	for _, res := range resources {
		last := findLast(rules, action, res)
		if last == nil {
			anyAsk = true
			continue
		}
		switch last.Action {
		case RuleDeny:
			return Deny
		case RuleAsk:
			anyAsk = true
		}
	}
	if anyAsk {
		return Ask
	}
	return Allow
}

// findLast returns the last rule matching action (permission exact or "*")
// and resource (glob), or nil.
func findLast(rules []protocol.Rule, action, res string) *protocol.Rule {
	return findLastWithWildcard(rules, action, res, true)
}

// findLastWithWildcard is findLast with an optional gate on the "*"
// permission rule (used by the decision path; see decisionFor).
func findLastWithWildcard(rules []protocol.Rule, action, res string, wildcardOK bool) *protocol.Rule {
	for i := len(rules) - 1; i >= 0; i-- {
		r := &rules[i]
		if r.Permission != action && (r.Permission != "*" || !wildcardOK) {
			continue
		}
		if !glob.Match(r.Pattern, res) {
			continue
		}
		return r
	}
	return nil
}

// Hidden reports which tools are hidden from the TUI, mirroring upstream
// disabled(): the tool's permission is "edit" for write tools, otherwise the
// tool name itself; hidden iff the LAST rule for that permission (no
// resource matching) has Pattern "*" and Action deny.
func Hidden(rules []protocol.Rule, tools []string) map[string]bool {
	out := map[string]bool{}
	for _, tool := range tools {
		perm := tool
		if tool == "edit" || tool == "write" {
			perm = "edit"
		}
		hidden := false
		for i := len(rules) - 1; i >= 0; i-- {
			r := &rules[i]
			if r.Permission != perm && r.Permission != "*" {
				continue
			}
			hidden = r.Pattern == "*" && r.Action == RuleDeny
			break
		}
		out[tool] = hidden
	}
	return out
}

// CallKey identifies a tool call for doom-loop detection: Hash is the sha256
// hex of canonical JSON args.
type CallKey struct{ Tool, Hash string }

// DoomLoopDue reports whether the next call would be the third consecutive
// identical call (last two of history == next).
func DoomLoopDue(history []CallKey, next CallKey) bool {
	n := len(history)
	if n < 2 {
		return false
	}
	return history[n-1] == next && history[n-2] == next
}
