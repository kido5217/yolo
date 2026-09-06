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

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// homeFrameSGRTokens are the 0.8.0 start-screen frame SGR color parameters
// under the pinned TTY_FORCE=1 + TERM=xterm-256color env (ANSI256 profile:
// the 24-bit hex tokens quantize onto the xterm-256 cube / gray ramp).
// Derived from the yolo dark-mode tokens (the S0.2 goldens):
//
//	secondary          #5c9cf5 -> 75   (cube: R=95,G=175,B=255 = 16+36+18+5)
//	backgroundElement  #1e1e1e -> 234  (gray ramp: 28 = 8+10*2, index 232+2)
//	textMuted          #808080 -> 244  (gray ramp: 128 = 8+10*12, index 232+12)
//
// Substring assertions (no escape/terminator boundaries): the renderer's
// pen-diff merges the changed params into ONE CSI whose inner param order
// is not pinned (the redSGR precedent).
var homeFrameSGRTokens = []string{
	"38;5;75",  // box border fg (secondary)
	"48;5;234", // box interior bg (backgroundElement)
	"38;5;244", // footer dir muted (textMuted)
	"38;5;255", // box meta model name + hint shortcuts (text)
}

// TestHomeFrameSGR is the teatest SGR golden for the 0.8.0 start-screen
// frame: boot the app with a REAL theme engine (the S0.7 wiring — the same
// theme the app uses), let it render home, and pin the frame's SGR color
// parameters + the footer's dir/version content. ONE merged condition
// (consecutive WaitFors drain each other).
func TestHomeFrameSGR(t *testing.T) {
	t.Parallel()
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
	if got := e.Active(); got != "yolo" {
		t.Fatalf("active theme = %s, want yolo (no config, no KV)", got)
	}

	ts := testutil.Boot(t)
	// init a git repo at the test dir (the Task-3 integration fixture shape):
	// the footer's branch segment reads the scope dir's attached branch.
	runGit(t, ts.Dir, "init", "-q", "-b", "yolo-wire-branch")
	c := client.New(ts.URL, ts.Dir)
	// the homeMeta test seam catalog (the kido provider / "Qwen" model) —
	// the meta line reads store.Providers (applyHydrate overwrites
	// store.Config on the home route, never the providers).
	a := NewApp(c, metaCatalog(), "", e)
	a.SetVersion("v0.8.0-4-gabcdef")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(200, 50),
		// The fake terminal is not a TTY, so lipgloss strips every style.
		// Pin the env that derives ANSI256 from TERM alone (suite
		// convention).
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	// ONE merged condition (consecutive WaitFors drain each other): the
	// frame's SGR tokens (box border fg, interior bg, footer muted, text) +
	// the box's Task-5 content (placeholder, meta line, hint line) + the
	// footer's dir :branch suffix + the plain-semver version.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		if !strings.Contains(s, homeLogoLine) {
			return false
		}
		for _, tok := range homeFrameSGRTokens {
			if !bytes.Contains(b, []byte(tok)) {
				return false
			}
		}
		return strings.Contains(s, mockPlaceholder) &&
			strings.Contains(s, "Build · Qwen kido") &&
			strings.Contains(s, "tab agents  ctrl+p commands") &&
			strings.Contains(s, ":yolo-wire-branch") && strings.Contains(s, "0.8.0")
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
