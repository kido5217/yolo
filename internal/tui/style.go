package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// dividerWidth is locked by the home layout: 28 box-drawing runes.
const dividerWidth = 28

var (
	title   = lipgloss.NewStyle().Bold(true)
	divider = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursor  = lipgloss.NewStyle().Bold(true)
	dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errRed  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	okGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	toolRow = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func dividerLine() string { return strings.Repeat("─", dividerWidth) }
