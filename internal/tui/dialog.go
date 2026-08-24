package tui

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

var (
	dlgYes = key.NewBinding(key.WithKeys("y", "enter", "ctrl+c"))
	dlgNo  = key.NewBinding(key.WithKeys("n", "esc"))
)

type dialogKind int

const (
	dlgNone dialogKind = iota // zero value: not a real dialog
	dlgQuit
	dlgHelp
	dlgModel
	dlgAgents
)

// dialog is a stack item; the picker dialogs (model/agent) carry their live
// state as the item's payload, so pop drops state with the item.
type dialog struct {
	kind  dialogKind
	model *modelDlg
	agent *agentDlg
}

type dialogStack struct{ items []dialog }

func (d *dialogStack) push(item dialog) { d.items = append(d.items, item) }

func (d *dialogStack) pop() {
	if n := len(d.items); n > 0 {
		d.items = d.items[:n-1]
	}
}

func (d *dialogStack) top() (dialog, bool) {
	n := len(d.items)
	if n == 0 {
		return dialog{}, false
	}
	return d.items[n-1], true
}

func (d dialogStack) empty() bool { return len(d.items) == 0 }

// model returns the open model picker's payload (the openers refuse to stack
// a second one, so at most one item carries it).
func (d *dialogStack) model() *modelDlg {
	for i := range d.items {
		if d.items[i].model != nil {
			return d.items[i].model
		}
	}
	return nil
}

// agent returns the open agent picker's payload (same invariant as model).
func (d *dialogStack) agent() *agentDlg {
	for i := range d.items {
		if d.items[i].agent != nil {
			return d.items[i].agent
		}
	}
	return nil
}

// Static frame parts render once at package init instead of on every frame:
// the styles involved (title, dim, divider) set no width, border, padding,
// or alignment, and lipgloss v2 Style.Render is a pure function of the style
// and the input (SGR output derives from the color type, no terminal state),
// so the results are process-constants. The session-title line in
// viewSession is the only dynamic render left.
var (
	dividerLineRendered = divider.Render(dividerLine())
	sessionHelpRendered = dim.Render(sessionHelp)
	quitDialogRendered  = title.Render("quit? [Y/n]")
	helpDialogRendered  = title.Render("Help") +
		"\n" + dim.Render("  | Key | Action |") +
		"\n" + dim.Render("  |---|---|") +
		"\n" + dim.Render("  | enter | send prompt |") +
		"\n" + dim.Render("  | esc | abort turn (busy) / close dialog |") +
		"\n" + dim.Render("  | ctrl+c | quit (confirm) |") +
		"\n" + dim.Render("  | ctrl+p | model dialog |") +
		"\n" + dim.Render("  | ctrl+a | agent dialog |") +
		"\n" + dim.Render("  | / | command menu |") +
		"\n" + dim.Render("  | pgup/pgdn | viewport scroll |") +
		"\n" + dim.Render("  | 1/2/3 | permission reply |") +
		"\n" + dim.Render("  | alt+e / alt+t | expand tool part / toggle reasoning |") +
		"\n" + dim.Render("  pgup/pgdn scroll \u00B7 \\+enter newline")
)

func (d dialogStack) view() string {
	top, ok := d.top()
	if !ok {
		return ""
	}
	switch top.kind {
	case dlgQuit:
		return quitDialogRendered
	case dlgHelp:
		return helpDialogRendered
	}
	return ""
}

// dlgView renders the top dialog: the model/agent pickers carry their state
// on the stack item, the rest render from the stack alone.
func (a *App) dlgView() string {
	switch d, ok := a.dlg.top(); {
	case !ok:
		return ""
	case d.kind == dlgModel && d.model != nil:
		return d.model.view(&a.store)
	case d.kind == dlgAgents && d.agent != nil:
		return d.agent.view(&a.store)
	}
	return a.dlg.view()
}

