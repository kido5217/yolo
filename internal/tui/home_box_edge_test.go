package tui

// home_box_edge_test.go — yolo-ql7.2 (the home-box right-edge golden
// regression): at a 200x50 terminal the last 2 interior cells of the box's
// two content rows (the placeholder row boxTop+1 and the meta row
// boxTop+3) MUST carry the interior bg (backgroundElement, `48;5;234`
// under the pinned TTY_FORCE=1 + TERM=xterm-256color ANSI256 env). The
// pre-fix 2-col right-pad shortfall (homeBox rows 1/3 emitted boxW-2
// display cols — the unpainted cells that showed the terminal's own bg,
// proven in-the-bytes by yolo-ql7.1; the raw renderer captures + the cell
// map live on the local-only research asset branch
// research/home-box-artifact @ dd1bd2d) can never silently return: this
// golden decodes the FULL raw renderer stream into a cell grid and pins
// the 4 artifact cells (+ the geometry anchors + the discriminative paint
// controls).
//
// Blackbox per the home_golden_test.go teatest pattern: the real app, the
// real theme engine, the real bubbletea v2 renderer — the decoded grid is
// the cell state a real xterm-256color terminal ends up with.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// the 200x50 box contract (the research cell map + the mock geometry): the
// box is homeBoxMaxWidth 75 at cols edgeBoxL..edgeBoxL+homeBoxMaxWidth-1
// (63..137, 0-based), the interior cols 64..137, the five box rows at
// edgeBoxTop..+4 (24..28: topSpacer 14 + the homeView stack prefix 4 pad +
// 4 logo + 2 spacers). The 4 artifact cells: the last 2 interior cols
// (136/137) of the two content rows 25 (placeholder) and 27 (meta).
const (
	edgeBoxL   = 63
	edgeBoxTop = 24
)

// TestHomeBoxRightEdge boots the real app at 200x50 (the home route, the
// placeholder state — the reported user state), pumps the full raw
// renderer stream, decodes it into a cell grid, and pins the 4 artifact
// cells + the geometry anchors + the discriminative controls.
func TestHomeBoxRightEdge(t *testing.T) {
	t.Parallel()
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
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
	a := NewApp(c, metaCatalog(), "", e)
	a.SetVersion("v0.8.0-4-gabcdef")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(mockW, mockH),
		// the fake terminal is not a TTY, so lipgloss strips every
		// style; pin the env that derives ANSI256 (suite convention).
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)
	// ONE settle: the home frame is up (the logo + the placeholder line).
	// The decode is of the FULL stream — the last rendered frame wins, so
	// the grid holds the settled frame (loadShown is false by then).
	raw := pumpUntil(t, tm, nil, func(b []byte) bool {
		s := stripANSI(string(b))
		return strings.Contains(s, homeLogoLine) && strings.Contains(s, mockPlaceholder)
	}, 15*time.Second)
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	raw = append(raw, drain(tm)...)

	g := decodeVT(raw, mockW, mockH)
	// the geometry anchors (0-based cells): the ┃ border (fg secondary
	// 38;5;75) at the box top row, the ╹ corner at boxTop+4.
	if got := g.cell(edgeBoxTop, edgeBoxL); got.ch != '┃' || got.fg != "38;5;75" {
		t.Fatalf("box top border cell = %q (fg %q), want ┃ (fg 38;5;75)", got.ch, got.fg)
	}
	if got := g.cell(edgeBoxTop+4, edgeBoxL); got.ch != '╹' {
		t.Fatalf("box bottom corner cell = %q, want ╹", got.ch)
	}
	// the 4 artifact cells: the last 2 interior cols of the two content
	// rows carry the interior bg (the unpainted 2-col chip is gone).
	for _, row := range []int{edgeBoxTop + 1, edgeBoxTop + 3} {
		for _, col := range []int{edgeBoxL + homeBoxMaxWidth - 2, edgeBoxL + homeBoxMaxWidth - 1} {
			if got := g.cell(row, col); got.bg != "48;5;234" {
				t.Fatalf("row %d col %d bg = %q, want 48;5;234 (the unpainted 2-col right-edge chip)", row, col, got.bg)
			}
		}
	}
	// the discriminative controls: a mid-interior cell IS painted (the
	// decoder paints) and the cell just outside the box right edge is NOT
	// (the decoder does not smear the bg past the frame row).
	if got := g.cell(edgeBoxTop+1, edgeBoxL+10); got.bg != "48;5;234" {
		t.Fatalf("mid-interior cell bg = %q, want 48;5;234 (decoder under-paints)", got.bg)
	}
	if got := g.cell(edgeBoxTop+1, edgeBoxL+homeBoxMaxWidth); got.bg != "" {
		t.Fatalf("outside-box cell bg = %q, want default (decoder over-paints)", got.bg)
	}
}

