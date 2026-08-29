package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// model_test.go — the S2.9 restyle: the flat model select (deviation 168)
// + the yolo-pinned a/b subchoice.

// pressCtrlP / pressCtrlA build the locked dialog openers (ctrl modifiers, no
// Text).
func pressCtrlP() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl} }
func pressCtrlA() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl} }

// providerFixture mirrors the offline server fixture (provider.
// NewStaticForTest): kido (key-less, Qwen 100k) and opencode (key-required,
// minimal zen catalog).
func providerFixture() []protocol.Provider {
	return []protocol.Provider{
		{
			ID: "kido", Name: "Kido",
			Models: map[string]protocol.Model{
				"q": {ID: "q", ProviderID: "kido", Name: "Qwen", Limit: protocol.ModelLimit{Context: 100000}},
			},
			Auth: &protocol.ProviderAuth{Type: "api", Status: "not-required"},
		},
		{
			ID: "opencode", Name: "OpenCode Zen",
			Models: map[string]protocol.Model{
				"claude-opus-4-7": {
					ID:         "claude-opus-4-7",
					ProviderID: "opencode",
					Name:       "Claude Opus 4.7",
					Limit:      protocol.ModelLimit{Context: 200000},
				},
				"gpt-5-nano": {
					ID:         "gpt-5-nano",
					ProviderID: "opencode",
					Name:       "GPT-5 Nano",
					Limit:      protocol.ModelLimit{Context: 400000},
				},
			},
			Auth: &protocol.ProviderAuth{Type: "api", Status: "missing", RequiresKey: true},
		},
	}
}

// agentFixture mirrors the server baseAgents.
func agentFixture() []protocol.Agent {
	return []protocol.Agent{
		{Name: "build", Description: "The default agent. Executes tools based on configured permissions."},
		{Name: "plan", Description: "Plan mode. Disallows all edit tools."},
		{Name: "yolo", Description: "Yolo agent. Permits everything without prompts."},
	}
}

// modelFixture builds a session-route app with the offline catalog hydrated.
func modelFixture() *recApp {
	a := testApp()
	a.store.Current = &protocol.Session{ID: "ses_1", Agent: "build", Model: refModel("kido", "q")}
	a.store.Providers = providerFixture()
	a.store.Agents = agentFixture()
	a.store.Config = map[string]any{"model": "kido/q"}
	a.route = routeSession
	a.curSessionID = "ses_1"
	return a
}

// openModelAt opens the model dialog and resets the recorded cmds.
func openModelAt() *recApp {
	a := modelFixture()
	a.openModelDialog()
	a.Cmds = nil
	return a
}

func TestModelDialogRender(t *testing.T) {
	t.Run("catalog flattens into the select, the current model is marked", func(t *testing.T) {
		a := openModelAt()
		got := stripANSI(a.dlg.model().view(&a.store, 80, 24, a.theme))
		lines := strings.Split(got, "\n")
		if !strings.Contains(lines[0], "Model") {
			t.Fatalf("title row = %q", lines[0])
		}
		if !strings.Contains(lines[1], "Search") {
			t.Fatalf("filter row = %q", lines[1])
		}
		// category headers + the rows (the zero-theme renders plain)
		if !strings.Contains(got, "   Kido") || !strings.Contains(got, "   OpenCode Zen") {
			t.Fatalf("category headers missing:\n%s", got)
		}
		// the current model: the ● gutter + the active-row title + the ctx/cost tail
		var qwenLine string
		for _, l := range lines {
			if strings.Contains(l, "Qwen") {
				qwenLine = l
			}
		}
		if !strings.Contains(qwenLine, "●") || !strings.Contains(qwenLine, "100k ctx") || !strings.Contains(qwenLine, "$0/$0") {
			t.Fatalf("current model row = %q", qwenLine)
		}
		if !strings.Contains(got, "Claude Opus 4.7") || !strings.Contains(got, "GPT-5 Nano") {
			t.Fatalf("catalog models missing:\n%s", got)
		}
	})

	t.Run("no providers renders the loading hint", func(t *testing.T) {
		a := modelFixture()
		a.store.Providers = nil
		a.openModelDialog()
		got := stripANSI(a.dlg.model().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "Model") || !strings.Contains(got, "loading…") {
			t.Fatalf("loading hint missing:\n%s", got)
		}
	})

	t.Run("filter narrows the list", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press('g')) // only "GPT-5 Nano" matches
		got := stripANSI(a.dlg.model().view(&a.store, 80, 24, a.theme))
		if strings.Contains(got, "Qwen") || strings.Contains(got, "Claude") {
			t.Fatalf("filter did not narrow:\n%s", got)
		}
		if !strings.Contains(got, "GPT-5 Nano") {
			t.Fatalf("filtered row missing:\n%s", got)
		}
	})

	t.Run("subchoice line is the locked [a]/[b] overlay", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyEnter)) // the current (selected) model
		got := stripANSI(a.dlg.model().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "[a] this session  [b] set default") {
			t.Fatalf("subchoice missing:\n%s", got)
		}
	})
}

