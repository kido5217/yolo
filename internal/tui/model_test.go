package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// pressTab builds a synthetic tab keypress (Text must stay empty or String()
// stops matching the "tab" binding).
func pressTab() tea.KeyPressMsg { return tea.KeyPressMsg{Code: '\t'} }

// pressCtrlP / pressCtrlA build the locked dialog openers (ctrl modifiers, no
// Text).
func pressCtrlP() tea.KeyPressMsg  { return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl} }
func pressCtrlA() tea.KeyPressMsg  { return tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl} }

// tuiProviderFixture mirrors the offline server fixture (provider.
// NewStaticForTest): kido (key-less, Qwen 100k) and opencode (key-required,
// minimal zen catalog).
func tuiProviderFixture() []protocol.Provider {
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
				"claude-opus-4-7": {ID: "claude-opus-4-7", ProviderID: "opencode", Name: "Claude Opus 4.7", Limit: protocol.ModelLimit{Context: 200000}},
				"gpt-5-nano":      {ID: "gpt-5-nano", ProviderID: "opencode", Name: "GPT-5 Nano", Limit: protocol.ModelLimit{Context: 400000}},
			},
			Auth: &protocol.ProviderAuth{Type: "api", Status: "missing", KeyRequired: true},
		},
	}
}

// tuiAgentFixture mirrors the server baseAgents.
func tuiAgentFixture() []protocol.Agent {
	return []protocol.Agent{
		{Name: "build", Description: "The default agent. Executes tools based on configured permissions."},
		{Name: "plan", Description: "Plan mode. Disallows all edit tools."},
		{Name: "yolo", Description: "Yolo agent. Permits everything without prompts."},
	}
}

// modelFixture builds a session-route app with the offline catalog hydrated.
func modelFixture() *App {
	a := testApp()
	a.store.Current = &protocol.Session{ID: "ses_1", Agent: "build", Model: refModel("kido", "q")}
	a.store.Providers = tuiProviderFixture()
	a.store.Agents = tuiAgentFixture()
	a.store.Config = map[string]any{"model": "kido/q"}
	a.route = routeSession
	a.cur = "ses_1"
	return a
}

// openModelAt opens the model dialog and resets the recorded cmds.
func openModelAt() *App {
	a := modelFixture()
	a.openModelDialog()
	a.Cmds = nil
	return a
}

func modelBlock(t *testing.T, a *App, want string) {
	t.Helper()
	if got := stripANSI(a.modelDlg.view(&a.store)); got != want {
		t.Errorf("model dialog mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestModelDialogRender(t *testing.T) {
	t.Run("current model is marked and its provider selected", func(t *testing.T) {
		a := openModelAt()
		want := "Model\n" +
			"  Kido  · not-required     Qwen*  100k ctx  $0/$0\n" +
			"  OpenCode Zen  ○ missing\n" +
			"  ↑/↓ move · tab pane · enter set · esc close"
		modelBlock(t, a, want)
	})

	t.Run("down selects the next provider; its models fill the right pane", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyDown))
		want := "Model\n" +
			"  Kido  · not-required\n" +
			"  OpenCode Zen  ○ missing  Claude Opus 4.7  200k ctx  $0/$0\n" +
			strings.Repeat(" ", 27) + "GPT-5 Nano  400k ctx  $0/$0\n" +
			"  ↑/↓ move · tab pane · enter set · esc close"
		modelBlock(t, a, want)
	})

	t.Run("subchoice line is the locked [a]/[b] overlay", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyDown))
		a.handleKey(pressTab())
		a.handleKey(press(tea.KeyEnter))
		want := "Model\n" +
			"  Kido  · not-required\n" +
			"  OpenCode Zen  ○ missing  Claude Opus 4.7  200k ctx  $0/$0\n" +
			strings.Repeat(" ", 27) + "GPT-5 Nano  400k ctx  $0/$0\n" +
			"  [a] this session  [b] set default\n" +
			"  ↑/↓ move · tab pane · enter set · esc close"
		modelBlock(t, a, want)
	})

	t.Run("config model marks the default when no session exists", func(t *testing.T) {
		a := openModelAt()
		a.store.Current = nil
		want := "Model\n" +
			"  Kido  · not-required     Qwen*  100k ctx  $0/$0\n" +
			"  OpenCode Zen  ○ missing\n" +
			"  ↑/↓ move · tab pane · enter set · esc close"
		modelBlock(t, a, want)
	})

	t.Run("no providers renders the loading hint", func(t *testing.T) {
		a := modelFixture()
		a.store.Providers = nil
		a.openModelDialog()
		modelBlock(t, a, "Model\n  loading…")
	})
}

