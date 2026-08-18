package tui

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// modelPane selects which dialog pane the list keys drive.
type modelPane int

const (
	paneProviders modelPane = iota
	paneModels
)

// modelDlg is the ctrl+p / /model two-pane picker: providers on the left with
// their auth dot, the selected provider's models on the right with context and
// cost. Enter on a model opens the locked [a] this session / [b] set default
// overlay.
type modelDlg struct {
	pane      modelPane
	selProv   int
	selModel  int
	subChoice bool
}

var (
	dlgModelKey  = key.NewBinding(key.WithKeys("ctrl+p"))
	dlgAgentsKey = key.NewBinding(key.WithKeys("ctrl+a"))
	dlgTabKey    = key.NewBinding(key.WithKeys("tab"))
	choiceThis   = key.NewBinding(key.WithKeys("a"))
	choiceDef    = key.NewBinding(key.WithKeys("b"))
)

// openModelDialog pushes the model dialog, seeds its selection from the
// session (or config) model, and fetches the catalog.
func (a *App) openModelDialog() []tea.Cmd {
	a.modelDlg = &modelDlg{}
	a.syncModelSel()
	a.dlg.push(dialog{kind: dlgModel})
	return a.emit(a.fetchCatalogCmd())
}

// closeModelDialog pops the dialog and drops its state.
func (a *App) closeModelDialog() {
	a.dlg.pop()
	a.modelDlg = nil
}

// syncModelSel points the dialog at the session model, falling back to the
// config model; nil-safe when the catalog is not hydrated yet.
func (a *App) syncModelSel() {
	m := a.modelDlg
	if m == nil {
		return
	}
	var ref protocol.ModelRef
	if cur := a.store.Current; cur != nil && cur.Model != nil {
		ref = *cur.Model
	} else if s, ok := a.store.Config["model"].(string); ok {
		if pid, mid, ok := splitModelRef(s); ok {
			ref = protocol.ModelRef{ProviderID: pid, ID: mid}
		}
	}
	for i, p := range a.store.Providers {
		if p.ID != ref.ProviderID {
			continue
		}
		m.selProv = i
		for j, mm := range modelsOf(p) {
			if mm.ID == ref.ID {
				m.selModel = j
			}
		}
		return
	}
}

// currentProv is the selected provider, if the index is in range.
func (m *modelDlg) currentProv(st *store.Store) (protocol.Provider, bool) {
	if m.selProv >= 0 && m.selProv < len(st.Providers) {
		return st.Providers[m.selProv], true
	}
	return protocol.Provider{}, false
}

// modelsOf returns the provider's models in stable (sorted id) order.
func modelsOf(p protocol.Provider) []protocol.Model {
	ids := make([]string, 0, len(p.Models))
	for id := range p.Models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]protocol.Model, 0, len(ids))
	for _, id := range ids {
		out = append(out, p.Models[id])
	}
	return out
}

// selectedRef is the "provider/id" wire value of the selected model.
func (m *modelDlg) selectedRef(st *store.Store) string {
	p, ok := m.currentProv(st)
	if !ok {
		return ""
	}
	ms := modelsOf(p)
	if len(ms) == 0 {
		return ""
	}
	if m.selModel < 0 || m.selModel >= len(ms) {
		m.selModel = 0
	}
	return p.ID + "/" + ms[m.selModel].ID
}

// modelIsCurrent reports whether the model is the session model or the config
// default (the row gets the "*" marker).
func modelIsCurrent(st *store.Store, p protocol.Provider, m protocol.Model) bool {
	if cur := st.Current; cur != nil && cur.Model != nil && cur.Model.ProviderID == p.ID && cur.Model.ID == m.ID {
		return true
	}
	if s, ok := st.Config["model"].(string); ok && s != "" {
		if pid, mid, ok := splitModelRef(s); ok && pid == p.ID && mid == m.ID {
			return true
		}
	}
	return false
}