func TestModelDialogKeys(t *testing.T) {
	t.Run("esc closes the subchoice first, then the dialog", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press(tea.KeyEscape))
		if a.dlg.model().hasSubChoice {
			t.Fatalf("first esc must close the subchoice only")
		}
		if a.dlg.empty() {
			t.Fatalf("the dialog must stay open")
		}
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() {
			t.Fatalf("second esc must close the dialog")
		}
	})

	t.Run("a/b apply through the existing patch flow", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyDown)) // to the next model (GPT-5 Nano? no — next option: Claude Opus 4.7)
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press('b')) // set default
		// the cmd chain is the existing patchDlgCmd (config patch)
		if len(a.Cmds) == 0 {
			t.Fatalf("b must emit the patch cmd")
		}
	})
}

func TestModelDialogApply(t *testing.T) {
	t.Run("session patch: success toasts, closes, and updates current", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyDown))  // claude-opus-4-7
		a.handleKey(press(tea.KeyDown))  // gpt-5-nano
		a.handleKey(press(tea.KeyEnter)) // open the subchoice
		a.handleKey(press('a'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1", len(a.Cmds))
		}
		a.applyDlgPatch(dlgPatchMsg{field: "model", value: "opencode/gpt-5-nano",
			sess: &protocol.Session{ID: "ses_1", Agent: "build", Model: refModel("opencode", "gpt-5-nano")}})
		if !a.dlg.empty() || a.dlg.model() != nil {
			t.Fatal("dialog must close after a successful session patch")
		}
		if !hasToast(a, "model set: opencode/gpt-5-nano") {
			t.Fatalf("toasts = %+v, want the model-set toast", a.toasts)
		}
		if got := a.store.Current.Model; got == nil || got.ID != "gpt-5-nano" || got.ProviderID != "opencode" {
			t.Fatalf("current model = %+v, want opencode/gpt-5-nano", got)
		}
	})

	t.Run("default patch: success updates the config model", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press('b'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1", len(a.Cmds))
		}
		a.applyDlgPatch(dlgPatchMsg{field: "model", value: "opencode/gpt-5-nano",
			cfg: map[string]any{"model": "opencode/gpt-5-nano"}})
		if !a.dlg.empty() {
			t.Fatal("dialog must close after a successful default patch")
		}
		if !hasToast(a, "model set: opencode/gpt-5-nano") {
			t.Fatalf("toasts = %+v, want the model-set toast", a.toasts)
		}
		if got := a.store.Config["model"]; got != "opencode/gpt-5-nano" {
			t.Fatalf("config model = %v, want opencode/gpt-5-nano", got)
		}
	})

	t.Run("error toasts and keeps the dialog", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyEnter))
		a.applyDlgPatch(dlgPatchMsg{field: "model", value: "opencode/gpt-5-nano", err: errors.New("boom")})
		if !hasToast(a, "boom") {
			t.Fatalf("toasts = %+v, want boom", a.toasts)
		}
		if a.dlg.empty() {
			t.Fatal("dialog must stay open after a failed patch")
		}
	})

	t.Run("'a' with no session toasts no-session", func(t *testing.T) {
		a := modelFixture()
		a.route = routeHome
		a.curSessionID = ""
		a.store.Current = nil
		a.openModelDialog()
		a.Cmds = nil
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press('a'))
		if len(a.Cmds) != 0 {
			t.Fatalf("recorded %d cmds, want none without a session", len(a.Cmds))
		}
		if !hasToast(a, "no session") {
			t.Fatalf("toasts = %+v, want no session", a.toasts)
		}
	})
}

