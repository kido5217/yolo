package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// T28 "done when" gate: the three full blackbox suites. Each drives a real
// server + fake driver through the TUI keys only (home → n → type → enter).

func suiteType(tm *teatest.TestModel, s string) {
	for _, r := range s {
		tm.Send(press(r))
	}
}

// TestTUIFullTurn: home → n → type → streamed reasoning+text+tool rendered →
// alt+t reveals reasoning → alt+e expands the tool I/O → ctrl+c/y quit; the
// full sequence is asserted from the captured output stream (v2 teatest
// drains per WaitFor, deviation 50).
func TestTUIFullTurn(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "reasoning", Text: "let me think"},
			{Kind: "text", Text: "thinking now"},
			{
				Kind:   "tool",
				Name:   "read",
				CallID: "call_1",
				Args:   json.RawMessage(`{"filePath":"hello.txt"}`),
				Finish: "tool_calls",
			},
		}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "all done"}}},
	)
	ts := testutil.BootWithDriver(t, drv)
	if err := os.WriteFile(filepath.Join(ts.Dir, "hello.txt"), []byte("world\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "do it")
	tm.Send(press(tea.KeyEnter))

	var full string
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		full = stripANSI(string(b))
		return strings.Contains(full, "User: do it") &&
			strings.Contains(full, "thinking now") &&
			strings.Contains(full, "\u2713 read") &&
			strings.Contains(full, "all done")
	}, teatest.WithDuration(10*time.Second))

	// The full turn rendered in order: user echo, text, completed tool, final.
	idx := []int{
		strings.Index(full, "User: do it"),
		strings.Index(full, "thinking now"),
		strings.Index(full, "\u2713 read"),
		strings.Index(full, "all done"),
	}
	for i := range idx {
		if idx[i] < 0 || (i > 0 && idx[i] <= idx[i-1]) {
			t.Fatalf("turn sequence out of order (idx=%v):\n%s", idx, full)
		}
	}

	tm.Send(pressAlt('t')) // reveal reasoning
	tm.Send(pressAlt('e')) // expand the last tool part's I/O
	tm.Send(ctrlCKey)      // quit confirm
	tm.Send(press('y'))    // exit
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	tail, err := io.ReadAll(tm.Output())
	if err != nil {
		t.Fatalf("read final output: %v", err)
	}
	tsTail := stripANSI(string(tail))
	// "think" is asserted via its \u25BE marker and the expanded content:
	// the v2 renderer cell-diffs frames, and on the alt screen's fixed
	// frame the unchanged tail of the marker line is never re-emitted, so
	// the contiguous "▾ think" line cannot appear in the byte stream.
	for _, w := range []string{"\u25BE", "let me think", "world", "quit?", "[Y/n]"} {
		if !strings.Contains(tsTail, w) {
			t.Errorf("final output missing %q:\n%s", w, tsTail)
		}
	}
}

// permFlowHarness boots the bash-ask scenario and a home-routed app; the
// caller drives n → type → enter itself.
func permFlowHarness(t *testing.T) (*teatest.TestModel, *testutil.TestServer) {
	t.Helper()
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "text", Text: "working"},
			{Kind: "tool", Name: "bash", CallID: "call_1", Args: json.RawMessage(`{"command":"echo hi"}`), Finish: "tool_calls"},
		}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "all done"}}},
	)
	cfg := &protocol.Config{Permission: map[string]any{"bash": "ask"}}
	ts := testutil.BootWithDriverConfig(t, drv, cfg)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		// teatest's fake terminal is not a TTY, so profile detection yields
		// NoTTY and lipgloss strips every style. Pin an env that derives
		// ANSI256 from TERM alone so the locked red SGR renders.
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)
	return tm, ts
}

// hasLines requires every token within one WaitFor's accumulated polls
// (consecutive WaitFors drain each other and would starve on a quiet app).
func hasLines(tokens ...string) func([]byte) bool {
	return func(b []byte) bool {
		s := stripANSI(string(b))
		for _, tok := range tokens {
			if !strings.Contains(s, tok) {
				return false
			}
		}
		return true
	}
}

// hasPermDialogEcho matches this scenario's bash ask (the shared
// hasPermDialog pins T25's "ls *" pattern).
func hasPermDialogEcho(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "permission · bash") &&
		strings.Contains(s, "patterns: echo *") &&
		strings.Contains(s, "[1] once  [2] always  [3] reject")
}

func driveToPermDialog(t *testing.T, tm *teatest.TestModel, ts *testutil.TestServer) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "run it")
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), hasPermDialogEcho, teatest.WithDuration(5*time.Second))
	// The park lands on the engine's goroutine after the render; sync on it
	// before replying (same guard as TestPermissionDialogKeyReply).
	waitPending(t, ts, 1)
}

