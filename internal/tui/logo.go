package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// The 8 logo lines — YOLO re-lettered in the upstream mark style
// (packages/tui/src/logo.ts; each half 9 columns). sha256-pinned in
// logo_test.go (root principle 3).
var (
	logoLeft = []string{
		"         ",
		"█  █ █▀▀█",
		" ██  █__█",
		" ██  ▀▀▀▀",
	}
	logoRight = []string{
		"         ",
		"█    █▀▀█",
		"█    █__█",
		"█▀▀▀ ▀▀▀▀",
	}
)

// logoWidth is the fixed block width (left 9 + gap 1 + right 9). The
// logo never wraps or shrinks — on a <19-column terminal the
// alt-screen frame clips it (the upstream look).
const logoWidth = 19

// Mark classes (upstream marks "_^~,"). The glyph is always translated;
// the paint follows the class.
const (
	markPlain  = iota // every unmarked rune: fg only
	markHollow        // '_','^' -> " "/"▀": fg + bg(shadow)
	markShadow        // '~',',' -> "▀"/"▄": fg(shadow)
)

// renderLogo renders the 4-line logo block (the port of
// logo.tsx:49–60): the left lines in textMuted (non-bold), the right
// lines in text (bold), one unstyled gap column between them. A zero
// Theme (nil-engine runs, S0.7) degrades to the plain translated
// glyphs — never a panic.
func renderLogo(th theme.Theme) string {
	fgLeft, leftOK := th.Color("textMuted")
	fgRight, rightOK := th.Color("text")
	bg, bgOK := th.Color("background")
	var b strings.Builder
	for i := range logoLeft {
		b.WriteString(logoLine(logoLeft[i], fgLeft, leftOK, bg, bgOK, false))
		b.WriteByte(' ')
		b.WriteString(logoLine(logoRight[i], fgRight, rightOK, bg, bgOK, true))
		if i+1 < len(logoLeft) {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// logoLine renders one 9-column line (the port of renderLine,
// logo.tsx:9–47). The mark glyphs are always translated ('_' -> " ",
// '^' -> "▀", '~' -> "▀", ',' -> "▄"); the paint follows the mark class
// with shadow = Tint(background, fg, 0.25) (logo.tsx:10). Consecutive
// same-class cells emit as one styled run; a missing token (zero Theme)
// renders the plain glyphs.
func logoLine(line string, fg theme.Rgba, fgOK bool, bg theme.Rgba, bgOK bool, bold bool) string {
	shadow := theme.Tint(bg, fg, 0.25)
	var (
		fgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(fg.Hex()[:7]))
		hollowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(fg.Hex()[:7])).Background(lipgloss.Color(shadow.Hex()[:7]))
		shadowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(shadow.Hex()[:7]))
	)
	if bold {
		fgStyle = fgStyle.Bold(true)
		hollowStyle = hollowStyle.Bold(true)
		shadowStyle = shadowStyle.Bold(true)
	}
	var b, run strings.Builder
	class := -1
	flush := func() {
		if run.Len() == 0 {
			return
		}
		glyphs := run.String()
		run.Reset()
		if !fgOK {
			b.WriteString(glyphs) // zero Theme: the plain translated glyphs
			return
		}
		var st lipgloss.Style
		switch {
		case class == markHollow && bgOK:
			st = hollowStyle
		case class == markShadow:
			st = shadowStyle
		default:
			st = fgStyle
		}
		b.WriteString(st.Render(glyphs))
	}
	for _, r := range line {
		var c int
		switch r {
		case '_':
			c, r = markHollow, ' '
		case '^':
			c, r = markHollow, '▀'
		case '~':
			c, r = markShadow, '▀'
		case ',':
			c, r = markShadow, '▄'
		default:
			c = markPlain
		}
		if c != class {
			flush()
			class = c
		}
		run.WriteRune(r)
	}
	flush()
	return b.String()
}
