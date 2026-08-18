package permission

import (
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
)

func r(perm, pattern, effect string) protocol.Rule {
	return protocol.Rule{Permission: perm, Pattern: pattern, Action: effect}
}

func TestEvaluateFindLastNoMatchAsk(t *testing.T) {
	rules := []protocol.Rule{r("*", "*", "allow"), r("read", "*.env", "ask")}
	if got := Evaluate(rules, "read", []string{"a.env"}); got != AskAction {
		t.Fatalf("got %v", got)
	}
	if got := Evaluate(rules, "read", []string{"a.go"}); got != Allow {
		t.Fatalf("got %v", got)
	}
	if got := Evaluate([]protocol.Rule{}, "bash", []string{"ls *"}); got != AskAction {
		t.Fatalf("no rule → ask, got %v", got)
	}
}

func TestMultiResourceAnyDenyWins(t *testing.T) {
	rules := []protocol.Rule{r("*", "*", "allow"), r("edit", "secrets/*", "deny")}
	if got := Evaluate(rules, "edit", []string{"secrets/a", "ok/b"}); got != Deny {
		t.Fatalf("got %v", got)
	}
}

func TestHiddenWildcardDenyLastWins(t *testing.T) {
	// build: no edit rule → findLast("*") = allow → not hidden
	hidden := Hidden(base, []string{"edit", "write", "bash"})
	if hidden["edit"] || hidden["write"] || hidden["bash"] {
		t.Fatalf("build hides: %v", hidden)
	}
	// plan: edit rules appended (deny * then allow data/plans/*.md) → LAST is allow → NOT hidden (upstream semantics)
	planRules := append(append([]protocol.Rule{}, base...),
		r("plan_exit", "*", "allow"),
		r("edit", "*", "deny"),
		r("edit", "/data/plans/*.md", "allow"))
	if Hidden(planRules, []string{"edit"})["edit"] {
		t.Fatal("plan edit must stay visible (last rule is allow)")
	}
	// a ruleset ending in wildcard deny hides edit AND write (edit-permission mapping)
	denied := append(append([]protocol.Rule{}, base...), r("edit", "*", "deny"))
	h := Hidden(denied, []string{"edit", "write", "bash"})
	if !h["edit"] || !h["write"] || h["bash"] {
		t.Fatalf("hidden = %v", h)
	}
}

func TestBuiltinsYoloAllowsEverything(t *testing.T) {
	rules, err := LoadBuiltins("yolo", "/data")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"read", "write", "edit", "bash", "doom_loop", "question"} {
		if got := Evaluate(rules, action, []string{"anything"}); got != Allow {
			t.Fatalf("yolo %s → %v", action, got)
		}
	}
}

func TestBuiltinsPlanDeniesEditUnlessPlanPath(t *testing.T) {
	rules, err := LoadBuiltins("plan", "/data")
	if err != nil {
		t.Fatal(err)
	}
	if got := Evaluate(rules, "edit", []string{"src/main.go"}); got != Deny {
		t.Fatalf("plan edit src → %v", got)
	}
	if got := Evaluate(rules, "edit", []string{"/data/plans/x.md"}); got != Allow {
		t.Fatalf("plan edit plans → %v", got)
	}
	if got := Evaluate(rules, "plan_exit", []string{"*"}); got != Allow {
		t.Fatalf("plan plan_exit → %v", got)
	}
	if got := Evaluate(rules, "bash", []string{"git *"}); got != Allow {
		t.Fatalf("plan bash → %v (base * allow)", got)
	}
}

func TestDoomLoopDue(t *testing.T) {
	k := func() CallKey { return CallKey{"bash", "abc"} }
	if DoomLoopDue(nil, k()) || DoomLoopDue([]CallKey{k()}, k()) {
		t.Fatal("too early")
	}
	if !DoomLoopDue([]CallKey{k(), k()}, k()) {
		t.Fatal("third consecutive identical must fire")
	}
}
