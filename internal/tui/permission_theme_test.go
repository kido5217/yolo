package tui

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
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

// permWarnHeaderRe / permPillBgRe are position-anchored: the CSI that OPENS
// the styled run must carry the warning token (param order within the CSI
// not pinned — the cell-diff renderer merges the changed params into ONE
// CSI, deviation 141's substring convention; the brief's bare "38;5;215m"
// substring is structurally unmatchable for the same reason, deviation 181).
var (
	permWarnHeaderRe = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*38;5;215(?:;[0-9]+)*m△ `)
	permPillBgRe     = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*48;5;215(?:;[0-9]+)*mAllow`)
)

// TestPermissionDialogSGR pins the restyled permission dialog's paint
// (TTY_FORCE ANSI256, real engine — opencode dark): the warning header
// token (fg 215) and the selected pill's warning background (bg 215 —
// yolo look pin, deviation 182). The SGR tokens are pinned in the dialog's
// FIRST drain (deviation 181): the cell-diff renderer emits the panel once
// and never re-emits the unchanged lines, and each WaitFor drains the
// shared stream — the brief's permFlowHarness shape was restructured per
// the deviation-141 convention for two reasons: the shared harness runs a
// nil engine (the zero Theme paints no SGR), and a second WaitFor after the
// echo match would drain only the footer spinner's bytes. The plain tokens
// are pinned per RUN (not the contiguous hasPermDialogEcho form): under the
// real theme the cell-diff splits the panel lines at pen changes, so e.g.
// the header lands as "△ Permission" + a separate "required" run — the
// contiguous form is only matchable in the zero-theme flow drains.
func TestPermissionDialogSGR(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "text", Text: "working"},
			{Kind: "tool", Name: "bash", CallID: "call_1", Args: json.RawMessage(`{"command":"echo hi"}`), Finish: "tool_calls"},
		}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "all done"}}},
	)
	cfg := &protocol.Config{Permission: map[string]any{"bash": "ask"}}
	ts := testutil.BootWithDriverConfig(t, drv, cfg)

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
	if got := e.Active(); got != "opencode" {
		t.Fatalf("active theme = %s, want opencode (no config, no KV)", got)
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
	suiteType(tm, "run it")
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		if !permWarnHeaderRe.Match(b) || !permPillBgRe.Match(b) {
			return false
		}
		s := stripANSI(string(b))
		for _, tok := range []string{"required", "# Shell", "$ echo hi", "patterns: echo *", "Always: echo *", "Allow", "Reject"} {
			if !strings.Contains(s, tok) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(10*time.Second))
	// The park lands on the engine's goroutine; sync on it before quitting
	// (same guard as the flow tests).
	waitPending(t, ts, 1)
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
