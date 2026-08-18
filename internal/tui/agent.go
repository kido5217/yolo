package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/store"
)

// agentDlg is the ctrl+a / /agents picker: the server's agent list (name +
// description + current marker). Enter opens the locked [a] this session /
// [b] set default overlay.
type agentDlg struct {
	sel       int
	subChoice bool
}

// openAgentDialog pushes the agent dialog, seeds its selection from the
// session (or config) agent, and fetches the catalog.
func (a *App) openAgentDialog() []tea.Cmd {
	a.agentDlg = &agentDlg{}
	a.syncAgentSel()
	a.dlg.push(dialog{kind: dlgAgents})
	return a.emit(a.fetchCatalogCmd())
}

// closeAgentDialog pops the dialog and drops its state.
func (a *App) closeAgentDialog() {
	a.dlg.pop()
	a.agentDlg = nil
}

// currentAgentName is the session agent, falling back to the config agent.
func currentAgentName(st *store.Store) string {
	if cur := st.Current; cur != nil && cur.Agent != "" {
		return cur.Agent
	}
	if s, ok := st.Config["agent"].(string); ok {
		return s
	}
	return ""
}

// syncAgentSel points the dialog at the current agent; nil-safe when the
// catalog is not hydrated yet.
func (a *App) syncAgentSel() {
	ag := a.agentDlg
	if ag == nil {
		return
	}
	name := currentAgentName(&a.store)
	for i, x := range a.store.Agents {
		if x.Name == name {
			ag.sel = i
			return
		}
	}
}

// selectedName is the name of the selected agent.
func (m *agentDlg) selectedName(st *store.Store) string {
	if m.sel >= 0 && m.sel < len(st.Agents) {
		return st.Agents[m.sel].Name
	}
	return ""
}

// handleKey drives the dialog while it owns the keys: esc closes (or cancels
// the subchoice), arrows move with wraparound, enter opens the subchoice, a/b
// apply (LOCKED overlay).
func (m *agentDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if key.Matches(k, escBinding) {
		if m.subChoice {
			m.subChoice = false
		} else {
			a.closeAgentDialog()
		}
		return nil
	}
	if m.subChoice {
		switch {
		case key.Matches(k, choiceThis):
			if a.cur == "" {
				a.toast("no session")
				return nil
			}
			return a.emit(a.patchDlgCmd("agent", m.selectedName(&a.store), true))
		case key.Matches(k, choiceDef):
			return a.emit(a.patchDlgCmd("agent", m.selectedName(&a.store), false))
		}
		return nil
	}
	n := len(a.store.Agents)
	switch {
	case key.Matches(k, homeKeyMap.Up):
		if n > 0 {
			m.sel = ((m.sel - 1) % n + n) % n
		}
	case key.Matches(k, homeKeyMap.Down):
		if n > 0 {
			m.sel = (m.sel + 1) % n
		}
	case key.Matches(k, homeKeyMap.Enter):
		if n > 0 {
			m.subChoice = true
		}
	}
	return nil
}

// view renders the agent list, the subchoice overlay and the keymap hint.
func (m *agentDlg) view(st *store.Store) string {
	var b strings.Builder
	b.WriteString(title.Render("Agents") + "\n")
	if len(st.Agents) == 0 {
		b.WriteString(dim.Render("  loading…"))
		return b.String()
	}
	if m.sel >= len(st.Agents) {
		m.sel = len(st.Agents) - 1
	}
	cur := currentAgentName(st)
	rows := make([]string, 0, len(st.Agents))
	for i, x := range st.Agents {
		line := "  " + x.Name
		if x.Name == cur {
			line += "*"
		}
		if x.Description != "" {
			line += "  " + x.Description
		}
		line = strings.TrimRight(line, " ")
		if i == m.sel {
			rows = append(rows, cursor.Render(line))
		} else {
			rows = append(rows, dim.Render(line))
		}
	}
	b.WriteString(strings.Join(rows, "\n"))
	if m.subChoice {
		b.WriteString("\n" + dim.Render("  [a] this session  [b] set default"))
	}
	b.WriteString("\n" + dim.Render("  \u2191/\u2193 move \u00B7 enter set \u00B7 esc close"))
	return b.String()
}
