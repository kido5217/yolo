package tui

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func TestRecallHistory(t *testing.T) {
	t.Run("up walks newest-first, down restores the draft", func(t *testing.T) {
		a := testApp()
		a.route = routeSession
		a.hist = []string{"a", "b", "c"} // c is the newest
		a.prompt.input.SetValue("draft")
		a.recallHistory(-1)
		if got := a.prompt.input.Value(); got != "c" {
			t.Fatalf("first up = %q, want c (newest)", got)
		}
		a.recallHistory(-1)
		if got := a.prompt.input.Value(); got != "b" {
			t.Fatalf("second up = %q, want b", got)
		}
		a.recallHistory(-1)
		if got := a.prompt.input.Value(); got != "a" {
			t.Fatalf("third up = %q, want a (oldest)", got)
		}
		a.recallHistory(-1) // beyond the oldest: no-op
		if got := a.prompt.input.Value(); got != "a" {
			t.Fatalf("fourth up must clamp at oldest, got %q", got)
		}
		a.recallHistory(1)
		if got := a.prompt.input.Value(); got != "b" {
			t.Fatalf("down = %q, want b", got)
		}
		a.recallHistory(1)
		if got := a.prompt.input.Value(); got != "c" {
			t.Fatalf("down = %q, want c", got)
		}
		a.recallHistory(1) // to present
		if got := a.prompt.input.Value(); got != "draft" {
			t.Fatalf("down to present = %q, want the captured draft", got)
		}
		a.recallHistory(1) // beyond present: no-op
		if got := a.prompt.input.Value(); got != "draft" {
			t.Fatalf("down beyond present must clamp, got %q", got)
		}
	})

	t.Run("dirty guard: an edited recall aborts nav", func(t *testing.T) {
		a := testApp()
		a.route = routeSession
		a.hist = []string{"a", "b"}
		a.prompt.input.SetValue("c")
		a.recallHistory(-1)           // up -> b
		a.prompt.input.SetValue("b2") // the user edits
		a.recallHistory(-1)           // dirty -> no-op
		if got := a.prompt.input.Value(); got != "b2" {
			t.Fatalf("dirty nav must not move, got %q", got)
		}
	})

	t.Run("empty history is a no-op", func(t *testing.T) {
		a := testApp()
		a.prompt.input.SetValue("x")
		a.recallHistory(-1)
		if got := a.prompt.input.Value(); got != "x" {
			t.Fatalf("empty history must not change the input, got %q", got)
		}
	})
}

func TestAppendHistory(t *testing.T) {
	t.Run("dedupes the newest and caps at 50", func(t *testing.T) {
		a := testApp()
		a.appendHistory("x")
		a.appendHistory("x") // duplicate: no add
		if len(a.hist) != 1 {
			t.Fatalf("dedupe: %d entries, want 1", len(a.hist))
		}
		for i := 0; i < 55; i++ {
			a.appendHistory(fmt.Sprintf("e%d", i))
		}
		if len(a.hist) != 50 {
			t.Fatalf("cap: %d entries, want 50", len(a.hist))
		}
		if a.hist[0] != "e5" || a.hist[49] != "e54" {
			t.Fatalf("cap dropped the wrong end: first=%q last=%q", a.hist[0], a.hist[49])
		}
	})
}

func TestPromptHistoryKey(t *testing.T) {
	t.Run("up/down on the session route recall (menu closed)", func(t *testing.T) {
		a := testApp()
		a.route = routeSession
		a.hist = []string{"one", "two"}
		a.handleKey(press(tea.KeyUp))
		if got := a.prompt.input.Value(); got != "two" {
			t.Fatalf("up = %q, want two (newest)", got)
		}
		a.handleKey(press(tea.KeyDown))
		if got := a.prompt.input.Value(); got != "" {
			t.Fatalf("down to present = %q, want empty (no draft captured)", got)
		}
	})
}

// TestTUIPromptHistoryRecall is the teatest leg: the real stack, a real
// session, the pre-seeded history is recalled through the real key pipeline.
func TestTUIPromptHistoryRecall(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	a.hist = []string{"alpha bravo"} // pre-seed the live app's history
	tm.Send(press(tea.KeyUp))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return a.prompt.input.Value() == "alpha bravo"
	}, teatest.WithDuration(5*time.Second))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