// vtPen is the active SGR pen (the exact param strings; "" = the default
// pen).
type vtPen struct {
	fg string
	bg string
}

// vtCell is one decoded terminal cell (the last paint wins; ch 0 =
// unpainted).
type vtCell struct {
	ch rune
	fg string
	bg string
}

// vtGrid is the decoded cell grid (w cols x h rows).
type vtGrid struct {
	w, h  int
	cells [][]vtCell
}

func newVTGrid(w, h int) *vtGrid {
	cells := make([][]vtCell, h)
	for r := range cells {
		cells[r] = make([]vtCell, w)
	}
	return &vtGrid{w: w, h: h, cells: cells}
}

// cell returns one grid cell (panics out of range — a decoder bug, not a
// stream condition).
func (g *vtGrid) cell(r, c int) vtCell {
	if r < 0 || r >= g.h || c < 0 || c >= g.w {
		panic(fmt.Sprintf("vtGrid.cell: (%d, %d) out of range (%dx%d)", r, c, g.w, g.h))
	}
	return g.cells[r][c]
}

// decodeVT replays a raw renderer stream into the cell grid: the cursor
// moves (CUP/CHA/VPA/arrows/LF/CR/BS/TAB), the SGR pen (the 38/48 256- and
// 24-bit color forms + the 39/49 defaults + the 0 reset; the display attrs
// are kept out of the pen model), the ECH fill paint (the renderer's
// interior-fill mechanism), the ED/EL erases, and the printable output.
// The private sequences (the ? / > / 0x40-0x45 intermediates — modes, DSR,
// device attributes) and the line/char inserts carry no cell effect and
// are skipped (the home-frame stream emits only the above).
func decodeVT(raw []byte, w, h int) *vtGrid {
	g := newVTGrid(w, h)
	var pen vtPen
	r, c := 0, 0
	for pos := 0; pos < len(raw); {
		b := raw[pos]
		switch b {
		case 0x1b:
			pos = g.stepEscape(raw, pos, &pen, &r, &c)
		case '\n':
			if r < h-1 {
				r++
			}
			pos++
		case '\r':
			c = 0
			pos++
		case '\b':
			if c > 0 {
				c--
			}
			pos++
		case '\t':
			c = (c/8 + 1) * 8
			if c > g.w {
				c = g.w
			}
			pos++
		default:
			ru, size := utf8.DecodeRune(raw[pos:])
			if ru == utf8.RuneError && size == 1 {
				pos++ // an undecodable byte — skip
				continue
			}
			if c < w {
				cell := &g.cells[r][c]
				cell.ch = ru
				cell.fg = pen.fg
				cell.bg = pen.bg
			}
			c += runeWidth(string(ru))
			pos += size
		}
	}
	return g
}

// stepEscape consumes one escape sequence starting at pos (the ESC byte)
// and returns the position just past it.
func (g *vtGrid) stepEscape(raw []byte, pos int, pen *vtPen, r, c *int) int {
	if pos+1 >= len(raw) {
		return len(raw)
	}
	switch raw[pos+1] {
	case '[':
		return g.stepCSI(raw, pos+2, pen, r, c)
	case ']': // OSC: skip to the terminator (BEL or the ST pair).
		pos += 2
		for pos < len(raw) {
			if raw[pos] == 0x07 {
				return pos + 1
			}
			if raw[pos] == 0x1b && pos+1 < len(raw) && raw[pos+1] == '\\' {
				return pos + 2
			}
			pos++
		}
		return pos
	default:
		return pos + 2 // a single-byte escape (an Fe key, etc.)
	}
}

