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

// agent_test.go — the S2.10 restyle: the plain agent list is the select +
// the yolo-pinned a/b subchoice (the list → select swap rides deviation 168's
// family entry, 183).

// agentApp builds a session-route app with the offline catalog hydrated.
func agentApp() *recApp {
	a := testApp()
	a.store.Current = &protocol.Session{ID: "ses_1", Agent: "build", Model: refModel("kido", "q")}
	a.store.Providers = providerFixture()
	a.store.Agents = agentFixture()
	a.store.Config = map[string]any{"agent": "build"}
	a.route = routeSession
	a.curSessionID = "ses_1"
	return a
}

// openAgentAt opens the agent dialog and resets the recorded cmds.
func openAgentAt() *recApp {
	a := agentApp()
	a.openAgentDialog()
	a.Cmds = nil
	return a
}

func TestAgentDialogRender(t *testing.T) {
	t.Run("agents flatten into the select, the current one is marked", func(t *testing.T) {
		a := openAgentAt()
		got := stripANSI(a.dlg.agent().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "Agents") {
			t.Fatalf("title missing:\n%s", got)
		}
		if !strings.Contains(got, "●") {
			t.Fatalf("current agent gutter missing:\n%s", got)
		}
		// agentFixture (model_test.go): build, plan, yolo — the session
		// agent "build" carries the ● gutter
		for _, tok := range []string{"build", "plan", "yolo"} {
			if !strings.Contains(got, tok) {
				t.Fatalf("agent %q missing:\n%s", tok, got)
			}
		}
	})

	t.Run("no agents renders the loading hint", func(t *testing.T) {
		a := agentApp()
		a.store.Agents = nil
		a.openAgentDialog()
		a.Cmds = nil
		got := stripANSI(a.dlg.agent().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "loading…") {
			t.Fatalf("loading hint missing:\n%s", got)
		}
	})

	t.Run("filter narrows the list", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press('b')) // only "build" matches
		got := stripANSI(a.dlg.agent().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "build") || strings.Contains(got, "yolo ") {
			t.Fatalf("filter did not narrow:\n%s", got)
		}
	})

	t.Run("subchoice line is the locked [a]/[b] overlay", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press(tea.KeyEnter))
		got := stripANSI(a.dlg.agent().view(&a.store, 80, 24, a.theme))
		if !strings.Contains(got, "[a] this session  [b] set default") {
			t.Fatalf("subchoice missing:\n%s", got)
		}
	})
}

func TestAgentDialogKeys(t *testing.T) {
	t.Run("down/up move with wraparound", func(t *testing.T) {
		a := openAgentAt()
		if got := a.dlg.agent().sel.sel; got != 0 {
			t.Fatalf("initial sel = %d, want 0 (session agent build)", got)
		}
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyDown))
		if got := a.dlg.agent().sel.sel; got != 2 {
			t.Fatalf("after two downs sel = %d, want 2 (yolo)", got)
		}
		a.handleKey(press(tea.KeyUp))
		a.handleKey(press(tea.KeyUp))
		a.handleKey(press(tea.KeyUp)) // wraps to the last agent
		if got := a.dlg.agent().sel.sel; got != 2 {
			t.Fatalf("after wrap sel = %d, want 2", got)
		}
	})

	t.Run("enter opens the subchoice", func(t *testing.T) {
		a := openAgentAt()
		if a.dlg.agent().hasSubChoice {
			t.Fatal("subchoice must start closed")
		}
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.agent().hasSubChoice {
			t.Fatal("enter must open the subchoice")
		}
	})

	t.Run("subchoice a/b emit one cmd; other keys are ignored", func(t *testing.T) {
		a := openAgentAt()
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
		if a.dlg.empty() {
			t.Fatal("dialog must stay open before the patch msg lands")
		}
	})

	t.Run("esc closes the subchoice, then the dialog", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press(tea.KeyEscape))
		if a.dlg.agent().hasSubChoice || a.dlg.empty() {
			t.Fatalf(
				"after esc: subChoice=%v dlg=%v, want subchoice closed and dialog open",
				a.dlg.agent().hasSubChoice, a.dlg.empty())
		}
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() || a.dlg.agent() != nil {
			t.Fatal("after second esc the dialog must be gone")
		}
	})

	t.Run("typed letters feed the filter, not the prompt", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press('z'))
		if got := a.dlg.agent().sel.input.Value(); got != "z" {
			t.Fatalf("filter input = %q, want z (typed letters feed the select filter)", got)
		}
		if a.prompt.input.Value() != "" {
			t.Fatalf("prompt input = %q, must stay empty while the dialog is open", a.prompt.input.Value())
		}
	})
}