func (a *App) handleDialogKey(d dialog, k tea.KeyPressMsg) []tea.Cmd {
	if d.kind == dlgQuit {
		if key.Matches(k, dlgYes) {
			return a.emit(quitCmd())
		}
		if key.Matches(k, dlgNo) {
			a.dlg.pop()
		}
		return nil
	}
	if d.kind == dlgNone {
		a.dlg.pop() // defensive: the zero dialog is not a real dialog
		return nil
	}
	switch d.kind {
	case dlgModel:
		if d.model == nil {
			a.dlg.pop()
			return nil
		}
		return d.model.handleKey(a, k)
	case dlgAgents:
		if d.agent == nil {
			a.dlg.pop()
			return nil
		}
		return d.agent.handleKey(a, k)
	}
	a.dlg.pop() // dlgHelp: any key closes
	return nil
}

// catalogMsg reports the provider + agent catalog fetched when the model or
// agent dialog opens.
type catalogMsg struct {
	provs  []protocol.Provider
	agents []protocol.Agent
	err    error
}

// fetchCatalogCmd lists providers and agents in one cmd.
func (a *App) fetchCatalogCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		provs, perr := a.ListProviders(ctx)
		agents, aerr := a.ListAgents(ctx)
		var err error
		if perr != nil {
			err = perr
		} else if aerr != nil {
			err = aerr
		}
		return catalogMsg{provs: provs, agents: agents, err: err}
	}
}

func (a *App) applyCatalog(m catalogMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	if m.provs != nil {
		a.store.Providers = m.provs
	}
	if m.agents != nil {
		a.store.Agents = m.agents
	}
	a.syncModelSel()
	a.syncAgentSel()
	return nil
}

// dlgPatchMsg reports the result of a model/agent dialog apply; sess is set
// for the "this session" scope, cfg for "set default".
type dlgPatchMsg struct {
	field string // "model" | "agent"
	value string
	sess  *protocol.Session
	cfg   map[string]any
	err   error
}

// patchDlgCmd patches the session or config with the chosen value.
func (a *App) patchDlgCmd(field, value string, thisSession bool) tea.Cmd {
	if thisSession {
		id := a.curSessionID
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ses, err := a.PatchSession(ctx, id, map[string]any{field: value})
			return dlgPatchMsg{field: field, value: value, sess: &ses, err: err}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cfg, err := a.PatchConfig(ctx, map[string]any{field: value})
		return dlgPatchMsg{field: field, value: value, cfg: cfg, err: err}
	}
}

// applyDlgPatch lands a successful apply (store + toast + close); an error
// toasts and keeps the dialog open.
func (a *App) applyDlgPatch(m dlgPatchMsg) tea.Cmd {
	if m.err != nil {
		a.toast(m.err.Error())
		return nil
	}
	if m.sess != nil {
		cp := *m.sess
		a.store.Current = &cp
		a.putSessionFirst(cp)
	}
	if m.cfg != nil {
		a.store.Config = m.cfg
	}
	switch m.field {
	case "agent":
		a.toast("agent set: " + m.value)
		a.closeAgentDialog()
	default:
		a.toast("model set: " + m.value)
		a.closeModelDialog()
	}
	return nil
}

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
	pane         modelPane
	selProv      int
	selModel     int
	hasSubChoice bool
}

var (
	dlgModelKey  = key.NewBinding(key.WithKeys("ctrl+p"))
	dlgAgentsKey = key.NewBinding(key.WithKeys("ctrl+a"))
	dlgTabKey    = key.NewBinding(key.WithKeys("tab"))
	choiceThis   = key.NewBinding(key.WithKeys("a"))
	choiceDef    = key.NewBinding(key.WithKeys("b"))
)

// openModelDialog pushes the model dialog with its payload, seeds its
// selection from the session (or config) model, and fetches the catalog.
func (a *App) openModelDialog() []tea.Cmd {
	mdl := &modelDlg{}
	a.dlg.push(dialog{kind: dlgModel, model: mdl})
	a.syncModelSel()
	return a.emit(a.fetchCatalogCmd())
}

