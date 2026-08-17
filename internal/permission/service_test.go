package permission

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
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
	return &env{db: db, bus: b, svc: New(db, b)}
}

func (e *env) req(id string) Request {
	return Request{RequestID: id, SessionID: "ses_1", Agent: "build",
		Permission: "read", Tool: "read", Resources: []string{"src/x.go"}, Always: []string{"src/*"},
		CreatedAt: time.Now().UnixMilli()}
}

func TestAskPreAllowNoBlock(t *testing.T) {
	e := newEnv(t)
	if err := e.db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	req := e.req("per_1")
	req.DecisionPre = Allow
	done := make(chan Decision, 1)
	go func() { d, err := e.svc.Ask(context.Background(), req); if err == nil { done <- d } }()
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
	if err := e.db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	// force an ask: read src/x.go is allow by build base → use a deny-then pattern: use agent yolo? no.
	// Use permission action "custom" with no rule → always ask.
	req := e.req("per_2")
	req.Permission = "custom"
	done := make(chan Decision, 1)
	go func() { d, err := e.svc.Ask(context.Background(), req); if err == nil { done <- d } }()
	time.Sleep(100 * time.Millisecond) // let it park
	if pend, _ := e.svc.Pending("ses_1"); len(pend) != 1 {
		t.Fatalf("pending = %d", len(pend))
	}
	if err := e.svc.Reply("per_2", "once"); err != nil {
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
	_ = e.db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"})
	// two parked asks, same permission, second fully covered by first's always pattern
	r1 := e.req("per_3"); r1.Permission = "custom"; r1.Resources = []string{"a/b"}; r1.Always = []string{"a/*"}
	r2 := e.req("per_4"); r2.Permission = "custom"; r2.Resources = []string{"a/c"}
	go func() { _, _ = e.svc.Ask(context.Background(), r1) }()
	go func() { _, _ = e.svc.Ask(context.Background(), r2) }()
	time.Sleep(100 * time.Millisecond)
	if err := e.svc.Reply("per_3", "always"); err != nil {
		t.Fatal(err)
	}
	rules, err := e.db.AlwaysRules("ses_1")
	if err != nil || len(rules) != 1 || rules[0].Pattern != "a/*" {
		t.Fatalf("always rules = %+v err=%v", rules, err)
	}
	// r2 auto-answered: no longer pending
	if pend, _ := e.svc.Pending("ses_1"); len(pend) != 0 {
		t.Fatalf("pending after always = %d", len(pend))
	}
}

func TestRejectCascade(t *testing.T) {
	e := newEnv(t)
	_ = e.db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Agent: "build", Model: "k"})
	r1 := e.req("per_5"); r1.Permission = "custom"
	r2 := e.req("per_6"); r2.Permission = "custom"
	res1 := make(chan Decision, 1)
	res2 := make(chan Decision, 1)
	go func() { d, _ := e.svc.Ask(context.Background(), r1); res1 <- d }()
	go func() { d, _ := e.svc.Ask(context.Background(), r2); res2 <- d }()
	time.Sleep(100 * time.Millisecond)
	if err := e.svc.Reply("per_5", "reject"); err != nil {
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
