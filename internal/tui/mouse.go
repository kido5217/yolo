package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleMouseMsg handles a mouse event on the open slash dropdown (S5 mouse
// slice, spec §3.1): a motion over a row moves the selection to the hovered
// row; a click on a row runs that command through the SAME enter path as
// handleMenuKey (runCommand, so the S4 touchCommandFrecency fires there too).
// Active only while the slash menu is open (prompt.slashActive); mouse on
// other routes/menus (the @ picker) is ignored (out of scope, spec §7). The
// cell row is mapped through the S3 anchor (slashDropdownRows) to the S1
// primitive's row geometry (dropdown.rowAt: 0 <= row < vis).
func (a *App) handleMouseMsg(m tea.MouseMsg) tea.Cmd {
	if !a.prompt.slashActive() {
		return nil
	}
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

// slashDropdownRows returns the S3 anchor for the open slash dropdown: the
// 0-based cell row of its first rendered row and its visible row count, read
// off the current frame. The dropdown is the only element that draws the S1
// border rune ('|'), so the '|' rows are exactly its rows. The frame's line
// index is the cell row (the frame renders from the top of the terminal).
// (-1, 0) when the dropdown is not in the current frame (menu closed, or a
// modal took over the frame).
func (a *App) slashDropdownRows() (int, int) {
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
