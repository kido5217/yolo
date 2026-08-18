package tui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	fakellm "github.com/kido5217/yolo/internal/llm/fake"
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
	a := tui.NewApp(c, &store.Store{}, "")
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
		return bytes.Contains(b, []byte("Hello \u00B7 kido/q"))
	}, teatest.WithDuration(5*time.Second))

	// The output stream is consumed by the WaitFor calls above (v2 teatest);
	// the two WaitFors are the locked assertions for this test.
	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestResumeMissingSessionExitsWithError(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := tui.NewApp(c, &store.Store{}, "ses_missing")
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
	drv := fakellm.New(
		fakellm.Turn{Parts: []llm.Part{
			{Kind: "text", Text: "thinking"},
			{Kind: "tool", Name: "read", CallID: "call_1", Args: json.RawMessage(`{"filePath":"hello.txt"}`), Finish: "tool_calls"},
		}},
		fakellm.Turn{Parts: []llm.Part{
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
	a := tui.NewApp(c, &store.Store{}, ses.ID)
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

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