func TestAgentDialogApply(t *testing.T) {
	t.Run("session patch: success toasts, closes, and updates current", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyDown)) // yolo
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press('a'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1", len(a.Cmds))
		}
		a.applyDlgPatch(dlgPatchMsg{field: "agent", value: "yolo",
			sess: &protocol.Session{ID: "ses_1", Agent: "yolo", Model: refModel("kido", "q")}})
		if !a.dlg.empty() || a.dlg.agent() != nil {
			t.Fatal("dialog must close after a successful session patch")
		}
		if !hasToast(a, "agent set: yolo") {
			t.Fatalf("toasts = %+v, want the agent-set toast", a.toasts)
		}
		if got := a.store.Current.Agent; got != "yolo" {
			t.Fatalf("current agent = %q, want yolo", got)
		}
	})

	t.Run("default patch: success updates the config agent", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press(tea.KeyEnter)) // build
		a.handleKey(press('b'))
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1", len(a.Cmds))
		}
		a.applyDlgPatch(dlgPatchMsg{field: "agent", value: "build",
			cfg: map[string]any{"agent": "build"}})
		if !a.dlg.empty() {
			t.Fatal("dialog must close after a successful default patch")
		}
		if !hasToast(a, "agent set: build") {
			t.Fatalf("toasts = %+v, want the agent-set toast", a.toasts)
		}
		if got := a.store.Config["agent"]; got != "build" {
			t.Fatalf("config agent = %v, want build", got)
		}
	})

	t.Run("error toasts and keeps the dialog", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press(tea.KeyEnter))
		a.applyDlgPatch(dlgPatchMsg{field: "agent", value: "yolo", err: errors.New("boom")})
		if !hasToast(a, "boom") {
			t.Fatalf("toasts = %+v, want boom", a.toasts)
		}
		if a.dlg.empty() {
			t.Fatal("dialog must stay open after a failed patch")
		}
	})

	t.Run("'a' with no session toasts no-session", func(t *testing.T) {
		a := agentApp()
		a.route = routeHome
		a.curSessionID = ""
		a.store.Current = nil
		a.openAgentDialog()
		a.Cmds = nil
		a.handleKey(press(tea.KeyDown)) // plan
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

func TestAgentDialogOpen(t *testing.T) {
	t.Run("leader+a opens the agent dialog", func(t *testing.T) {
		a := agentApp()
		a.handleKey(pressLeader())
		a.Cmds = nil
		a.handleKey(press('a'))
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgAgents || d.agent == nil {
			t.Fatalf("after leader+a: top=%+v agentDlg=%v, want the agent dialog", d, d.agent)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("leader+a emitted %d cmds, want the catalog fetch", len(a.Cmds))
		}
	})

	t.Run("/agents opens the agent dialog", func(t *testing.T) {
		a := agentApp()
		a.runCommand("/agents")
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgAgents || d.agent == nil {
			t.Fatalf("after /agents: top=%+v agentDlg=%v, want the agent dialog", d, d.agent)
		}
	})

	t.Run("leader is ignored while a dialog is on top", func(t *testing.T) {
		a := agentApp()
		a.dlg.push(dialog{kind: dlgQuit})
		a.handleKey(pressLeader())
		if a.pendingLeader {
			t.Fatal("the leader must not arm while a dialog is open")
		}
		d, _ := a.dlg.top()
		if d.kind != dlgQuit || a.dlg.agent() != nil {
			t.Fatalf("leader must not stack dialogs: top=%+v agentDlg=%v", d, a.dlg.agent())
		}
	})

	t.Run("catalog msg hydrates the store and re-syncs the selection", func(t *testing.T) {
		a := agentApp()
		a.store.Agents = nil
		a.openAgentDialog()
		a.applyCatalog(catalogMsg{provs: providerFixture(), agents: agentFixture()})
		sel := a.dlg.agent().sel
		if sel == nil || sel.sel != 0 {
			t.Fatalf("after catalog selection = %v, want 0 (session agent build)", sel)
		}
	})
}

// TestTUIAgentDialog is the teatest scenario: open the agent dialog with the
// /agents slash command (S4.2 remap: the ctrl+a opener frees to the prompt
// input, deviation 211), filter to the yolo agent (typed letter), enter → the
// subchoice, and set it for this session with [a].
func TestTUIAgentDialog(t *testing.T) {
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

	for _, r := range "/agents" {
		tm.Send(press(r))
	}
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), hasAgentDialog, teatest.WithDuration(5*time.Second))

	tm.Send(press('y')) // filter: only yolo matches
	tm.Send(press(tea.KeyEnter))
	tm.Send(press('a')) // this session

	teatest.WaitFor(t, tm.Output(), hasLine("agent set: yolo"), teatest.WithDuration(5*time.Second))

	got, err := c.GetSession(ctx, ses.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Agent != "yolo" {
		t.Fatalf("session agent = %q, want yolo", got.Agent)
	}

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func hasAgentDialog(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "Agents") &&
		strings.Contains(s, "● build") &&
		strings.Contains(s, "The default agent.") &&
		strings.Contains(s, "yolo") &&
		strings.Contains(s, "Yolo agent. Permits everything")
}

// TestCyclePendingAgent pins the home pending-agent cycle (0.8.0 Task 7,
// decision 1): the walk over store.Agents (wire order), the wrap both
// directions, the config-only start positions, the empty-list no-op, and
// the pin-sticks contract.
func TestCyclePendingAgent(t *testing.T) {
	t.Parallel()
	agents := agentFixture() // build, plan, yolo (the GET /agent wire order)

	t.Run("wraps both directions", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		if got := a.pendingAgentName(); got != "build" {
			t.Fatalf("initial = %q, want build (the unset default)", got)
		}
		a.cyclePendingAgent(1)
		if got := a.pendingAgent; got != "plan" {
			t.Fatalf("after tab = %q, want plan", got)
		}
		a.cyclePendingAgent(1)
		if got := a.pendingAgent; got != "yolo" {
			t.Fatalf("after tab = %q, want yolo", got)
		}
		a.cyclePendingAgent(1)
		if got := a.pendingAgent; got != "build" {
			t.Fatalf("after tab = %q, want build (the forward wrap)", got)
		}
		a.cyclePendingAgent(-1)
		if got := a.pendingAgent; got != "yolo" {
			t.Fatalf("after shift+tab = %q, want yolo (the reverse wrap)", got)
		}
	})

	t.Run("a config-only current agent starts at the direction end", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		a.store.Config = map[string]any{"agent": "custom"} // not in the list
		a.cyclePendingAgent(1)
		if got := a.pendingAgent; got != "build" {
			t.Fatalf("forward start = %q, want build (index 0)", got)
		}
		a.pendingAgent = ""
		a.cyclePendingAgent(-1)
		if got := a.pendingAgent; got != "yolo" {
			t.Fatalf("reverse start = %q, want yolo (index len-1)", got)
		}
	})

	t.Run("an empty agent list is a no-op", func(t *testing.T) {
		a := testApp()
		a.cyclePendingAgent(1)
		if a.pendingAgent != "" {
			t.Fatalf("pendingAgent = %q, want the empty no-op", a.pendingAgent)
		}
	})

	t.Run("the pin sticks over a later config change", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		a.cyclePendingAgent(1)
		if got := a.pendingAgent; got != "plan" {
			t.Fatalf("after the cycle = %q, want plan (the pin)", got)
		}
		a.store.Config = map[string]any{"agent": "custom"} // a later change
		a.cyclePendingAgent(1)
		if got := a.pendingAgent; got != "yolo" {
			t.Fatalf("after the config change + cycle = %q, want yolo (the pin walked on, it did not re-flow from the config)", got)
		}
	})
}

