package tui

import (
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// partEv builds a message.part.updated event carrying a tool part (the
// 0.8.0 VCS re-read fixture; local-only, no network).
func partEv(tool, status, sessionID string) protocol.Event {
	ev, _ := protocol.MakeEvent(protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
		SessionID: sessionID,
		Part: protocol.Part{
			ID:        "p1",
			SessionID: sessionID,
			MessageID: "m1",
			Type:      "tool",
			Tool:      tool,
			CallID:    "p1",
			State:     &protocol.ToolState{Status: status, Input: map[string]any{"command": "git checkout x"}},
		},
		Time: testNow,
	})
	return ev
}

// TestBranchMsgApplyGuard pins the stale-fetch race guard (decision 4):
// the msg carries the scope dir AT LAUNCH, so a fetch racing a scope
// change is dropped and a matching-dir fetch lands.
func TestBranchMsgApplyGuard(t *testing.T) {
	a := testApp()
	dirA := t.TempDir()
	dirB := t.TempDir()

	t.Run("matching dir lands", func(t *testing.T) {
		a.Service.Dir = dirA
		a.updateMsg(branchMsg{dir: dirA, branch: "landed"})
		if a.branch != "landed" {
			t.Fatalf("a.branch = %q, want %q (the matching-dir fetch must land)", a.branch, "landed")
		}
	})
	t.Run("stale dir dropped", func(t *testing.T) {
		a.branch = "prev"
		a.Service.Dir = dirA
		cmd := a.branchCmd() // launched against dirA
		a.Service.Dir = dirB // the scope changes while the fetch is in flight
		msg, ok := cmd().(branchMsg)
		if !ok {
			t.Fatalf("branchCmd delivered %T, want branchMsg", msg)
		}
		if msg.dir != dirA {
			t.Fatalf("the msg carries %q, want the launch-time dir %q", msg.dir, dirA)
		}
		a.updateMsg(msg)
		if a.branch != "prev" {
			t.Fatalf("a.branch = %q, want %q (the stale-dir fetch must be dropped)", a.branch, "prev")
		}
	})
}

// TestEnterHomeArmsBranchReRead pins the home-entry hook: it re-rolls the
// tip index (the upstream per-mount re-roll — asserted via the tipRand
// seam) and returns the branch re-read cmd for the caller to batch.
func TestEnterHomeArmsBranchReRead(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.tipRand = func() float64 { return 0.5 }
	before := a.tipIdx
	cmd := a.enterHome()
	if cmd == nil {
		t.Fatal("enterHome must return the branch re-read cmd (callers batch it)")
	}
	want := int(0.5 * float64(len(tips)))
	if a.tipIdx != want {
		t.Fatalf("tipIdx = %d (before %d), want %d (the seeded re-roll)", a.tipIdx, before, want)
	}
}

// TestBranchReReadEventHook pins the decision-4 bash-part-completion
// re-read: only a COMPLETED bash tool part on the CURRENT session arms the
// re-read cmd (a shell-mode submit reuses the bash tool — covered); every
// other part shape is a no-op (no extra cmd beyond the pump).
func TestBranchReReadEventHook(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.route = routeSession
	a.curSessionID = "s1"

	cases := []struct {
		name string
		ev   protocol.Event
		want bool
	}{
		{"a completed bash part on the current session", partEv("bash", "completed", "s1"), true},
		{"a write part", partEv("write", "completed", "s1"), false},
		{"another session's bash part", partEv("bash", "completed", "other"), false},
		{"an incomplete bash part", partEv("bash", "running", "s1"), false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.branchReRead(tt.ev); (got != nil) != tt.want {
				t.Fatalf("branchReRead = %v, want nil=%v", got, !tt.want)
			}
		})
	}

	// the plan's literal pin: the hook fires through updateMsg, which
	// returns the applied event's cmd (pump + the batched re-read).
	if cmd := a.updateMsg(EventMsg{Event: partEv("bash", "completed", "s1")}); cmd == nil {
		t.Fatal("updateMsg must return a cmd for a completed bash part on the current session")
	}
}

// TestBranchCmdIntegration is the real-stack leg (local git, no network):
// a repo at the server scope dir; the bootstrap branchCmd resolves the
// branch and the apply guard lands it on the app.
func TestBranchCmdIntegration(t *testing.T) {
	ts := testutil.Boot(t)
	runGit(t, ts.Dir, "init", "-q", "-b", "yolo-wire-branch")
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)

	cmd := a.branchCmd()
	msg, ok := cmd().(branchMsg)
	if !ok {
		t.Fatalf("branchCmd delivered %T, want branchMsg", msg)
	}
	if msg.dir != ts.Dir {
		t.Fatalf("msg.dir = %q, want the scope dir %q", msg.dir, ts.Dir)
	}
	if _, c := a.Update(msg); c != nil {
		t.Fatalf("the branchMsg apply returned cmd %v, want nil", c)
	}
	if a.branch != "yolo-wire-branch" {
		t.Fatalf("a.branch = %q, want %q", a.branch, "yolo-wire-branch")
	}
}