// closeModelDialog pops the dialog; the payload dies with the item.
func (a *App) closeModelDialog() {
	a.dlg.pop()
}

// syncModelSel points the dialog at the session model, falling back to the
// config model; nil-safe when the catalog is not hydrated yet.
func (a *App) syncModelSel() {
	m := a.dlg.model()
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
func (m *modelDlg) currentProv(st *store.State) (protocol.Provider, bool) {
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
func (m *modelDlg) selectedRef(st *store.State) string {
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
func modelIsCurrent(st *store.State, p protocol.Provider, m protocol.Model) bool {
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
		if m.hasSubChoice {
			m.hasSubChoice = false
		} else {
			a.closeModelDialog()
		}
		return nil
	}
	if m.hasSubChoice {
		switch {
		case key.Matches(k, choiceThis):
			if a.curSessionID == "" {
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
			m.hasSubChoice = true
		}
	}
	return nil
}

// move steps the focused pane's selection with wraparound.
func (m *modelDlg) move(st *store.State, d int) {
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
func (m *modelDlg) modelCount(st *store.State) int {
	if p, ok := m.currentProv(st); ok {
		return len(modelsOf(p))
	}
	return 0
}

// view renders the two panes: provider rows (auth dot + status), the selected
// provider's models in the right pane, the subchoice overlay and the keymap
// hint.
func (m *modelDlg) view(st *store.State) string {
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
	if m.hasSubChoice {
		b.WriteString("\n" + dim.Render("  [a] this session  [b] set default"))
	}
	b.WriteString("\n" + dim.Render("  \u2191/\u2193 move \u00B7 tab pane \u00B7 enter set \u00B7 esc close"))
	return b.String()
}

// modelCell renders one right-pane model row (default marker, context, cost).
func (m *modelDlg) modelCell(st *store.State, p protocol.Provider, models []protocol.Model, j int) string {
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
	case auth != nil && auth.RequiresKey && auth.Status == "missing":
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

// agentDlg is the ctrl+a / /agents picker: the server's agent list (name +
// description + current marker). Enter opens the locked [a] this session /
// [b] set default overlay.
type agentDlg struct {
	sel          int
	hasSubChoice bool
}

// openAgentDialog pushes the agent dialog with its payload, seeds its
// selection from the session (or config) agent, and fetches the catalog.
func (a *App) openAgentDialog() []tea.Cmd {
	agd := &agentDlg{}
	a.dlg.push(dialog{kind: dlgAgents, agent: agd})
	a.syncAgentSel()
	return a.emit(a.fetchCatalogCmd())
}

// closeAgentDialog pops the dialog; the payload dies with the item.
func (a *App) closeAgentDialog() {
	a.dlg.pop()
}

// currentAgentName is the session agent, falling back to the config agent.
func currentAgentName(st *store.State) string {
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
	ag := a.dlg.agent()
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
func (m *agentDlg) selectedName(st *store.State) string {
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
		if m.hasSubChoice {
			m.hasSubChoice = false
		} else {
			a.closeAgentDialog()
		}
		return nil
	}
	if m.hasSubChoice {
		switch {
		case key.Matches(k, choiceThis):
			if a.curSessionID == "" {
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
			m.sel = ((m.sel-1)%n + n) % n
		}
	case key.Matches(k, homeKeyMap.Down):
		if n > 0 {
			m.sel = (m.sel + 1) % n
		}
	case key.Matches(k, homeKeyMap.Enter):
		if n > 0 {
			m.hasSubChoice = true
		}
	}
	return nil
}

// view renders the agent list, the subchoice overlay and the keymap hint.
func (m *agentDlg) view(st *store.State) string {
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
	if m.hasSubChoice {
		b.WriteString("\n" + dim.Render("  [a] this session  [b] set default"))
	}
	b.WriteString("\n" + dim.Render("  \u2191/\u2193 move \u00B7 enter set \u00B7 esc close"))
	return b.String()
}
