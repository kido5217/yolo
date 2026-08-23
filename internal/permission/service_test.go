package permission

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

type env struct {
	db  *storage.DB
	bus *bus.Bus
	svc *Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	b := bus.New()
	return &env{db: db, bus: b, svc: New(db, b, nil, "")}
}

// awaitPending polls Pending until it holds exactly want entries, failing on
// a 2s deadline — a deterministic park-wait (fixed sleeps flake under load).
func (e *env) awaitPending(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pend, _ := e.svc.Pending(t.Context(), "ses_1"); len(pend) == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("pending did not reach %d entries", want)
}

func (e *env) req(id string) Request {
	return Request{RequestID: id, SessionID: "ses_1", Agent: "build",
		Permission: "read", Tool: "read", Resources: []string{"src/x.go"}, Always: []string{"src/*"},
		CreatedAt: time.Now().UnixMilli()}
}

func TestAskPreAllowNoBlock(t *testing.T) {
	e := newEnv(t)
	if err := e.db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	req := e.req("per_1")
	req.PreDecision = Allow
	done := make(chan Decision, 1)
	go func() {
		d, err := e.svc.Ask(context.Background(), req)
		if err == nil {
			done <- d
		}
	}()
	select {
	case d := <-done:
		if d != Allow {
			t.Fatalf("d = %v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pre-allow must not block")
	}
}

func TestAskAskBlocksThenOnce(t *testing.T) {
	e := newEnv(t)
	if err := e.db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	// force an ask: read src/x.go is allow by build base → use a deny-then pattern: use agent yolo? no.
	// Use permission action "custom" with no rule → always ask.
	req := e.req("per_2")
	req.Permission = "custom"
	done := make(chan Decision, 1)
	go func() {
		d, err := e.svc.Ask(context.Background(), req)
		if err == nil {
			done <- d
		}
	}()
	e.awaitPending(t, 1)
	if pend, _ := e.svc.Pending(t.Context(), "ses_1"); len(pend) != 1 {
		t.Fatalf("pending = %d", len(pend))
	}
	if err := e.svc.Reply(t.Context(), "per_2", "once"); err != nil {
		t.Fatal(err)
	}
	select {
	case d := <-done:
		if d != Allow {
			t.Fatalf("d = %v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reply did not unblock")
	}
}

func TestAlwaysPersistsRuleAndCoveredAutoAnswer(t *testing.T) {
	e := newEnv(t)
	_ = e.db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"})
	// two parked asks, same permission, second fully covered by first's always pattern
	r1 := e.req("per_3")
	r1.Permission = "custom"
	r1.Resources = []string{"a/b"}
	r1.Always = []string{"a/*"}
	r2 := e.req("per_4")
	r2.Permission = "custom"
	r2.Resources = []string{"a/c"}
	go func() { _, _ = e.svc.Ask(context.Background(), r1) }()
	go func() { _, _ = e.svc.Ask(context.Background(), r2) }()
	e.awaitPending(t, 2)
	if err := e.svc.Reply(t.Context(), "per_3", "always"); err != nil {
		t.Fatal(err)
	}
	rules, err := e.db.AlwaysRules(t.Context(), "ses_1")
	if err != nil || len(rules) != 1 || rules[0].Pattern != "a/*" {
		t.Fatalf("always rules = %+v err=%v", rules, err)
	}
	// r2 auto-answered: no longer pending
	if pend, _ := e.svc.Pending(t.Context(), "ses_1"); len(pend) != 0 {
		t.Fatalf("pending after always = %d", len(pend))
	}
}

func TestReplyUnblocksAndPublishesWhenPersistFails(t *testing.T) {
	e := newEnv(t)
	if err := e.db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	sub, cancel := e.bus.Subscribe()
	defer cancel()
	req := e.req("per_9")
	req.Permission = "custom" // no rule matches -> parks
	done := make(chan Decision, 1)
	go func() {
		d, err := e.svc.Ask(context.Background(), req)
		if err == nil {
			done <- d
		}
	}()
	e.awaitPending(t, 1)
	// Simulate a transient DB failure at reply-persist time: close the
	// handle, so ReplyPermission returns a non-ErrNotFound error.
	if err := e.db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Reply(t.Context(), "per_9", "once"); err != nil {
		t.Fatal(err)
	}
	// The decision must still reach the blocked asker (a DB write failure
	// must not hang the turn).
	select {
	case d := <-done:
		if d != Allow {
			t.Fatalf("d = %v, want Allow", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked ask not unblocked after persist failure (turn would hang)")
	}
	// ... and the permission.replied event must still be published.
	for {
		select {
		case ev := <-sub:
			if ev.Type == protocol.EventTypePermissionReplied {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("permission.replied not published after persist failure")
		}
	}
}

func TestCustomAgentFallsBackToBuildMatrix(t *testing.T) {
	e := newEnv(t)
	if err := e.db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	// A config-defined custom agent evaluates against the build matrix
	// (same as the engine's ruleset path), not an empty rule set: "read"
	// of a plain source file is build-allow.
	req := e.req("per_10")
	req.Permission = "read"
	req.Resources = []string{"src/x.go"}
	req.Agent = "custom"
	if d := e.svc.DecisionFor(t.Context(), req); d != Allow {
		t.Fatalf("custom-agent DecisionFor = %v, want Allow (build matrix)", d)
	}
	if d := e.svc.EvaluateRules("custom", nil, "read", []string{"src/x.go"}); d != Allow {
		t.Fatalf("custom-agent EvaluateRules = %v, want Allow (build matrix)", d)
	}
}

func TestRejectCascade(t *testing.T) {
	e := newEnv(t)
	_ = e.db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"})
	r1 := e.req("per_5")
	r1.Permission = "custom"
	r2 := e.req("per_6")
	r2.Permission = "custom"
	res1 := make(chan Decision, 1)
	res2 := make(chan Decision, 1)
	go func() { d, _ := e.svc.Ask(context.Background(), r1); res1 <- d }()
	go func() { d, _ := e.svc.Ask(context.Background(), r2); res2 <- d }()
	e.awaitPending(t, 2)
	if err := e.svc.Reply(t.Context(), "per_5", "reject"); err != nil {
		t.Fatal(err)
	}
	for i, ch := range []chan Decision{res1, res2} {
		select {
		case d := <-ch:
			if d != Deny {
				t.Fatalf("cascade result %d = %v", i, d)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("cascade did not unblock %d", i)
		}
	}
}

// TestDecisionForUsesRequestCfgRules pins ⑧: config rules are a per-request
// parameter (no shared service state). The same service, two different rule
// sets in the same request stream — each request is evaluated against its
// own rules, so a later request cannot bleed into an earlier evaluation.
func TestDecisionForUsesRequestCfgRules(t *testing.T) {
	e := newEnv(t)
	denyRead := []protocol.Rule{{Permission: "read", Pattern: "/a/*", Action: RuleDeny}}
	allowBash := []protocol.Rule{{Permission: "bash", Pattern: "*", Action: RuleAllow}}

	// Deterministic single-shot checks: each request is evaluated against
	// its OWN rules (no shared service state to bleed through).
	reqA := Request{SessionID: "s1", Agent: "build", Permission: "read",
		Resources: []string{"/a/x.txt"}, CfgRules: denyRead}
	reqB := Request{SessionID: "s2", Agent: "build", Permission: "bash",
		Resources: []string{"git *"}, CfgRules: allowBash}
	if d := e.svc.decisionFor(t.Context(), reqA); d != Deny {
		t.Fatalf("reqA = %v, want Deny (its own cfg rules)", d)
	}
	if d := e.svc.decisionFor(t.Context(), reqB); d != Allow {
		t.Fatalf("reqB = %v, want Allow (its own cfg rules)", d)
	}

	// Concurrent (spec: "concurrent sessions with different project rules
	// -> no cross-contamination"): many evaluations interleave the two rule
	// sets on the SAME service; each keeps its own verdict. Guards that a
	// future re-introduction of shared per-request state stays -race clean
	// and does not change per-request verdicts. Run under -race.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if d := e.svc.decisionFor(t.Context(), reqA); d != Deny {
				t.Errorf("concurrent reqA = %v, want Deny", d)
			}
		}()
		go func() {
			defer wg.Done()
			if d := e.svc.decisionFor(t.Context(), reqB); d != Allow {
				t.Errorf("concurrent reqB = %v, want Allow", d)
			}
		}()
	}
	wg.Wait()
}

// TestResolveSettlesExactlyOnce pins ② double-settle: two concurrent
// resolve() calls for the same parked request (the Reply "once" vs a
// ctx-cancel "aborted" — both funnel through resolve) must settle exactly
// once: one permission.replied event, one row flip. Only the remover
// settles; the lost claim is a no-op.
func TestResolveSettlesExactlyOnce(t *testing.T) {
	e := newEnv(t)
	if err := e.db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	sub, cancel := e.bus.Subscribe()
	defer cancel()
	req := e.req("per_ds")
	req.Permission = "custom" // no rule -> parks
	done := make(chan Decision, 1)
	go func() {
		d, err := e.svc.Ask(context.Background(), req)
		if err == nil {
			done <- d
		}
	}()
	e.awaitPending(t, 1)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); e.svc.resolve(t.Context(), "per_ds", Allow, "once", "once", false) }()
	go func() { defer wg.Done(); e.svc.resolve(t.Context(), "per_ds", Deny, "aborted", "reject", false) }()
	wg.Wait()

	replied := 0
	drain := time.After(300 * time.Millisecond)
drain:
	for {
		select {
		case ev := <-sub:
			if ev.Type == protocol.EventTypePermissionReplied {
				replied++
			}
		case <-drain:
			break drain
		}
	}
	if replied != 1 {
		t.Fatalf("permission.replied events = %d, want exactly 1 (② double-settle)", replied)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("asker not unblocked by the winning settle")
	}
}