// TestTUIPermissionFlow: the locked bash ask replied with 1/2/3 in separate
// runs — allow (once/always) proceeds and completes; reject renders the tool
// error part (theme error token — SGR pinned by TestSessionChromeThemeSGR
// under the real engine) with the engine's locked "permission rejected" text
// (the plan's "forbidden" word deviates; deviation 56).
func TestTUIPermissionFlow(t *testing.T) {
	t.Run("once", func(t *testing.T) {
		tm, ts := permFlowHarness(t)
		driveToPermDialog(t, tm, ts)
		tm.Send(press('1'))
		// Zero-engine run: the completed row re-emits only its changed
		// icon cell (no static SGR to force a whole-line re-render), so
		// pin the ✓ icon + the final text (deviation 140).
		teatest.WaitFor(t, tm.Output(), hasLines("\u2713", "all done"), teatest.WithDuration(5*time.Second))
		_ = tm.Quit()
		tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	})
	t.Run("always", func(t *testing.T) {
		tm, ts := permFlowHarness(t)
		driveToPermDialog(t, tm, ts)
		tm.Send(press('2'))
		// Zero-engine run: the completed row re-emits only its changed
		// icon cell (no static SGR to force a whole-line re-render), so
		// pin the ✓ icon + the final text (deviation 140).
		teatest.WaitFor(t, tm.Output(), hasLines("\u2713", "all done"), teatest.WithDuration(5*time.Second))
		_ = tm.Quit()
		tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	})
	t.Run("reject", func(t *testing.T) {
		tm, ts := permFlowHarness(t)
		driveToPermDialog(t, tm, ts)
		tm.Send(press('3'))
		teatest.WaitFor(t, tm.Output(), hasLines("\u2717 bash", "permission rejected", "all done"), teatest.WithDuration(5*time.Second))
		_ = tm.Quit()
		tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	})
}

// TestTUIDialogs: model, agent, help and quit-confirm each scripted in one
// run; the sequence order is asserted over the accumulated drains.
func TestTUIDialogs(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	var seq strings.Builder
	capture := func(want ...string) {
		t.Helper()
		got := ""
		teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
			got = stripANSI(string(b))
			for _, w := range want {
				if !strings.Contains(got, w) {
					return false
				}
			}
			return true
		}, teatest.WithDuration(5*time.Second))
		seq.WriteString(got)
	}

	capture("New session")
	tm.Send(pressCtrlP())
	capture("Model", "Kido", "\u00B7 not-required", "\u25CB missing")
	tm.Send(press(tea.KeyEscape))
	tm.Send(pressCtrlA())
	capture("Agents", "build", "yolo", "Yolo agent. Permits everything")
	tm.Send(press(tea.KeyEscape))
	suiteType(tm, "/help")
	tm.Send(press(tea.KeyEnter))
	capture("Help", "| enter | send prompt |", "pgup/pgdn scroll \u00B7 \\+enter newline")
	tm.Send(press(tea.KeyEscape))
	tm.Send(ctrlCKey)
	capture("quit? [Y/n]")

	last := -1
	for _, w := range []string{"Model", "Agents", "Help", "quit? [Y/n]"} {
		i := strings.Index(seq.String(), w)
		if i < 0 || i <= last {
			t.Fatalf("dialog sequence out of order at %q (idx=%d, last=%d)\n%s", w, i, last, seq.String())
		}
		last = i
	}

	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestTUILongReplyWraps: a 1000-word single-line reply must be word-wrapped
// at the terminal width. The last word is only visible once the line wraps
// (the pre-wrap build clipped the line at the edge and no horizontal scroll
// exists, so w1000 never reached the screen); the viewport content must hold
// the full text with every line within the terminal width.
func TestTUILongReplyWraps(t *testing.T) {
	words := make([]string, 1000)
	for i := range words {
		words[i] = fmt.Sprintf("w%04d", i+1)
	}
	long := strings.Join(words, " ")
	drv := fake.New(fake.Turn{Parts: []llm.Part{{Kind: "text", Text: long, Finish: "stop"}}})
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "print 1000 words")
	tm.Send(press(tea.KeyEnter))

	teatest.WaitFor(t, tm.Output(), hasLine("w1000"), teatest.WithDuration(10*time.Second))

	var widest int
	for _, l := range strings.Split(a.sess.content, "\n") {
		if w := len([]rune(l)); w > widest {
			widest = w
		}
	}
	if widest > 80 {
		t.Fatalf("transcript content has lines wider than the terminal: %d > 80", widest)
	}
	// Wrapping reflows the text across lines; after re-joining the newlines
	// the full single-line text must be back (no word lost or clipped).
	if !strings.Contains(strings.ReplaceAll(a.sess.content, "\n", " "), long) {
		t.Fatal("transcript content lost text (clipped instead of wrapped)")
	}

	tm.Send(ctrlCKey)
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