func TestModelDialogOpen(t *testing.T) {
	t.Run("ctrl+p opens the model dialog", func(t *testing.T) {
		a := modelFixture()
		a.handleKey(pressCtrlP())
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgModel || d.model == nil {
			t.Fatalf("after ctrl+p: top=%+v modelDlg=%v, want the model dialog", d, d.model)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("ctrl+p emitted %d cmds, want the catalog fetch", len(a.Cmds))
		}
	})

	t.Run("/model opens the model dialog", func(t *testing.T) {
		a := modelFixture()
		a.runCommand("/model")
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgModel || d.model == nil {
			t.Fatalf("after /model: top=%+v modelDlg=%v, want the model dialog", d, d.model)
		}
	})

	t.Run("ctrl+p is ignored while a dialog is on top", func(t *testing.T) {
		a := modelFixture()
		a.dlg.push(dialog{kind: dlgQuit})
		a.handleKey(pressCtrlP())
		d, _ := a.dlg.top()
		if d.kind != dlgQuit || a.dlg.model() != nil {
			t.Fatalf("ctrl+p must not stack dialogs: top=%+v modelDlg=%v", d, a.dlg.model())
		}
	})

	t.Run("catalog msg hydrates the store and re-syncs the selection", func(t *testing.T) {
		a := modelFixture()
		a.store.Providers = nil
		a.store.Agents = nil
		a.openModelDialog()
		a.applyCatalog(catalogMsg{provs: providerFixture(), agents: agentFixture()})
		if len(a.store.Providers) != 2 || len(a.store.Agents) != 3 {
			t.Fatalf("store = %d providers / %d agents, want 2 / 3", len(a.store.Providers), len(a.store.Agents))
		}
		sel := a.dlg.model().sel
		if sel == nil || sel.sel != 0 {
			t.Fatalf("after catalog selection = %v, want 0 (config model kido/q)", sel)
		}
	})

	t.Run("catalog error toasts", func(t *testing.T) {
		a := modelFixture()
		a.openModelDialog()
		a.applyCatalog(catalogMsg{err: errors.New("nope")})
		if !hasToast(a, "nope") {
			t.Fatalf("toasts = %+v, want nope", a.toasts)
		}
	})
}

// TestTUIModelDialog is the teatest scenario: open the model dialog with
// ctrl+p (the offline server fixture), filter to GPT-5 Nano, enter → the
// subchoice, and set it for this session with [a].
func TestTUIModelDialog(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	ctx := context.Background()
	ses, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := newRecApp(c, store.State{}, ses.ID)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))

	tm.Send(pressCtrlP())
	teatest.WaitFor(t, tm.Output(), hasModelDialog, teatest.WithDuration(5*time.Second))

	// type the filter to GPT-5 Nano, enter → the subchoice, a = this session
	tm.Send(press('g'))
	tm.Send(press(tea.KeyEnter))
	tm.Send(press('a'))

	teatest.WaitFor(t, tm.Output(), hasLine("model set: opencode/gpt-5-nano"), teatest.WithDuration(5*time.Second))

	got, err := c.GetSession(ctx, ses.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Model == nil || got.Model.ProviderID != "opencode" || got.Model.ID != "gpt-5-nano" {
		t.Fatalf("session model = %+v, want opencode/gpt-5-nano", got.Model)
	}

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func hasModelDialog(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "Kido") &&
		strings.Contains(s, "OpenCode Zen") &&
		strings.Contains(s, "● Qwen") &&
		strings.Contains(s, "100k ctx")
}