// splitModelRef splits a "provider/id" model ref.
func splitModelRef(s string) (pid, mid string, ok bool) {
	i := strings.IndexByte(s, '/')
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// handleKey drives the dialog while it owns the keys: esc closes (or cancels
// the subchoice), tab switches panes (LOCKED), arrows move with wraparound,
// enter opens the subchoice on the models pane, a/b apply (LOCKED overlay).
func (m *modelDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if key.Matches(k, escBinding) {
		if m.subChoice {
			m.subChoice = false
		} else {
			a.closeModelDialog()
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
			return a.emit(a.patchDlgCmd("model", m.selectedRef(&a.store), true))
		case key.Matches(k, choiceDef):
			return a.emit(a.patchDlgCmd("model", m.selectedRef(&a.store), false))
		}
		return nil
	}
	switch {
	case key.Matches(k, dlgTabKey):
		if m.pane == paneProviders {
			m.pane = paneModels
		} else {
			m.pane = paneProviders
		}
	case key.Matches(k, homeKeyMap.Up):
		m.move(&a.store, -1)
	case key.Matches(k, homeKeyMap.Down):
		m.move(&a.store, 1)
	case key.Matches(k, homeKeyMap.Enter):
		if m.pane == paneModels && m.modelCount(&a.store) > 0 {
			m.subChoice = true
		}
	}
	return nil
}

// move steps the focused pane's selection with wraparound.
func (m *modelDlg) move(st *store.Store, d int) {
	if m.pane == paneProviders {
		n := len(st.Providers)
		if n == 0 {
			return
		}
		m.selProv = ((m.selProv+d)%n + n) % n
		return
	}
	n := m.modelCount(st)
	if n == 0 {
		return
	}
	m.selModel = ((m.selModel+d)%n + n) % n
}

// modelCount is the length of the selected provider's model list.
func (m *modelDlg) modelCount(st *store.Store) int {
	if p, ok := m.currentProv(st); ok {
		return len(modelsOf(p))
	}
	return 0
}

// view renders the two panes: provider rows (auth dot + status), the selected
// provider's models in the right pane, the subchoice overlay and the keymap
// hint.
func (m *modelDlg) view(st *store.Store) string {
	var b strings.Builder
	b.WriteString(title.Render("Model") + "\n")
	provs := st.Providers
	if len(provs) == 0 {
		b.WriteString(dim.Render("  loading…"))
		return b.String()
	}
	if m.selProv >= len(provs) {
		m.selProv = len(provs) - 1
	}
	curProv, ok := m.currentProv(st)
	var models []protocol.Model
	if ok {
		models = modelsOf(curProv)
	}
	if m.selModel >= len(models) {
		m.selModel = 0
	}
	leftCol := 0
	for _, p := range provs {
		sPlain, _ := providerStatus(p.Auth)
		if l := len("  "+p.Name+"  ") + utf8.RuneCountInString(sPlain); l > leftCol {
			leftCol = l
		}
	}
	leftCol += 2
	rows := make([]string, 0, len(provs)+len(models))
	for i, p := range provs {
		sPlain, sStyled := providerStatus(p.Auth)
		name := "  " + p.Name + "  "
		row := name + sStyled + strings.Repeat(" ", leftCol-len(name)-utf8.RuneCountInString(sPlain))
		switch {
		case i == m.selProv && len(models) > 0:
			// The cell follows the pad, so no trailing spaces to trim.
			row = cursor.Render(row + m.modelCell(st, curProv, models, 0))
		default:
			// Trim before styling: the style's trailing SGR reset would
			// otherwise survive TrimRight as a visible trailing space.
			row = strings.TrimRight(row, " ")
			if i == m.selProv {
				row = cursor.Render(row)
			} else {
				row = dim.Render(row)
			}
		}
		rows = append(rows, row)
		if i == m.selProv {
			for j := 1; j < len(models); j++ {
				rows = append(rows, strings.Repeat(" ", leftCol)+m.modelCell(st, curProv, models, j))
			}
		}
	}
	b.WriteString(strings.Join(rows, "\n"))
	if m.subChoice {
		b.WriteString("\n" + dim.Render("  [a] this session  [b] set default"))
	}
	b.WriteString("\n" + dim.Render("  \u2191/\u2193 move \u00B7 tab pane \u00B7 enter set \u00B7 esc close"))
	return b.String()
}

// modelCell renders one right-pane model row (default marker, context, cost).
func (m *modelDlg) modelCell(st *store.Store, p protocol.Provider, models []protocol.Model, j int) string {
	mm := models[j]
	cell := mm.Name
	if modelIsCurrent(st, p, mm) {
		cell += "*"
	}
	cell += "  " + fmtCtx(mm.Limit.Context) + " ctx  " + usd(mm.Cost.Input) + "/" + usd(mm.Cost.Output)
	if j == m.selModel {
		return cursor.Render(cell)
	}
	return dim.Render(cell)
}

// providerStatus maps the wire auth state to the locked dot + label.
func providerStatus(auth *protocol.ProviderAuth) (plain, styled string) {
	switch {
	case auth != nil && auth.Status == "loaded":
		return "● loaded", okGreen.Render("● loaded")
	case auth != nil && auth.KeyRequired && auth.Status == "missing":
		return "○ missing", errRed.Render("○ missing")
	default:
		return "· not-required", dim.Render("· not-required")
	}
}

// fmtCtx renders a context size: 100000 → "100k".
func fmtCtx(n int) string {
	if n >= 1000 {
		return strconv.Itoa(n/1000) + "k"
	}
	return strconv.Itoa(n)
}

// usd renders a per-million price without trailing zeros: 2 → "$2".
func usd(v float64) string { return "$" + strconv.FormatFloat(v, 'f', -1, 64) }
