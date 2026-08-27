package tui

import (
	"bytes"
	"context"
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

// TestReasoningSummary ports the thinking.ts:12 regex table.
func TestReasoningSummary(t *testing.T) {
	tests := []struct {
		in    string
		title string
		body  string
	}{
		{in: "**Inspecting PR workflow**\n\nThe body here.", title: "Inspecting PR workflow", body: "The body here."},
		{in: "**Title only**", title: "Title only", body: ""},
		{in: "**No blank line**\ntext", title: "", body: "**No blank line**\ntext"},
		{in: "no title at all", title: "", body: "no title at all"},
		{in: "  **Padded**\n\nbody", title: "Padded", body: "body"},
	}
	for _, tc := range tests {
		gotT, gotB := reasoningSummary(tc.in)
		if gotT != tc.title || gotB != tc.body {
			t.Errorf("reasoningSummary(%q) = (%q, %q), want (%q, %q)",
				tc.in, gotT, gotB, tc.title, tc.body)
		}
	}
}

// TestDurationText ports the Locale.duration table (util/locale.ts:39).
func TestDurationText(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{500, "500ms"}, {1000, "1.0s"}, {1234, "1.2s"},
		{61000, "1m 1s"}, {3600000, "1h 0m"}, {90061000, "1d 1h"},
	}
	for _, tc := range tests {
		if got := durationText(tc.ms); got != tc.want {
			t.Errorf("durationText(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}

// TestReasoningPartSGR pins the S1.6 reasoning restyle under the pinned
// TTY_FORCE=1 + TERM=xterm-256color env. The DONE reasoning row is
// "+/- Thought: <title>" with an OPTIONAL " · <duration>" suffix — the
// engine stamps PartTime at ms resolution (round.go:52,86 — Start at part
// creation, End at finalization) and the fake turn finalizes in <1ms
// (End−Start=0), so the brief's own title-only row form is the actual row;
// the regex pins the shape, never a forced duration. (The fake part
// carries no Time of its own — llm.Part has no Time field; the engine owns
// it.) The expanded body renders through the subtle renderer per the S1.4
// contract: the base text keeps the RAW theme.textMuted fg (→ 38;5;244)
// while only the CHROMA is pre-blended to subtle, and the open header
// takes warning-subtle (→ 38;5;94).
func TestReasoningPartSGR(t *testing.T) {
	thoughtRe := regexp.MustCompile(`[+-] Thought: Planning( · \d+(?:ms|\.\ds))?`)
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "reasoning", Text: "**Planning**\n\nvar x = 1"},
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
	// the turn is done: the reasoning row shows (collapsed by default).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return thoughtRe.Match([]byte(stripANSI(string(b))))
	}, teatest.WithDuration(10*time.Second))
	// alt+t expands: the body renders (subtle markdown).
	tm.Send(pressAlt('t'))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		// duration optional (same root cause as the collapsed regex).
		if !strings.Contains(s, "- Thought: Planning") {
			return false
		}
		if !strings.Contains(s, "var x = 1") {
			return false
		}
		// subtle base text: raw theme.textMuted fg (→ 244), only the
		// chroma pre-blended to subtle.
		if !bytes.Contains(b, []byte("38;5;244")) {
			return false
		}
		// deterministic open header: warning-subtle (→ 94).
		return bytes.Contains(b, []byte("38;5;94"))
	}, teatest.WithDuration(10*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
