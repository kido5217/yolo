package tui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// stripANSITest removes SGR color sequences from raw teatest output so the
// assertions can match the visible text (the ✓ tool row is split across two
// styled spans, so the raw bytes never contain "✓ read" contiguously).
var sgrTestRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSITest(b []byte) string { return sgrTestRe.ReplaceAllString(string(b), "") }

func TestHomeRendersListAndNewSession(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := tui.NewApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Yolo")) &&
			bytes.Contains(b, []byte("New session"))
	}, teatest.WithDuration(5*time.Second))

	ctx := context.Background()
	ses, err := c.CreateSession(ctx, "Hello")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if ses.ID == "" {
		t.Fatal("CreateSession returned no id")
	}

	tm.Send(tui.HydrateMsg{})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSITest(b)
		return strings.Contains(s, "Hello \u00B7 kido/q")
	}, teatest.WithDuration(5*time.Second))

	// The output stream is consumed by the WaitFor calls above (v2 teatest);
	// the two WaitFors are the locked assertions for this test.
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestResumeMissingSessionExitsWithError(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := tui.NewApp(c, store.State{}, "ses_missing")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	out, _ := io.ReadAll(tm.Output())
	if !bytes.Contains(out, []byte("session not found")) {
		t.Fatalf("final output missing \"session not found\":\n%s", out)
	}
}

// TestSessionStreamingViewport is the T24 blackbox check: a real server with
// a scripted fake driver (turn 1: text + read tool, turn 2: text) drives the
// session route over SSE; the viewport must show the completed tool row and
// the final streamed text on its last line. The 80x10 terminal gives the
// viewport 7 rows, enough to hold the whole 6-line transcript, so the
// completed-tool row is asserted deterministically (bubbletea coalesces view
// changes between 60fps ticks, so a smaller terminal could scroll past it).
func TestSessionStreamingViewport(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "text", Text: "thinking"},
			{
				Kind:   "tool",
				Name:   "read",
				CallID: "call_1",
				Args:   json.RawMessage(`{"filePath":"hello.txt"}`),
				Finish: "tool_calls",
			},
		}},
		fake.Turn{Parts: []llm.Part{
			{Kind: "text", Text: "done", Finish: "stop"},
		}},
	)
	ts := testutil.BootWithDriver(t, drv)
	if err := os.WriteFile(filepath.Join(ts.Dir, "hello.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	c := client.New(ts.URL, ts.Dir)
	ctx := context.Background()
	ses, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := tui.NewApp(c, store.State{}, ses.ID)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 10))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("New session"))
	}, teatest.WithDuration(5*time.Second))

	if _, err := c.SendMessage(ctx, ses.ID, "do it"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := []byte(stripANSITest(b))
		return bytes.Contains(s, []byte("\u2713 read")) && bytes.Contains(s, []byte("done"))
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// keyMsg builds a synthetic keypress (teatest v2 Send takes tea.Msg directly,
// bypassing the terminal string parser).
func keyMsg(r rune) tea.KeyPressMsg {
	switch r {
	case tea.KeyUp, tea.KeyDown, tea.KeyEnter, tea.KeyEscape:
		return tea.KeyPressMsg{Code: r}
	}
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func typeIn(tm *teatest.TestModel, s string) {
	for _, r := range s {
		tm.Send(keyMsg(r))
	}
}

// TestPromptSendAndSlashMenu is the T25 blackbox check: the prompt line is a
// real client path — typing "hello"+enter must land the message on the server
// (asserted via testutil.LastMessages), "/m" must open the slash menu filtered
// to /model, and enter routes to the (T28-stub) model dialog. Prompt-cleared
// on success is asserted in the whitebox TestPromptSend.
func TestPromptSendAndSlashMenu(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	ctx := context.Background()
	ses, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := tui.NewApp(c, store.State{}, ses.ID)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("New session"))
	}, teatest.WithDuration(5*time.Second))

	typeIn(tm, "hello")
	tm.Send(keyMsg(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSITest(b)
		return strings.Contains(s, "User:") && strings.Contains(s, "hello")
	}, teatest.WithDuration(5*time.Second))

	found := false
	for _, m := range ts.LastMessages(t, ses.ID) {
		if m.Info.Role != "user" {
			continue
		}
		for _, p := range m.Parts {
			if p.Type == "text" && p.Text == "hello" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("server-side messages do not contain the user text \"hello\"")
	}

	typeIn(tm, "/m")
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("/model"))
	}, teatest.WithDuration(5*time.Second))
	tm.Send(keyMsg(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSITest(b)
		return strings.Contains(s, "Model")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestPromptSendWhileBusyToasts is the T25 409 path: with a turn held open
// (fake Turn.Delay), a second prompt send must surface the locked toast —
// either from the store-side busy pre-check or the server 409 (client
// ErrBusy); both carry the same text.
func TestPromptSendWhileBusyToasts(t *testing.T) {
	drv := fake.New(fake.Turn{
		Parts: []llm.Part{{Kind: "text", Text: "working", Finish: "stop"}},
		Delay: 5 * time.Second,
	})
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	ctx := context.Background()
	ses, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := tui.NewApp(c, store.State{}, ses.ID)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("New session"))
	}, teatest.WithDuration(5*time.Second))

	typeIn(tm, "first")
	tm.Send(keyMsg(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSITest(b)
		return strings.Contains(s, "User:") && strings.Contains(s, "first")
	}, teatest.WithDuration(5*time.Second))

	typeIn(tm, "second")
	tm.Send(keyMsg(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSITest(b)
		return strings.Contains(s, "abort or wait (esc aborts)")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestPromptSlashNewWithoutSession is the LOCKED no-session path: /new typed
// at home with no current session must create the session directly (not via
// POST /session/{id}/command, which needs an id).
func TestPromptSlashNewWithoutSession(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := tui.NewApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Yolo"))
	}, teatest.WithDuration(5*time.Second))

	typeIn(tm, "/new")
	tm.Send(keyMsg(tea.KeyEnter))

	ctx := context.Background()
	for i := 0; i < 100; i++ {
		list, err := c.ListSessions(ctx)
		if err == nil && len(list) == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	list, err := c.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("sessions = %d, want 1 (locked CreateSession-direct path)", len(list))
	}

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