// TestAgentCycleKeyDispatch pins the BaseMode wiring (0.8.0 Task 7,
// decision 1): tab/shift+tab cycle the pending agent on home AND the
// session route (the tab char is no longer inserted), a SetKeybinds
// override remaps the cycle, and the ladder precedence (a dialog open or
// a pending permission) suppresses the cycle.
func TestAgentCycleKeyDispatch(t *testing.T) {
	t.Parallel()
	agents := agentFixture() // build, plan, yolo

	t.Run("tab cycles the pending agent on home", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		a.handleKey(pressTab())
		if got := a.pendingAgent; got != "plan" {
			t.Fatalf("after tab = %q, want plan", got)
		}
		if got := a.prompt.input.Value(); got != "" {
			t.Fatalf("input = %q, want empty (the tab is consumed, not inserted)", got)
		}
	})

	t.Run("tab cycles the pending agent on the session route", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		a.route = routeSession
		a.curSessionID = "ses_1"
		a.handleKey(pressTab())
		if got := a.pendingAgent; got != "plan" {
			t.Fatalf("after tab = %q, want plan (BaseMode owns any route)", got)
		}
		if got := a.prompt.input.Value(); got != "" {
			t.Fatalf("input = %q, want empty (the tab no longer inserts a tab char)", got)
		}
	})

	t.Run("shift+tab reverses", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		a.handleKey(pressTab())
		a.handleKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		if got := a.pendingAgent; got != "build" {
			t.Fatalf("after tab + shift+tab = %q, want build", got)
		}
	})

	t.Run("a SetKeybinds override remaps agent_cycle", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		if err := a.SetKeybinds(map[string]any{"agent_cycle": "f5"}); err != nil {
			t.Fatal(err)
		}
		a.prompt.input.SetValue("x")
		a.handleKey(pressTab())
		if a.pendingAgent != "" {
			t.Fatalf("after tab = %q, want the cycle off (the override replaces the default)", a.pendingAgent)
		}
		if got := a.prompt.input.Value(); got != "x" {
			t.Fatalf("input = %q, want x unchanged (tab fell through to the prompt; the named key carries no text to insert — deviation 278)", got)
		}
		a.handleKey(tea.KeyPressMsg{Code: tea.KeyF5})
		if got := a.pendingAgent; got != "plan" {
			t.Fatalf("after f5 = %q, want plan (the new key cycles)", got)
		}
	})

	t.Run("a dialog open does not fire the cycle (the ladder precedence)", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		a.dlg.push(dialog{kind: dlgQuit})
		a.handleKey(pressTab())
		if a.pendingAgent != "" {
			t.Fatalf("after tab with the dialog open = %q, want the cycle suppressed", a.pendingAgent)
		}
	})

	t.Run("a pending permission does not fire the cycle (the ladder precedence)", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agents
		a.store.Pending = []protocol.PermissionAskedProps{permProps()}
		a.handleKey(pressTab())
		if a.pendingAgent != "" {
			t.Fatalf("after tab with the pending permission = %q, want the cycle suppressed", a.pendingAgent)
		}
	})
}
