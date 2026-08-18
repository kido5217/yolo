package tui_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

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
