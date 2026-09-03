package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func attentionApp() *recApp {
	a := testApp()
	a.route = routeSession
	a.curSessionID = "s1"
	return a
}

func statusEv(t string) protocol.Event {
	ev, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{SessionID: "s1", Status: protocol.SessionStatus{Type: t}})
	return ev
}

func msgErrEv(typ string) protocol.Event {
	ev, _ := protocol.MakeEvent(protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "s1",
		Info:      protocol.Message{ID: "m1", Error: &protocol.MessageError{Type: typ, Message: "boom"}},
	})
	return ev
}

func TestAttentionBell(t *testing.T) {
	t.Run("done: idle after busy rings", func(t *testing.T) {
		a := attentionApp()
		if c := a.onAttention(statusEv("busy")); c != nil {
			t.Fatalf("busy must not ring, got %v", c)
		}
		if c := a.onAttention(statusEv("idle")); c == nil {
			t.Fatal("idle after busy must ring the done bell")
		}
	})
	t.Run("done: idle after an error does not ring", func(t *testing.T) {
		a := attentionApp()
		a.onAttention(statusEv("busy"))
		a.onAttention(msgErrEv("unknown"))
		if c := a.onAttention(statusEv("idle")); c != nil {
			t.Fatalf("idle after an error must not ring the done bell, got %v", c)
		}
	})
	t.Run("done: a second idle without a busy does not re-ring", func(t *testing.T) {
		a := attentionApp()
		a.onAttention(statusEv("busy"))
		if c := a.onAttention(statusEv("idle")); c == nil {
			t.Fatal("idle after busy must ring the done bell")
		}
		if a.attention.active {
			t.Fatal("idle must clear active")
		}
		if c := a.onAttention(statusEv("idle")); c != nil {
			t.Fatalf("a second idle without a busy must not re-ring, got %v", c)
		}
	})
	t.Run("error: an errored idle clears the flags so the next turn rings", func(t *testing.T) {
		a := attentionApp()
		a.onAttention(statusEv("busy"))
		a.onAttention(msgErrEv("unknown"))
		if c := a.onAttention(statusEv("idle")); c != nil {
			t.Fatalf("idle after an error must not ring the done bell, got %v", c)
		}
		if a.attention.errored {
			t.Fatal("an errored idle must clear errored (the upstream idle handler deletes it)")
		}
		if c := a.onAttention(statusEv("busy")); c != nil {
			t.Fatalf("busy must not ring, got %v", c)
		}
		if c := a.onAttention(statusEv("idle")); c == nil {
			t.Fatal("idle after the next busy must ring the done bell")
		}
	})
	t.Run("error: the current message error rings", func(t *testing.T) {
		a := attentionApp()
		if c := a.onAttention(msgErrEv("unknown")); c == nil {
			t.Fatal("a turn error must ring")
		}
	})
	t.Run("permission ask rings, deduped by id", func(t *testing.T) {
		a := attentionApp()
		ev, _ := protocol.MakeEvent(protocol.EventTypePermissionAsked, protocol.PermissionAskedProps{ID: "p1", SessionID: "s1", Permission: "bash"})
		if c := a.onAttention(ev); c == nil {
			t.Fatal("a permission ask must ring")
		}
		ev2, _ := protocol.MakeEvent(protocol.EventTypePermissionAsked, protocol.PermissionAskedProps{ID: "p1", SessionID: "s1", Permission: "bash"})
		if c := a.onAttention(ev2); c != nil {
			t.Fatalf("a duplicate permission ask must not re-ring, got %v", c)
		}
	})
	t.Run("a non-current session's status is ignored", func(t *testing.T) {
		a := attentionApp()
		ev, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{SessionID: "other", Status: protocol.SessionStatus{Type: "busy"}})
		a.onAttention(ev)
		if a.attention.active {
			t.Fatal("a non-current session's busy must not set active")
		}
	})
}

// TestTUIAttentionBell is the teatest leg: a real turn completes -> the
// session goes busy then idle -> the done bell (tea.Raw("\a")) lands in the
// captured output.
func TestTUIAttentionBell(t *testing.T) {
	drv := fake.New(fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop"}}})
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "hello")
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(stripANSI(string(b)), "done") && bytes.Contains(b, []byte{0x07})
	}, teatest.WithDuration(10*time.Second))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
