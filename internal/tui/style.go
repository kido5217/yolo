package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// dividerWidth is locked by the home layout: 28 box-drawing runes.
const dividerWidth = 28

// Static styles. S0.8 moved the home surface, S0.9 the footer, and S0.10
// the session chrome to the theme accessors (a.theme). The two remaining
// statics have no upstream session-chrome analog (the upstream session
// route has no title line and separates messages with margins, not
// dividers, index.tsx:1199,1404): title (bold) + divider (ANSI 240) serve
// dialog.go (5 uses — the quit/help/model/agents titles + the
// dividerLineRendered const) and the view.go session title/divider + the
// session.go transcript divider — S2–S3 (dialog restyles) / S1 (glamour
// message layout) retheme them.
var (
	title   = lipgloss.NewStyle().Bold(true)
	divider = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// cursorStyle is the theme cursor/selection style (S0.10 removed the
// static cursor): bold + the theme text foreground — a zero Theme
// (nil-engine runs, S0.7) degrades to plain bold, never a panic.
func cursorStyle(th theme.Theme) lipgloss.Style { return th.Text().Bold(true) }

func dividerLine() string { return strings.Repeat("─", dividerWidth) }
