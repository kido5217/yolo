package tui

import (
	"bytes"
	"context"
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
	"github.com/kido5217/yolo/internal/tui/theme"
)

// TestMarkdownTextPartSGR pins the S1.3 transcript rendering under the
// pinned TTY_FORCE=1 + TERM=xterm-256color env: the text part renders
// through the glamour renderer — the markdown markers are stripped, the
// base text takes markdownText (38;5;255), the bold run takes
// markdownStrong (38;5;215), and the rendered lines carry the upstream
// 3-column indent (index.tsx:1701).
func TestMarkdownTextPartSGR(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "text", Text: "Here is **bold** text\n\nsome more\n"},
		}},
	)
	ts := testutil.BootWithDriverConfig(t, drv, &protocol.Config{})
	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}

	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "hi")
	tm.Send(press(tea.KeyEnter))

	// ONE merged terminal state (suite convention): the markers stripped
	// and the 3-column indent on both lines, both SGR tokens. The help
	// line is NOT pinned here: it settles in the session-route frame and
	// the cell-diff renderer never re-emits an unchanged line into a later
	// drain (deviation 141/142) — it stays pinned by the wait above.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		if !strings.Contains(s, "   Here is bold text") ||
			!strings.Contains(s, "   some more") {
			return false
		}
		if strings.Contains(s, "**") {
			t.Error("markdown markers not stripped")
			return false
		}
		return bytes.Contains(b, []byte("38;5;255")) &&
			bytes.Contains(b, []byte("38;5;215"))
	}, teatest.WithDuration(10*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
