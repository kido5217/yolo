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
// Derived from the yolo dark-mode tokens (the S0.2 goldens):
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
	if got := e.Active(); got != "yolo" {
		t.Fatalf("active theme = %s, want yolo (no config, no KV)", got)
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

// homeSGRTokens are the S0.9 home-row/footer SGR color parameters under the
// pinned TTY_FORCE=1 + TERM=xterm-256color env (ANSI256 profile: the 24-bit
// hex tokens quantize through x/ansi v0.11.8 Convert256 —
// charmbracelet/x/ansi color.go: to6Cube (v<48→0, v<115→1, else (v-35)/40 →
// 0x00/0x5f/0x87/0xaf/0xd7/0xff), an exact cube hit returns early, else the
// grey index (avg-3)/10 with avg>238 → 23 (avg = (r+g+b)/3) and a
// DistanceHSLuv cube-vs-grey tie-break. Derived from the yolo
// dark-mode tokens:
//
//	text (non-selected row title)       #eeeeee (238):
//	    grey 23 exact (238 = 8+10*23) -> 255
//	textMuted (row meta tail, help line, footer)  #808080 (128):
//	    grey 12 exact (128 = 8+10*12) -> 244
//	SelectedForeground (selected row text + the ▸ indicator) — the port
//	    returns the opaque `background` token, #0a0a0a (10):
//	    cube 16 (0,0,0) vs grey 232 (8): the closer achromatic is 8 -> 232
//	primary (selected row background)   #fab283 (250,178,131):
//	    cube 16+36*5+6*3+2 = 216 (255,175,135) vs grey 250 (188, avg 186):
//	    the peach cube beats the achromatic grey in HSLuv -> 216
//
// 255/244 also appear in the S0.8 logo block (right/left lines) — the row
// and footer markers ("T1", "↑0 ↓0") in the WaitFor condition pin their
// contribution here.
//
// Substring assertions (no escape/terminator boundaries): the renderer's
// pen-diff merges the changed params into ONE CSI whose inner param order
// is not pinned (the redSGR / logoBoldRe precedent).
var homeSGRTokens = []string{
	"38;5;255", // non-selected row title (text)
	"38;5;244", // row meta tail, help line, footer (textMuted)
	"38;5;232", // selected row text + ▸ (SelectedForeground = background)
	"48;5;216", // selected row background (primary)
}

// selectedRowSGRRe matches the ▸ cell's merged bold+fg+bg CSI (all six
// param permutations — the order is not pinned). The ▸ run re-emits all
// three params in one transition: the two plain lead columns before it
// reset the pen to the default state, so the ▸ cell sets bold, foreground
// and background together.
var selectedRowSGRRe = regexp.MustCompile(`\x1b\[(?:1;38;5;232;48;5;216|1;48;5;216;38;5;232|38;5;232;1;48;5;216|38;5;232;48;5;216;1|48;5;216;1;38;5;232|48;5;216;38;5;232;1)m`)

// TestHomeFooterThemeSGR is the teatest SGR golden for the S0.9 shell
// restyle: boot the app with a REAL theme engine (the S0.7 wiring) over a
// real server seeded with one session, let it render home, and pin the
// home-row + footer SGR color parameters.
func TestHomeFooterThemeSGR(t *testing.T) {
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
	c := client.New(ts.URL, ts.Dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, err := c.CreateSession(ctx, "T1"); err != nil {
		t.Fatalf("seed session T1: %v", err)
	}
	cancel()

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
	// home markers (the selected "New session" row, the seeded "T1" row,
	// the home footer), every SGR token and the ▸ cell's merged bold+fg+bg
	// CSI.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		if !strings.Contains(s, "New session") || !strings.Contains(s, "T1") ||
			!strings.Contains(s, "↑0 ↓0") {
			return false
		}
		for _, tok := range homeSGRTokens {
			if !bytes.Contains(b, []byte(tok)) {
				return false
			}
		}
		return selectedRowSGRRe.Match(b)
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestHomeRowLines pins the segment-preserving wrap (rowLines): a row that
// fits keeps its verbatim spacing; a wrapped row re-derives the title/meta
// split per visual line — a mid-line boundary leaves the trailing join
// space on the title run, a line-boundary boundary drops it (wrapLine drops
// leading spaces on continuation lines).
func TestHomeRowLines(t *testing.T) {
	got := rowLines("  ▸ ", "New session", "", 80)
	if len(got) != 1 || got[0].cur != "▸ " || got[0].title != "New session" || got[0].meta != "" {
		t.Fatalf("fast path = %+v, want one verbatim line", got)
	}
	got = rowLines("  ", "T1", " · kido/q · 2m", 80)
	if len(got) != 1 || got[0].title != "T1 " || got[0].meta != "· kido/q · 2m" {
		t.Fatalf("join split = %+v, want the join space on the title run", got)
	}
	// effW = 18: six 3-word lines, then "word word · 2m" (the mid-line
	// boundary leaves the join space on the title run).
	long := strings.Repeat("word ", 20)
	got = rowLines("  ", long, " · 2m", 20)
	if len(got) != 7 {
		t.Fatalf("lines = %d, want 7: %+v", len(got), got)
	}
	for i := 0; i < 6; i++ {
		if got[i].cur != "" || got[i].title != "word word word" || got[i].meta != "" {
			t.Fatalf("line %d = %+v", i, got[i])
		}
	}
	if got[6].cur != "" || got[6].title != "word word " || got[6].meta != "· 2m" {
		t.Fatalf("last line = %+v", got[6])
	}
	// An over-long token hard-splits at the width (the wrapLine contract).
	got = rowLines("  ", strings.Repeat("x", 30), "", 20)
	if len(got) != 2 || got[0].title != strings.Repeat("x", 18) || got[1].title != strings.Repeat("x", 12) {
		t.Fatalf("hard split = %+v, want 18 + 12", got)
	}
}

// TestHomeRenderRowZeroTheme pins the nil-engine degradation (the S0.7
// zero-Theme rule): plain text — the cursor row keeps the static cursor
// bold on the "▸" run with plain content, and a wrapped continuation line
// indents 2 (4 for the cursor row).
func TestHomeRenderRowZeroTheme(t *testing.T) {
	var zero theme.Theme
	h := homeModel{cursor: 0}
	if got := stripANSI(h.renderRow(0, "New session", "", 80, zero)); got != "  ▸ New session" {
		t.Fatalf("cursor row = %q, want %q", got, "  ▸ New session")
	}
	if got := stripANSI(h.renderRow(1, "T1", " · kido/q · 2m", 80, zero)); got != "  T1 · kido/q · 2m" {
		t.Fatalf("row = %q, want %q", got, "  T1 · kido/q · 2m")
	}
	long := strings.Repeat("word ", 20)
	want := "  word word word\n" +
		strings.Repeat("  word word word\n", 5) +
		"  word word · 2m"
	if got := stripANSI(h.renderRow(1, long, " · 2m", 20, zero)); got != want {
		t.Fatalf("wrapped row = %q, want %q", got, want)
	}
}
