package tui

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// dropdownRowOpen reports whether the frame carries a dropdown row — a line
// with the split ┃ border on both edges (≥2 ┃). The home box is left-border-
// only (one ┃ per row), so a ≥2-┃ line is exactly a dropdown row.
func dropdownRowOpen(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.Count(line, borderChar) >= 2 {
			return true
		}
	}
	return false
}

// screenDropdownRows reconstructs the rendered screen from the cumulative
// teatest output and counts its dropdown rows (the same ≥2-┃ rule as
// dropdownRowOpen, applied to the rebuilt screen instead of the raw bytes).
//
// It exists because teatest.WaitFor evaluates its predicate against every
// byte drained since the call — not the latest frame — while the bubbletea
// v2 renderer emits incremental diffs (each key press rewrites only the
// changed cells). A raw count over the raw bytes would sum the dropdown
// lines of every intermediate frame (9+4+2+1) and never observe the settled
// 1-row frame on its own. Rebuilding the screen recovers the per-frame row
// count, which is strictly decreasing as the queued keys are processed
// (9→4→2→1) and only reaches 1 once the program has processed the last key
// and rendered the settled frame.
//
// Screen model: the renderer's cursor contract for a non-TTY output (the
// teatest harness) — LF lands at column 0 of the next row (the map-newline
// contract) and ECH(n) clears n cells forward from the cursor; explicit
// CUP/CHA/move sequences reposition before every write either way. termW and
// termH match WithInitialTermSize(80, 24) in these tests. Only the
// operations the renderer emits for these screens are interpreted; unknown
// escapes and stray control bytes are skipped.
func screenDropdownRows(s string) int {
	const (
		termW = 80
		termH = 24
	)
	var grid [termH][termW]rune
	r, c := 0, 0
	regTop, regBot := 0, termH-1
	for i := 0; i < len(s); {
		switch ch := s[i]; {
		case ch == '\n':
			if r == regBot {
				for y := regTop; y < regBot; y++ {
					grid[y] = grid[y+1]
				}
				for x := range grid[regBot] {
					grid[regBot][x] = ' '
				}
			} else if r < termH-1 {
				r++
			}
			c = 0
			i++
		case ch == '\r':
			c = 0
			i++
		case ch == 0x1b:
			if i+1 >= len(s) || s[i+1] != '[' {
				i++
				continue
			}
			j := i + 2
			for j < len(s) && s[j] >= 0x30 && s[j] <= 0x3f {
				j++
			}
			if j < len(s) {
				switch s[j] {
				case 'H', 'f':
					if parts := strings.Split(s[i+2:j], ";"); len(parts) == 2 {
						if rr, err := strconv.Atoi(parts[0]); err == nil {
							r = clampRow(rr-1, termH)
						}
						if cc, err := strconv.Atoi(parts[1]); err == nil {
							c = clampCol(cc-1, termW)
						}
					}
				case 'G':
					if cc, err := strconv.Atoi(s[i+2 : j]); err == nil {
						c = clampCol(cc-1, termW)
					}
				case 'A':
					if n, err := strconv.Atoi(s[i+2 : j]); err == nil {
						r = max(0, r-n)
					}
				case 'B':
					if n, err := strconv.Atoi(s[i+2 : j]); err == nil {
						r = min(termH-1, r+n)
					}
				case 'M':
					if r > 0 {
						r--
					}
				case 'r':
					if parts := strings.Split(s[i+2:j], ";"); len(parts) == 2 {
						if t, err := strconv.Atoi(parts[0]); err == nil {
							regTop = clampRow(t-1, termH)
						}
						if b, err := strconv.Atoi(parts[1]); err == nil {
							regBot = clampRow(b-1, termH)
						}
					}
				case 'X':
					if n, err := strconv.Atoi(s[i+2 : j]); err == nil && c < termW {
						for x := c; x < c+n && x < termW; x++ {
							grid[r][x] = ' '
						}
					}
				case 'K':
					mode := 0
					if s[i+2:j] != "" {
						mode, _ = strconv.Atoi(s[i+2 : j])
					}
					switch mode {
					case 0:
						for x := c; x < termW; x++ {
							grid[r][x] = ' '
						}
					case 1:
						for x := 0; x <= c && x < termW; x++ {
							grid[r][x] = ' '
						}
					default:
						for x := range grid[r] {
							grid[r][x] = ' '
						}
					}
				}
			}
			i = j + 1
		default:
			if ch < 0x20 {
				i++
				continue
			}
			rn, sz := utf8.DecodeRuneInString(s[i:])
			if c < termW {
				grid[r][c] = rn
			}
			c++
			i += sz
		}
	}
	n := 0
	for y := range grid {
		if strings.Count(string(grid[y][:]), borderChar) >= 2 {
			n++
		}
	}
	return n
}