// stepCSI consumes one CSI sequence (pos is just past ESC[) and returns
// the position just past it. The private sequences (the ? / > / 0x40-0x45
// intermediates) are consumed but never dispatched.
func (g *vtGrid) stepCSI(raw []byte, pos int, pen *vtPen, r, c *int) int {
	var params []int
	var cur int
	have := false
	priv := false
	end := pos
	for end < len(raw) {
		b := raw[end]
		switch {
		case b >= '0' && b <= '9':
			cur = cur*10 + int(b-'0')
			have = true
		case b == ';' || b == ':':
			if !priv {
				params = append(params, cur)
			}
			cur, have = 0, false
		case b == '?' || b == '>' || (b >= 0x40 && b <= 0x45):
			priv = true
		default: // the final byte (0x46-0x7E)
			if have && !priv {
				params = append(params, cur)
			}
			if !priv {
				g.dispatchCSI(b, params, pen, r, c)
			}
			return end + 1
		}
		end++
	}
	return len(raw) // an unterminated CSI at the stream end
}

// dispatchCSI applies one decoded (non-private) CSI sequence. n(i) is the
// i-th param (the VT default d when absent or 0). The cursor is clamped to
// the grid (one column past the right edge is legal — a print there is a
// no-op).
func (g *vtGrid) dispatchCSI(final byte, params []int, pen *vtPen, r, c *int) {
	n := func(i, def int) int {
		if i < len(params) && params[i] > 0 {
			return params[i]
		}
		return def
	}
	erase := func(r0, r1, c0, c1 int) {
		for rr := r0; rr <= r1; rr++ {
			for cc := c0; cc <= c1; cc++ {
				g.cells[rr][cc] = vtCell{}
			}
		}
	}
	switch final {
	case 'H':
		*r = n(0, 1) - 1
		*c = n(1, 1) - 1
	case 'G':
		*c = n(0, 1) - 1
	case 'd':
		*r = n(0, 1) - 1
	case 'A':
		*r -= n(0, 1)
	case 'B':
		*r += n(0, 1)
	case 'C':
		*c += n(0, 1)
	case 'D':
		*c -= n(0, 1)
	case 'E':
		*c = 0
		*r += n(0, 1)
	case 'F':
		*c = 0
		*r -= n(0, 1)
	case 'X': // ECH: fill-erase n cells with the active pen (the
		// renderer's interior-fill mechanism).
		for i := 0; i < n(0, 1) && *c < g.w; i++ {
			cell := &g.cells[*r][*c]
			cell.ch = ' '
			cell.fg = pen.fg
			cell.bg = pen.bg
			*c++
		}
	case 'J':
		switch n(0, 0) {
		case 0:
			erase(*r, *r, *c, g.w-1)
			erase(*r+1, g.h-1, 0, g.w-1)
		case 2, 3:
			erase(0, g.h-1, 0, g.w-1)
		}
	case 'K':
		switch n(0, 0) {
		case 0:
			erase(*r, *r, *c, g.w-1)
		case 1:
			erase(*r, *r, 0, *c)
		case 2:
			erase(*r, *r, 0, g.w-1)
		}
	case 'm':
		applyVTSGR(pen, params)
	}
	// the 'h'/'l' modes, the 'n'/'u'/'p' status/attribute responses, and
	// the line/char inserts ('L'/'M'/'@'/'P') carry no cell effect here
	// (the home-frame stream emits none of them).
	if *r < 0 {
		*r = 0
	}
	if *r > g.h-1 {
		*r = g.h - 1
	}
	if *c < 0 {
		*c = 0
	}
	if *c > g.w {
		*c = g.w
	}
}

// applyVTSGR updates the pen from the SGR param list (the 0 reset, the
// 38/48 introducers (the 5- and 24-bit color forms), the 39/49 defaults;
// the display attrs 1/3/4/5/7 are kept out of the pen model).
func applyVTSGR(pen *vtPen, params []int) {
	if len(params) == 0 {
		*pen = vtPen{} // a bare ESC[m
		return
	}
	for i := 0; i < len(params); {
		switch params[i] {
		case 0:
			*pen = vtPen{}
			i++
		case 39:
			pen.fg = ""
			i++
		case 49:
			pen.bg = ""
			i++
		case 38, 48:
			isFg := params[i] == 38
			if i+2 < len(params) && params[i+1] == 5 {
				col := fmt.Sprintf("%d;5;%d", params[i], params[i+2])
				if isFg {
					pen.fg = col
				} else {
					pen.bg = col
				}
				i += 3
			} else if i+4 < len(params) && params[i+1] == 2 {
				col := fmt.Sprintf("%d;2;%d;%d;%d", params[i], params[i+2], params[i+3], params[i+4])
				if isFg {
					pen.fg = col
				} else {
					pen.bg = col
				}
				i += 5
			} else {
				i++
			}
		default:
			i++ // a display attr (bold, italic, underline, blink, ...)
		}
	}
}
