package tui

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
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
	dlgForm
	dlgPerm
	dlgSessions
	dlgDeleteFailed
	dlgProvider
	dlgStatus
	dlgRetryAction
	dlgThemes
	dlgPalette
	dlgMessage // S7.3: the full-message view (the snapshot payload)
)

// dlgSize is the modal panel width (upstream DialogSize: medium 60, large
// 88, xlarge 116; clamped to the terminal width by the stack).
type dlgSize int

const (
	dlgMedium dlgSize = iota
	dlgLarge
	dlgXLarge
)

// width is the size's panel width in columns.
func (s dlgSize) width() int {
	switch s {
	case dlgLarge:
		return 88
	case dlgXLarge:
		return 116
	default:
		return 60
	}
}

// dialog is a stack item; the picker dialogs (model/agent) carry their live
// state as the item's payload, so pop drops state with the item.
type dialog struct {
	kind         dialogKind
	model        *modelDlg
	agent        *agentDlg
	form         *huhFormDlg                // dlgForm payload (S2.3)
	sel          *selectModel               // S2.5: the select payload (dlgModel/dlgAgents from S2.9/10)
	perm         *permDlg                   // S2.8: the permission payload (dlgPerm)
	sessions     *sessionsDlg               // S3.1: the session picker payload (dlgSessions)
	deleteFailed *deleteFailedDlg           // S3.3: the delete-failed payload (dlgDeleteFailed)
	provider     *providerDlg               // S3.4: the provider picker payload (dlgProvider)
	retry        *retryDlg                  // S3.7: the retry-action payload (dlgRetryAction)
	themes       *themeDlg                  // S3.8: the theme picker payload (dlgThemes)
	message      *protocol.MessageWithParts // S7.3: the full-message payload (dlgMessage)
	modal        bool                       // true: rendered as the overlay frame (S2.2)
	size         dlgSize                    // the panel width, modal only
	onClose      func(*App)                 // the stack-pop callback (upstream result callback)
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

// form returns the open huh-form modal's payload (same invariant as model).
func (d *dialogStack) form() *huhFormDlg {
	for i := range d.items {
		if d.items[i].form != nil {
			return d.items[i].form
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

// sessions returns the open session picker's payload (same invariant as
// model).
func (d *dialogStack) sessions() *sessionsDlg {
	for i := range d.items {
		if d.items[i].sessions != nil {
			return d.items[i].sessions
		}
	}
	return nil
}

// deleteFailed returns the open delete-failed dialog's payload (same
// invariant as model).
func (d *dialogStack) deleteFailed() *deleteFailedDlg {
	for i := range d.items {
		if d.items[i].deleteFailed != nil {
			return d.items[i].deleteFailed
		}
	}
	return nil
}

// provider returns the open provider picker's payload (same invariant as
// model).
func (d *dialogStack) provider() *providerDlg {
	for i := range d.items {
		if d.items[i].provider != nil {
			return d.items[i].provider
		}
	}
	return nil
}

// retryAction returns the open retry-action dialog's payload (same invariant
// as model).
func (d *dialogStack) retryAction() *retryDlg {
	for i := range d.items {
		if d.items[i].retry != nil {
			return d.items[i].retry
		}
	}
	return nil
}

// themes returns the open theme picker's payload (same invariant as model).
func (d *dialogStack) themes() *themeDlg {
	for i := range d.items {
		if d.items[i].themes != nil {
			return d.items[i].themes
		}
	}
	return nil
}

// pushModal pushes a modal item (upstream <Dialog> push): it owns the keys
// until esc/ctrl+c or its own completion closes it; onClose fires when the
// stack pops it.
func (a *App) pushModal(item dialog, size dlgSize, onClose func(*App)) {
	item.modal = true
	item.size = size
	item.onClose = onClose
	a.dlg.push(item)
}

// closeTopModal pops the top modal and fires its onClose (esc/ctrl+c).
func (a *App) closeTopModal() {
	d, ok := a.dlg.top()
	if !ok || !d.modal {
		return
	}
	a.dlg.pop()
	if d.onClose != nil {
		d.onClose(a)
	}
}

// replaceModal closes the top modal (firing its onClose) and pushes the
// replacement (upstream DialogProvider replace).
func (a *App) replaceModal(item dialog, size dlgSize, onClose func(*App)) {
	a.closeTopModal()
	a.pushModal(item, size, onClose)
}

// clearModals closes every modal top-down (non-modal items stop the walk).
func (a *App) clearModals() {
	for {
		d, ok := a.dlg.top()
		if !ok || !d.modal {
			return
		}
		a.dlg.pop()
		if d.onClose != nil {
			d.onClose(a)
		}
	}
}

// modalCanceler is a modal payload that consumes esc for its own inner state
// before the stack closes it (upstream: the model/agent subchoice).
type modalCanceler interface {
	cancelInner(tea.KeyPressMsg) bool
}

// cancelInner consumes esc while the subchoice overlay is open (the next esc
// closes the dialog).
func (m *modelDlg) cancelInner(tea.KeyPressMsg) bool {
	if m.hasSubChoice {
		m.hasSubChoice = false
		return true
	}
	return false
}

// cancelInner is the agentDlg twin.
func (m *agentDlg) cancelInner(tea.KeyPressMsg) bool {
	if m.hasSubChoice {
		m.hasSubChoice = false
		return true
	}
	return false
}

// dialogCanceler is the payload's esc veto, if it has one.
func dialogCanceler(d dialog) (modalCanceler, bool) {
	switch {
	case d.sel != nil:
		return nil, false
	case d.model != nil:
		return d.model, true
	case d.agent != nil:
		return d.agent, true
	}
	return nil, false
}

// syncPermDialog keeps the permission modal in step with the parked asks
// (S2.8): pushed when the first ask parks, popped when the queue drains
// (the reply landed or the permission.replied event dropped it).
func (a *App) syncPermDialog() {
	has := false
	for _, d := range a.dlg.items {
		if d.kind == dlgPerm {
			has = true
			break
		}
	}
	if len(a.store.Pending) > 0 && !has {
		a.pushModal(dialog{kind: dlgPerm, perm: &permDlg{}}, dlgMedium, nil)
		return
	}
	if len(a.store.Pending) == 0 && has {
		for i := len(a.dlg.items) - 1; i >= 0; i-- {
			if a.dlg.items[i].kind == dlgPerm {
				a.dlg.items = append(a.dlg.items[:i], a.dlg.items[i+1:]...)
				break
			}
		}
	}
}

// Static frame parts render once at package init instead of on every frame:
// the styles involved (title, divider) set no width, border, padding, or
// alignment, and lipgloss v2 Style.Render is a pure function of the style
// and the input (SGR output derives from the color type, no terminal state),
// so the results are process-constants. The session-title line in
// viewSession is the only dynamic render left.
var (
	dividerLineRendered = divider.Render(dividerLine())
	quitDialogRendered  = title.Render("quit? [Y/n]")
)

// helpHeaderRow is the help dialog header: the bold "Help" left, the muted
// "esc/enter" right, space-between at the panel width.
func (a *App) helpHeaderRow(w int, th theme.Theme) string {
	const t = "Help"
	pad := w - runeWidth(t) - runeWidth("esc/enter")
	if pad < 0 {
		pad = 0
	}
	return title.Render(t) + strings.Repeat(" ", pad) + th.TextMuted().Render("esc/enter")
}

// helpOKPill renders the right-aligned "ok" pill: pad 0 3, the primary bg and
// the SelectedForeground fg (the yolo token has no selectedListItemText
// → the fallback; 48;5;216 bg + 38;5;232 fg under the pinned test env).
func (a *App) helpOKPill(w int, th theme.Theme) string {
	const label = "ok"
	pill := cursorStyle(th).Padding(0, 3).Render(label)
	if bg, ok := th.Color("primary"); ok {
		fg := lipgloss.Color(th.SelectedForeground(bg).Hex()[:7])
		pill = lipgloss.NewStyle().Foreground(fg).
			Background(lipgloss.Color(bg.Hex()[:7])).Padding(0, 3).Render(label)
	}
	pad := w - ansiWidth(pill)
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + pill
}

// helpDialogView renders the modal help (S3.6, the upstream dialog-help.tsx
// shape): the header row, the muted body (the palette line + the locked V1
// note) and the right-aligned "ok" pill. The pre-S3 markdown table is dropped.
func (a *App) helpDialogView(w, h int, th theme.Theme) string {
	var b strings.Builder
	b.WriteString(a.helpHeaderRow(w, th))
	muted := th.TextMuted()
	b.WriteString("\n" + muted.Render("Press "+a.paletteShortcut()+" to see all available actions and commands in any context."))
	b.WriteString("\n")
	b.WriteString(muted.Render("pgup/pgdn scroll \u00B7 \\+enter newline"))
	b.WriteString("\n" + a.helpOKPill(w, th))
	return b.String()
}

// paletteShortcut is the palette keybind the hint surfaces report (the
// registry-integration seam, deviation 195 resolved at S4.7): the
// registry's command_list binding, formatted for display. The S4.1 default
// is "ctrl+p", so the /help + teatest goldens are byte-identical.
func (a *App) paletteShortcut() string { return a.keymap.Format("command_list") }

func (d dialogStack) view(th theme.Theme) string {
	top, ok := d.top()
	if !ok {
		return ""
	}
	switch top.kind {
	case dlgQuit:
		return quitDialogRendered
	}
	return ""
}

// dlgView renders the top dialog: the model/agent pickers carry their state
// on the stack item, the rest render from the stack alone. The pickers
// word-wrap their rows at the terminal width; the locked quit/help blocks
// stay as-is (short fixed text).
func (a *App) dlgView(w int) string {
	switch d, ok := a.dlg.top(); {
	case !ok:
		return ""
	case d.kind == dlgModel && d.model != nil:
		return d.model.view(&a.store, w, a.size.Height, a.theme)
	case d.kind == dlgAgents && d.agent != nil:
		return d.agent.view(&a.store, w, a.size.Height, a.theme)
	}
	return a.dlg.view(a.theme)
}

// modalInner renders the top modal's payload content at the panel width
// (the stack draws the panel chrome; the payload supplies the inner lines).
func (a *App) modalInner(d *dialog, w, h int) string {
	switch d.kind {
	case dlgModel:
		if d.sel != nil {
			return d.sel.view(w, h, a.theme)
		}
		if d.model != nil {
			return d.model.view(&a.store, w, h, a.theme)
		}
	case dlgAgents:
		if d.sel != nil {
			return d.sel.view(w, h, a.theme)
		}
		if d.agent != nil {
			return d.agent.view(&a.store, w, h, a.theme)
		}
	case dlgForm:
		if d.form != nil {
			return d.form.form.View()
		}
	case dlgPerm:
		if d.perm != nil {
			return d.perm.view(&a.store, w, a.theme)
		}
	case dlgSessions:
		if d.sessions != nil {
			return d.sessions.view(w, h)
		}
	case dlgDeleteFailed:
		if d.deleteFailed != nil {
			return d.deleteFailed.view(w, h, a.theme)
		}
	case dlgProvider:
		if d.provider != nil {
			return d.provider.view(w, h)
		}
	case dlgStatus:
		return a.statusView(w, h, a.theme)
	case dlgRetryAction:
		if d.retry != nil {
			return d.retry.view(w, h)
		}
	case dlgThemes:
		if d.themes != nil {
			return d.themes.view(w, h)
		}
	case dlgHelp:
		return a.helpDialogView(w, h, a.theme)
	case dlgPalette:
		if d.sel != nil {
			return d.sel.view(w, h, a.theme)
		}
	case dlgMessage:
		if d.message != nil {
			return a.messageView(d.message, w, a.theme)
		}
	}
	return ""
}

func (a *App) handleDialogKey(d dialog, k tea.KeyPressMsg) []tea.Cmd {
	if d.modal && (key.Matches(k, escBinding) || key.Matches(k, dlgCtrlC)) {
		if c, ok := dialogCanceler(d); ok && c.cancelInner(k) {
			return nil
		}
		a.closeTopModal()
		return nil
	}
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
		if d.sel != nil {
			return d.sel.handleKey(a, k)
		}
		if d.model == nil {
			a.dlg.pop()
			return nil
		}
		return d.model.handleKey(a, k)
	case dlgAgents:
		if d.sel != nil {
			return d.sel.handleKey(a, k)
		}
		if d.agent == nil {
			a.dlg.pop()
			return nil
		}
		return d.agent.handleKey(a, k)
	case dlgForm:
		if d.form == nil {
			a.dlg.pop()
			return nil
		}
		return d.form.handleKey(a, k)
	case dlgSessions:
		if d.sessions == nil {
			a.dlg.pop()
			return nil
		}
		return d.sessions.handleKey(a, k)
	case dlgDeleteFailed:
		if d.deleteFailed == nil {
			a.dlg.pop()
			return nil
		}
		return d.deleteFailed.handleKey(a, k)
	case dlgProvider:
		if d.provider == nil {
			a.dlg.pop()
			return nil
		}
		return d.provider.handleKey(a, k)
	case dlgRetryAction:
		if d.retry == nil {
			a.dlg.pop()
			return nil
		}
		return d.retry.handleKey(a, k)
	case dlgThemes:
		if d.themes == nil {
			a.dlg.pop()
			return nil
		}
		return d.themes.handleKey(a, k)
	case dlgPalette:
		if d.sel != nil {
			return d.sel.handleKey(a, k)
		}
		a.dlg.pop()
		return nil
	case dlgStatus:
		return nil // static view: the keys are ignored (esc/ctrl+c close via the stack)
	case dlgHelp:
		if key.Matches(k, promptEnter) {
			a.closeTopModal()
		}
		return nil // enter closes; every other key ignored (esc/ctrl+c via the stack)
	}
	a.dlg.pop() // defensive: an unhandled non-modal dialog kind is closed
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
	a.syncProviderSel()
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

// modelDlg is the <leader>m / /model picker (S4.2: the ctrl+p opener frees to
// the command palette; S2.9: the two-pane picker is replaced by the flat
// select + the a/b subchoice — deviation 168).
type modelDlg struct {
	sel          *selectModel
	hasSubChoice bool
	pick         string // the "provider/id" the subchoice applies
}

var (
	choiceThis = key.NewBinding(key.WithKeys("a"))
	choiceDef  = key.NewBinding(key.WithKeys("b"))
)

// openModelDialog pushes the model select modal (dlgLarge — upstream
// dialog-model is size="large") and fetches the catalog. A nil sel = the
// pre-catalog loading hint; syncModelSel builds the select once the catalog
// is hydrated (open-time or on arrival).
func (a *App) openModelDialog() []tea.Cmd {
	mdl := &modelDlg{}
	a.pushModal(dialog{kind: dlgModel, model: mdl}, dlgLarge, nil)
	a.syncModelSel()
	return a.emit(a.fetchCatalogCmd())
}

// closeModelDialog pops the dialog; the payload dies with the item.
func (a *App) closeModelDialog() {
	a.dlg.pop()
}

// syncModelSel (re)builds the open model select on the catalog: it builds
// the select when the catalog first hydrates (nil sel = loading) and
// re-points the selection at the session model (or the config default).
func (a *App) syncModelSel() {
	m := a.dlg.model()
	if m == nil {
		return
	}
	if m.sel == nil {
		if len(a.store.Providers) == 0 {
			return
		}
		m.sel = selectNew("Model", "Search", modelOptions(&a.store),
			modelIsCurrentOpt(&a.store), func(app *App, o selectOption) { app.modelSelectPick(o) }, nil)
	}
	m.sel.options = modelOptions(&a.store)
	if i := modelSelIndex(&a.store); i >= 0 {
		m.sel.sel = i
	}
}

// modelSelectPick is the select's onSelect: enter opens the a/b subchoice
// (yolo pin — the model applies through it, never directly).
func (a *App) modelSelectPick(o selectOption) {
	if d, ok := a.dlg.top(); ok && d.kind == dlgModel && d.model != nil {
		d.model.hasSubChoice = true
		if v, okk := o.value.(string); okk {
			d.model.pick = v
		}
	}
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

// splitModelRef splits a "provider/id" model ref.
func splitModelRef(s string) (pid, mid string, ok bool) {
	i := strings.IndexByte(s, '/')
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// handleKey drives the dialog while it owns the keys: the subchoice owns a/b
// (the locked overlay); esc on the subchoice is the S2.2 cancelInner veto
// (the modal stack consumes it); everything else forwards to the select
// (enter re-opens the subchoice via onSelect on a different model).
func (m *modelDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if m.hasSubChoice {
		switch {
		case key.Matches(k, choiceThis):
			if a.curSessionID == "" {
				a.toast("no session")
				return nil
			}
			return a.emit(a.patchDlgCmd("model", m.pick, true))
		case key.Matches(k, choiceDef):
			return a.emit(a.patchDlgCmd("model", m.pick, false))
		}
		return nil
	}
	if m.sel == nil {
		return nil // the loading hint: nothing to drive until the catalog lands
	}
	return m.sel.handleKey(a, k)
}

// view renders the select + the subchoice overlay (the modal stack draws
// the frame; a nil sel — the pre-catalog open — renders the loading hint).
func (m *modelDlg) view(st *store.State, w, h int, th theme.Theme) string {
	if m.sel == nil {
		return title.Render("Model") + "\n" + th.TextMuted().Render("  loading…")
	}
	var b strings.Builder
	b.WriteString(m.sel.view(w, h, th))
	if m.hasSubChoice {
		b.WriteString("\n" + dimWrapped(th, "  [a] this session  [b] set default", w))
	}
	return b.String()
}

// modelOptions flattens the catalog into select options (providers in
// catalog order, their models in the stable sorted order — modelsOf):
// title = the model name, category = the provider name, description = the
// context + cost tail, footer = the provider status, value = "provider/id".
func modelOptions(st *store.State) []selectOption {
	var opts []selectOption
	for _, p := range st.Providers {
		for _, mm := range modelsOf(p) {
			opts = append(opts, selectOption{
				title:       mm.Name,
				description: fmtCtx(mm.Limit.Context) + " ctx  " + usd(mm.Cost.Input) + "/" + usd(mm.Cost.Output),
				footer:      providerStatusText(p.Auth),
				category:    p.Name,
				value:       p.ID + "/" + mm.ID,
			})
		}
	}
	return opts
}

// modelIsCurrentOpt is the select's isCurrent (value = "provider/id"; the
// session model or the config default).
func modelIsCurrentOpt(st *store.State) func(selectOption) bool {
	var ref protocol.ModelRef
	if cur := st.Current; cur != nil && cur.Model != nil {
		ref = *cur.Model
	} else if s, ok := st.Config["model"].(string); ok {
		if pid, mid, ok := splitModelRef(s); ok {
			ref = protocol.ModelRef{ProviderID: pid, ID: mid}
		}
	}
	return func(o selectOption) bool {
		v, _ := o.value.(string)
		return ref.ProviderID != "" && v == ref.ProviderID+"/"+ref.ID
	}
}

// modelSelIndex is the select index of the current model (-1 = none).
func modelSelIndex(st *store.State) int {
	var ref protocol.ModelRef
	if cur := st.Current; cur != nil && cur.Model != nil {
		ref = *cur.Model
	} else if s, ok := st.Config["model"].(string); ok {
		if pid, mid, ok := splitModelRef(s); ok {
			ref = protocol.ModelRef{ProviderID: pid, ID: mid}
		}
	}
	for i, o := range modelOptions(st) {
		if v, _ := o.value.(string); v == ref.ProviderID+"/"+ref.ID {
			return i
		}
	}
	return -1
}

// dimWrapped word-wraps a plain line at w and renders each visual line in
// the theme's textMuted token (the static dim was removed in S0.9).
func dimWrapped(th theme.Theme, s string, w int) string {
	muted := th.TextMuted()
	var b strings.Builder
	for i, l := range strings.Split(wrapLine(s, w), "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(muted.Render(l))
	}
	return b.String()
}

// providerStatusText is the locked dot + label (the select footer tail).
func providerStatusText(auth *protocol.ProviderAuth) string {
	switch {
	case auth != nil && auth.Status == "loaded":
		return "● loaded"
	case auth != nil && auth.RequiresKey && auth.Status == "missing":
		return "○ missing"
	default:
		return "· not-required"
	}
}

// providerStatus maps the wire auth state to the dot + label style (the
// transcript surfaces; the select tail uses providerStatusText plain).
func providerStatus(th theme.Theme, auth *protocol.ProviderAuth) (string, lipgloss.Style) {
	switch {
	case auth != nil && auth.Status == "loaded":
		return providerStatusText(auth), th.Success()
	case auth != nil && auth.RequiresKey && auth.Status == "missing":
		return providerStatusText(auth), th.Error()
	default:
		return providerStatusText(auth), th.TextMuted()
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

// agentDlg is the <leader>a / /agents picker (S4.2: the ctrl+a opener frees to
// the prompt input; S2.10: the plain list is the select + the a/b subchoice —
// the yolo scope pin; upstream applies directly).
type agentDlg struct {
	sel          *selectModel
	hasSubChoice bool
	pick         string // the agent name the subchoice applies
}

// openAgentDialog pushes the agent select modal (dlgMedium — upstream
// dialog-agent is size="medium") and fetches the catalog. A nil sel = the
// pre-catalog loading hint; syncAgentSel builds the select once the catalog
// is hydrated (open-time or on arrival).
func (a *App) openAgentDialog() []tea.Cmd {
	agd := &agentDlg{}
	a.pushModal(dialog{kind: dlgAgents, agent: agd}, dlgMedium, nil)
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

// agentOptions is the select's option list: title = the name, description
// = the agent description, value = the name (the wire value).
func agentOptions(st *store.State) []selectOption {
	var opts []selectOption
	for _, x := range st.Agents {
		opts = append(opts, selectOption{
			title:       x.Name,
			description: x.Description,
			value:       x.Name,
		})
	}
	return opts
}

// agentIsCurrentOpt is the select's isCurrent (the session agent, falling
// back to the config agent).
func agentIsCurrentOpt(st *store.State) func(selectOption) bool {
	name := currentAgentName(st)
	return func(o selectOption) bool {
		v, _ := o.value.(string)
		return name != "" && v == name
	}
}

// agentSelectPick is the select's onSelect: enter opens the a/b subchoice
// (yolo pin — the agent applies through it, never directly).
func (a *App) agentSelectPick(o selectOption) {
	if d, ok := a.dlg.top(); ok && d.kind == dlgAgents && d.agent != nil {
		d.agent.hasSubChoice = true
		if v, okk := o.value.(string); okk {
			d.agent.pick = v
		}
	}
}

// syncAgentSel (re)builds the open agent select on the catalog: it builds
// the select when the catalog first hydrates (nil sel = loading) and
// re-points the selection at the current agent.
func (a *App) syncAgentSel() {
	ag := a.dlg.agent()
	if ag == nil {
		return
	}
	if ag.sel == nil {
		if len(a.store.Agents) == 0 {
			return
		}
		ag.sel = selectNew("Agents", "Search", agentOptions(&a.store),
			agentIsCurrentOpt(&a.store), func(app *App, o selectOption) { app.agentSelectPick(o) }, nil)
	}
	ag.sel.options = agentOptions(&a.store)
	name := currentAgentName(&a.store)
	for i, x := range a.store.Agents {
		if x.Name == name {
			ag.sel.sel = i
			return
		}
	}
}

// handleKey drives the dialog while it owns the keys: the subchoice owns a/b
// (the locked overlay); esc on the subchoice is the S2.2 cancelInner veto
// (the modal stack consumes it); everything else forwards to the select
// (enter re-opens the subchoice via onSelect on a different agent).
func (m *agentDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if m.hasSubChoice {
		switch {
		case key.Matches(k, choiceThis):
			if a.curSessionID == "" {
				a.toast("no session")
				return nil
			}
			return a.emit(a.patchDlgCmd("agent", m.pick, true))
		case key.Matches(k, choiceDef):
			return a.emit(a.patchDlgCmd("agent", m.pick, false))
		}
		return nil
	}
	if m.sel == nil {
		return nil // the loading hint: nothing to drive until the catalog lands
	}
	return m.sel.handleKey(a, k)
}

// view renders the select + the subchoice overlay (the modal stack draws
// the frame; a nil sel — the pre-catalog open — renders the loading hint).
func (m *agentDlg) view(st *store.State, w, h int, th theme.Theme) string {
	if m.sel == nil {
		return title.Render("Agents") + "\n" + th.TextMuted().Render("  loading…")
	}
	var b strings.Builder
	b.WriteString(m.sel.view(w, h, th))
	if m.hasSubChoice {
		b.WriteString("\n" + dimWrapped(th, "  [a] this session  [b] set default", w))
	}
	return b.String()
}