func TestModelDialogKeys(t *testing.T) {
	t.Run("down/up move the provider with wraparound", func(t *testing.T) {
		a := openModelAt()
		if got := a.modelDlg.selProv; got != 0 {
			t.Fatalf("initial selProv = %d, want 0 (current model's provider)", got)
		}
		a.handleKey(press(tea.KeyDown))
		if got := a.modelDlg.selProv; got != 1 {
			t.Fatalf("after down selProv = %d, want 1", got)
		}
		a.handleKey(press(tea.KeyUp))
		if got := a.modelDlg.selProv; got != 0 {
			t.Fatalf("after up selProv = %d, want 0", got)
		}
		a.handleKey(press(tea.KeyUp)) // wraps to the last provider
		if got := a.modelDlg.selProv; got != 1 {
			t.Fatalf("after wrap selProv = %d, want 1", got)
		}
	})

	t.Run("tab toggles the focused pane", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(pressTab())
		if a.modelDlg.pane != paneModels {
			t.Fatalf("after tab pane = %d, want models", a.modelDlg.pane)
		}
		a.handleKey(pressTab())
		if a.modelDlg.pane != paneProviders {
			t.Fatalf("after second tab pane = %d, want providers", a.modelDlg.pane)
		}
	})

	t.Run("model arrows move and wrap in the models pane", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyDown)) // opencode
		a.handleKey(pressTab())
		a.handleKey(press(tea.KeyDown))
		if got := a.modelDlg.selModel; got != 1 {
			t.Fatalf("after down selModel = %d, want 1 (gpt-5-nano)", got)
		}
		a.handleKey(press(tea.KeyDown)) // wraps to the first model
		if got := a.modelDlg.selModel; got != 0 {
			t.Fatalf("after wrap selModel = %d, want 0", got)
		}
	})

	t.Run("enter opens the subchoice only on the models pane", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyEnter)) // providers pane: no subchoice
		if a.modelDlg.subChoice {
			t.Fatal("enter on the providers pane must not open the subchoice")
		}
		a.handleKey(pressTab())
		a.handleKey(press(tea.KeyEnter))
		if !a.modelDlg.subChoice {
			t.Fatal("enter on the models pane must open the subchoice")
		}
	})

	t.Run("subchoice a/b emit one cmd; other keys are ignored", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(pressTab())
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press('x'))
		if len(a.Cmds) != 0 {
			t.Fatalf("key x in subchoice emitted %d cmds, want 0", len(a.Cmds))
		}
		a.handleKey(press('a'))
		if len(a.Cmds) != 1 {
			t.Fatalf("key a emitted %d cmds, want 1", len(a.Cmds))
		}
		a.Cmds = nil
		a.handleKey(press('b'))
		if len(a.Cmds) != 1 {
			t.Fatalf("key b emitted %d cmds, want 1", len(a.Cmds))
		}
		// The dialog stays open until the patch msg is applied.
		if !a.dlg.has() {
			t.Fatal("dialog must stay open before the patch msg lands")
		}
	})

	t.Run("esc closes the subchoice, then the dialog", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(pressTab())
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press(tea.KeyEscape))
		if a.modelDlg.subChoice || !a.dlg.has() {
			t.Fatalf("after esc: subChoice=%v dlg=%v, want subchoice closed and dialog open", a.modelDlg.subChoice, a.dlg.has())
		}
		a.handleKey(press(tea.KeyEscape))
		if a.dlg.has() || a.modelDlg != nil {
			t.Fatal("after second esc the dialog must be gone")
		}
	})

	t.Run("list keys never fall through to the prompt", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press('z'))
		if a.prompt.input.Value() != "" {
			t.Fatalf("prompt input = %q, must stay empty while the dialog is open", a.prompt.input.Value())
		}
		if len(a.Cmds) != 0 {
			t.Fatalf("key z emitted %d cmds, want 0", len(a.Cmds))
		}
	})
}