func clampRow(v, termH int) int {
	if v < 0 {
		return 0
	}
	if v > termH-1 {
		return termH - 1
	}
	return v
}

func clampCol(v, termW int) int {
	if v < 0 {
		return 0
	}
	if v > termW-1 {
		return termW - 1
	}
	return v
}

// TestPromptSlashMouseHover drives a mouse motion over a row of the open slash
// dropdown (S5 mouse, spec §3.1) and asserts the selection moves to the hovered
// row. The teatest leg sends a tea.MouseMsg at the S3 anchor for row 1
// (slashDropdownRows). The selection is asserted on the model after the program
// has quit (not a race with the running program).
func TestPromptSlashMouseHover(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", nil)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("● Tip"))
	}, teatest.WithDuration(5*time.Second))
	for _, r := range "/" {
		tm.Send(press(r))
	}
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return dropdownRowOpen(string(b))
	}, teatest.WithDuration(3*time.Second))

	// the S3 anchor for the open dropdown: the first row's cell + the visible
	// count (the menu opens with at least two rows, at sel 0).
	firstRow, vis := a.slashDropdownRows()
	if firstRow < 0 || vis < 2 {
		t.Fatalf("dropdown not open (firstRow=%d vis=%d)", firstRow, vis)
	}

	// a motion over row 1 (a non-top row) moves the selection there.
	tm.Send(tea.MouseMotionMsg{X: 10, Y: firstRow + 1, Button: tea.MouseNone})

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	if a.prompt.sel != 1 {
		t.Fatalf("expected sel 1 after hovering row 1, got %d", a.prompt.sel)
	}
}

// TestPromptSlashMouseClick drives a mouse click on the /new row of the open
// slash dropdown (a home without session) and asserts the command runs — a
// session is minted (the TestPromptSlashNewWithoutSession idiom, with the enter
// key replaced by the mouse click). The click runs the SAME enter path as the
// key handler (runCommand, so the S4 touchCommandFrecency fires there too).
func TestPromptSlashMouseClick(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", nil)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	ctx := context.Background()

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("● Tip"))
	}, teatest.WithDuration(5*time.Second))
	for _, r := range "/new" {
		tm.Send(press(r))
	}
	// The settled-state barrier: wait until the rendered frame shows exactly
	// one dropdown row. The row count is strictly decreasing as the queued
	// keys are processed (9→4→2→1), so this can only hold after the program
	// has processed all four keys and rendered the final frame — after which
	// no input is queued and the model is quiescent.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return screenDropdownRows(string(b)) == 1
	}, teatest.WithDuration(3*time.Second))

	// /new is the only dropdown row; the S3 anchor is its cell.
	firstRow, vis := a.slashDropdownRows()
	if firstRow < 0 || vis != 1 {
		t.Fatalf("expected 1 dropdown row (/new), got vis=%d firstRow=%d", vis, firstRow)
	}

	// a click on the /new row runs the command (mints a session on a home).
	tm.Send(tea.MouseClickMsg{X: 10, Y: firstRow, Button: tea.MouseLeft})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		sessions, err := c.ListSessions(ctx)
		return err == nil && len(sessions) >= 1
	}, teatest.WithDuration(5*time.Second))

	sessions, err := c.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 minted session after the click, got %d", len(sessions))
	}

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
