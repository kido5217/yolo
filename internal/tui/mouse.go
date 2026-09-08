package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleMouseMsg routes a mouse event to the open picker: the slash
// dropdown while the slash menu is open (the S5 mouse slice, spec §3.1),
// the @-picker dropdown while the @ menu is open (the S4 mouse slice,
// spec §3.8). The @-precedence gate (spec §3.9) keeps the two menus
// mutually exclusive (slashActive is false while mentionActive — S1), so
// at most one branch owns the frame; the check order is safe either way.
// The mouse on the other routes and menus is ignored (out of scope,
// spec §7).
func (a *App) handleMouseMsg(m tea.MouseMsg) tea.Cmd {
	if a.prompt.slashActive() {
		return a.handleSlashMouse(m)
	}
	if a.prompt.mentionActive() {
		return a.handleMentionMouse(m)
	}
	return nil
}

// handleSlashMouse handles a mouse event on the open slash dropdown (S5
// mouse slice, spec §3.1): a motion over a row moves the selection to the
// hovered row; a click on a row runs that command through the SAME enter
// path as handleMenuKey (runCommand, so the S4 touchCommandFrecency fires
// there too). The cell row is mapped through the S3 anchor
// (slashDropdownRows) to the S1 primitive's row geometry (dropdown.rowAt:
// 0 <= row < vis).
func (a *App) handleSlashMouse(m tea.MouseMsg) tea.Cmd {
	items := a.menuItems()
	if len(items) == 0 {
		return nil
	}
	firstRow, vis := a.slashDropdownRows()
	if firstRow < 0 {
		return nil
	}
	d := dropdown{vis: vis}
	row, ok := d.rowAt(m.Mouse().Y - firstRow)
	if !ok {
		return nil
	}
	switch m.(type) {
	case tea.MouseMotionMsg:
		a.prompt.sel = row
	case tea.MouseClickMsg:
		a.prompt.sel = row
		name := items[row].Name
		a.touchCommandFrecency(name)
		cmds := a.runCommand(name)
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	return nil
}

// handleMentionMouse handles a mouse event on the open @-picker dropdown
// (the S4 mouse slice, spec §3.8): a motion over a row moves the selection
// to the hovered row; a click on a row runs the SAME enter path as
// handleAcKey (acInsert — the S3 insert semantics, the insert-only frecency
// touch, and the menu close via the insert's trailing-space value change).
// A click on a directory row is the enter action (insert @<path> + close),
// NOT the tab expand (spec §3.8: only tab expands). The cell row is mapped
// through the S1 anchor (mentionDropdownRows — home: above the box top edge
// at boxL/boxW, session: the acMenu append above the prompt line) to the S1
// primitive's row geometry (dropdown.rowAt: 0 <= row < vis).
func (a *App) handleMentionMouse(m tea.MouseMsg) tea.Cmd {
	opts := a.mentionOptions()
	if len(opts) == 0 {
		return nil
	}
	firstRow, vis := a.mentionDropdownRows()
	if firstRow < 0 {
		return nil
	}
	d := dropdown{vis: vis}
	row, ok := d.rowAt(m.Mouse().Y - firstRow)
	if !ok {
		return nil
	}
	switch m.(type) {
	case tea.MouseMotionMsg:
		a.prompt.sel = row
	case tea.MouseClickMsg:
		a.prompt.sel = row
		if mo, isOpt := opts[row].value.(mentionOption); isOpt {
			a.acInsert(mo) // the S3 insert semantics (@-prefixed path + trailing space)
		}
	}
	return nil
}

// slashDropdownRows returns the S3 anchor for the open slash dropdown (the
// S5 mouse slice's referent): the dropdownRows scan over the current frame.
func (a *App) slashDropdownRows() (int, int) {
	return a.dropdownRows()
}

// mentionDropdownRows returns the S1 anchor for the open @-picker dropdown
// (the @-epic S1, spec §3.2): the same dropdownRows scan. Home: the
// box-anchored rows above the box top edge at boxL/boxW (homeView); session:
// the acMenu append above the prompt line (view.go).
func (a *App) mentionDropdownRows() (int, int) {
	return a.dropdownRows()
}

// dropdownRows scans the current frame for the open dropdown's rows: the
// 0-based cell row of its first rendered row and its visible row count. The
// open dropdown — the slash menu or the @-picker (the @-precedence gate,
// spec §3.9, keeps them mutually exclusive, so at most one is open) — is
// the only element that draws the S1 border rune ('|'), so the '|' rows are
// exactly its rows (the no-match line carries no border and is skipped). The
// frame's line index is the cell row (the frame renders from the top of the
// terminal). (-1, 0) when no dropdown is in the current frame (menus closed,
// or a modal took over the frame).
func (a *App) dropdownRows() (int, int) {
	first, count := -1, 0
	for i, line := range strings.Split(a.view(), "\n") {
		if strings.Contains(line, "|") {
			if first < 0 {
				first = i
			}
			count++
		}
	}
	return first, count
}