func TestModelDialogApply(t *testing.T) {
	t.Run("session patch: success toasts, closes, and updates current", func(t *testing.T) {
		a := openModelAt()
		a.handleKey(press(tea.KeyDown)) // opencode
		a.handleKey(pressTab())
		a.handleKey(press(tea.KeyDown)) // gpt-5-nano
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press('a'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1", len(a.Cmds))
		}
		a.applyDlgPatch(dlgPatchMsg{field: "model", value: "opencode/gpt-5-nano",
			sess: &protocol.Session{ID: "ses_1", Agent: "build", Model: refModel("opencode", "gpt-5-nano")}})
		if a.dlg.has() || a.modelDlg != nil {
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
		a.handleKey(pressTab())
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press('b'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1", len(a.Cmds))
		}
		a.applyDlgPatch(dlgPatchMsg{field: "model", value: "opencode/gpt-5-nano",
			cfg: map[string]any{"model": "opencode/gpt-5-nano"}})
		if a.dlg.has() {
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
		a.handleKey(pressTab())
		a.handleKey(press(tea.KeyEnter))
		a.applyDlgPatch(dlgPatchMsg{field: "model", value: "opencode/gpt-5-nano", err: errors.New("boom")})
		if !hasToast(a, "boom") {
			t.Fatalf("toasts = %+v, want boom", a.toasts)
		}
		if !a.dlg.has() {
			t.Fatal("dialog must stay open after a failed patch")
		}
	})

	t.Run("'a' with no session toasts no-session", func(t *testing.T) {
		a := modelFixture()
		a.route = routeHome
		a.cur = ""
		a.store.Current = nil
		a.openModelDialog()
		a.Cmds = nil
		a.handleKey(pressTab())
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
		if !ok || d.kind != dlgModel || a.modelDlg == nil {
			t.Fatalf("after ctrl+p: top=%+v modelDlg=%v, want the model dialog", d, a.modelDlg)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("ctrl+p emitted %d cmds, want the catalog fetch", len(a.Cmds))
		}
	})

	t.Run("/model opens the model dialog", func(t *testing.T) {
		a := modelFixture()
		a.runCommand("/model")
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgModel || a.modelDlg == nil {
			t.Fatalf("after /model: top=%+v modelDlg=%v, want the model dialog", d, a.modelDlg)
		}
	})

	t.Run("ctrl+p is ignored while a dialog is on top", func(t *testing.T) {
		a := modelFixture()
		a.dlg.push(dialog{kind: dlgQuit})
		a.handleKey(pressCtrlP())
		d, _ := a.dlg.top()
		if d.kind != dlgQuit || a.modelDlg != nil {
			t.Fatalf("ctrl+p must not stack dialogs: top=%+v modelDlg=%v", d, a.modelDlg)
		}
	})

	t.Run("catalog msg hydrates the store and re-syncs the selection", func(t *testing.T) {
		a := modelFixture()
		a.store.Providers = nil
		a.store.Agents = nil
		a.openModelDialog()
		a.applyCatalog(catalogMsg{provs: tuiProviderFixture(), agents: tuiAgentFixture()})
		if len(a.store.Providers) != 2 || len(a.store.Agents) != 3 {
			t.Fatalf("store = %d providers / %d agents, want 2 / 3", len(a.store.Providers), len(a.store.Agents))
		}
		if a.modelDlg.selProv != 0 {
			t.Fatalf("after catalog selProv = %d, want 0 (config model kido/q)", a.modelDlg.selProv)
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
// ctrl+p (the offline server fixture), navigate to opencode/gpt-5-nano, and
// set it for this session with [a].
func TestTUIModelDialog(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	ctx := context.Background()
	ses, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := NewApp(c, &store.Store{}, ses.ID)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))

	tm.Send(pressCtrlP())
	teatest.WaitFor(t, tm.Output(), hasModelDialog, teatest.WithDuration(5*time.Second))

	tm.Send(press(tea.KeyDown)) // opencode provider
	tm.Send(press(tea.KeyEnter))
	tm.Send(pressTab())
	tm.Send(press(tea.KeyDown)) // gpt-5-nano
	tm.Send(press(tea.KeyEnter))
	tm.Send(press('a')) // this session

	teatest.WaitFor(t, tm.Output(), hasLine("model set: opencode/gpt-5-nano"), teatest.WithDuration(5*time.Second))

	got, err := c.GetSession(ctx, ses.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Model == nil || got.Model.ProviderID != "opencode" || got.Model.ID != "gpt-5-nano" {
		t.Fatalf("session model = %+v, want opencode/gpt-5-nano", got.Model)
	}

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func hasModelDialog(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "Kido") &&
		strings.Contains(s, "· not-required") &&
		strings.Contains(s, "OpenCode Zen") &&
		strings.Contains(s, "○ missing") &&
		strings.Contains(s, "Qwen*") &&
		strings.Contains(s, "100k ctx")
}
