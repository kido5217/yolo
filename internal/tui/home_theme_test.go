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

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// logoSGRTokens are the logo + divider SGR color parameters under the
// pinned TTY_FORCE=1 + TERM=xterm-256color env (ANSI256 profile: the
// 24-bit hex tokens quantize onto the xterm-256 gray ramp 232–255).
// Derived from the opencode dark-mode tokens (the S0.2 goldens):
//
//	textMuted      #808080 -> 244  (128 = (244-232)*10+8, exact)
//	text           #eeeeee -> 255  (238 = (255-232)*10+8, exact)
//	Tint(#0a0a0a, #808080, .25) = #282828 -> 235 (40: |38-40|=2 < |48-40|=8)
//	Tint(#0a0a0a, #eeeeee, .25) = #434343 -> 238 (67: |68-67|=1)
//	borderSubtle   #3c3c3c  -> 237  (60: |58-60|=2 < |68-60|=8)
//
// Substring assertions (no escape/terminator boundaries): the renderer's
// pen-diff merges the changed params into ONE CSI whose inner param order
// is not pinned (the redSGR precedent).
var logoSGRTokens = []string{
	"38;5;244", // left lines fg (textMuted)
	"38;5;255", // right lines fg (text)
	"48;5;235", // hollow marks bg (the shadow, left)
	"48;5;238", // hollow marks bg (the shadow, right)
	"38;5;237", // bottom divider (borderSubtle)
}

// logoBoldRe: the right block is bold (logo.tsx:49–60) — the fg-255 CSI
// must carry the bold attribute; the param order within the CSI is not
// pinned, so match both orders.
var logoBoldRe = regexp.MustCompile(`\x1b\[(?:1;38;5;255|38;5;255;1)m`)

// TestHomeLogoThemeSGR is the teatest SGR golden: boot the app with a
// REAL theme engine (the S0.7 wiring — the same theme the app uses), let
// it render home, and pin the logo + divider SGR color parameters.
func TestHomeLogoThemeSGR(t *testing.T) {
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

	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		// The fake terminal is not a TTY, so lipgloss strips every style.
		// Pin the env that derives ANSI256 from TERM alone (suite
		// convention).
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	// ONE merged condition (consecutive WaitFors drain each other): the
	// logo plain text (left line 2, the stable box-drawing marker),
	// every logo/divider SGR token, and the right block's bold flag.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		if !strings.Contains(stripANSI(string(b)), logoLeft[1]) {
			return false
		}
		for _, tok := range logoSGRTokens {
			if !bytes.Contains(b, []byte(tok)) {
				return false
			}
		}
		return logoBoldRe.Match(b)
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