// TestModelDialogPatchPaths executes the subchoice cmds end-to-end
// (testing-2): 'b' (set default) must PATCH the config's model field, 'a'
// (this session) must PATCH the session's model field. The existing
// subtest only counts cmds — a 'b' wired to the session-patch path passed
// everything before this pin.
func TestModelDialogPatchPaths(t *testing.T) {
	ts := testutil.Boot(t)
	ctx := context.Background()
	c := client.New(ts.URL, ts.Dir)
	ses, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := newRecApp(c, store.State{
		Current:   &protocol.Session{ID: ses.ID, Agent: "build", Model: refModel("kido", "q")},
		Providers: providerFixture(),
		Agents:    agentFixture(),
		Config:    map[string]any{"model": "kido/q"},
	}, "")
	a.route = routeSession
	a.curSessionID = ses.ID
	t.Cleanup(a.Close)

	open := func() {
		a.openModelDialog()
		a.Cmds = nil
		a.handleKey(press(tea.KeyEnter)) // open the subchoice on the current model
	}

	t.Run("b sets the config default model", func(t *testing.T) {
		open()
		a.handleKey(press('b'))
		if len(a.Cmds) != 1 {
			t.Fatalf("key b emitted %d cmds, want 1", len(a.Cmds))
		}
		m := a.Cmds[0]()
		pm, ok := m.(dlgPatchMsg)
		if !ok || pm.err != nil {
			t.Fatalf("b cmd delivered %v (%T), want a successful dlgPatchMsg", pm, m)
		}
		if pm.field != "model" || pm.sess != nil || pm.cfg == nil {
			t.Fatalf("b must PATCH the config: field=%q sess=%v cfg=%v", pm.field, pm.sess, pm.cfg)
		}
		if got, _ := pm.cfg["model"].(string); got != "kido/q" {
			t.Fatalf("config PATCH model = %q, want kido/q", got)
		}
		if a.dlg.empty() {
			t.Fatal("dialog must stay open before the patch msg lands")
		}
		a.Update(pm)
		if !a.dlg.empty() {
			t.Fatal("dialog must close after the patch msg lands")
		}
	})

	t.Run("a sets the session model", func(t *testing.T) {
		open()
		a.handleKey(press('a'))
		if len(a.Cmds) != 1 {
			t.Fatalf("key a emitted %d cmds, want 1", len(a.Cmds))
		}
		m := a.Cmds[0]()
		pm, ok := m.(dlgPatchMsg)
		if !ok || pm.err != nil {
			t.Fatalf("a cmd delivered %v (%T), want a successful dlgPatchMsg", pm, m)
		}
		if pm.field != "model" || pm.cfg != nil || pm.sess == nil {
			t.Fatalf("a must PATCH the session: field=%q cfg=%v sess=%v", pm.field, pm.cfg, pm.sess)
		}
		if pm.sess.ID != ses.ID {
			t.Fatalf("session PATCH id = %q, want %q", pm.sess.ID, ses.ID)
		}
		if pm.sess.Model == nil || pm.sess.Model.ID != "q" || pm.sess.Model.ProviderID != "kido" {
			t.Fatalf("session model after PATCH = %+v, want kido/q", pm.sess.Model)
		}
		a.Update(pm)
		if !a.dlg.empty() {
			t.Fatal("dialog must close after the patch msg lands")
		}
	})
}
