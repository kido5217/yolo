package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// dividerWidth is locked by the home layout: 28 box-drawing runes.
const dividerWidth = 28

// Static styles. S0.8 moved the home surface to the theme accessors
// (a.theme) — home no longer consumes title/divider. S0.9 removed dim —
// every former consumer reads the theme's textMuted accessor. Ownership of
// the remaining statics: cursor (home cursor row, model/agent dialog rows,
// slash-menu selection), errRed/okGreen (footer conn segment, the `!`
// error line, provider dots), toolRow (transcript tool rows) → S0.10;
// title/divider serve the non-home surfaces (session chrome view.go +
// session.go, the dialogs dialog.go) → S0.10/S3.
var (
	title   = lipgloss.NewStyle().Bold(true)
	divider = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursor  = lipgloss.NewStyle().Bold(true)
	errRed  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	okGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	toolRow = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func dividerLine() string { return strings.Repeat("─", dividerWidth) }
