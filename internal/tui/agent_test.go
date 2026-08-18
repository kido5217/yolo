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

// agentFixture builds a session-route app with the offline catalog hydrated.
func agentFixture() *App {
	a := testApp()
	a.store.Current = &protocol.Session{ID: "ses_1", Agent: "build", Model: refModel("kido", "q")}
	a.store.Providers = tuiProviderFixture()
	a.store.Agents = tuiAgentFixture()
	a.store.Config = map[string]any{"agent": "build"}
	a.route = routeSession
	a.cur = "ses_1"
	return a
}

// openAgentAt opens the agent dialog and resets the recorded cmds.
func openAgentAt() *App {
	a := agentFixture()
	a.openAgentDialog()
	a.Cmds = nil
	return a
}

func agentBlock(t *testing.T, a *App, want string) {
	t.Helper()
	if got := stripANSI(a.agentDlg.view(&a.store)); got != want {
		t.Errorf("agent dialog mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestAgentDialogRender(t *testing.T) {
	t.Run("current agent is marked", func(t *testing.T) {
		a := openAgentAt()
		want := "Agents\n" +
			"  build*  The default agent. Executes tools based on configured permissions.\n" +
			"  plan  Plan mode. Disallows all edit tools.\n" +
			"  yolo  Yolo agent. Permits everything without prompts.\n" +
			"  ↑/↓ move · enter set · esc close"
		agentBlock(t, a, want)
	})

	t.Run("config agent marks the default when no session exists", func(t *testing.T) {
		a := openAgentAt()
		a.store.Current = nil
		a.store.Config["agent"] = "plan"
		want := "Agents\n" +
			"  build  The default agent. Executes tools based on configured permissions.\n" +
			"  plan*  Plan mode. Disallows all edit tools.\n" +
			"  yolo  Yolo agent. Permits everything without prompts.\n" +
			"  ↑/↓ move · enter set · esc close"
		agentBlock(t, a, want)
	})

	t.Run("no agents renders the loading hint", func(t *testing.T) {
		a := agentFixture()
		a.store.Agents = nil
		a.openAgentDialog()
		agentBlock(t, a, "Agents\n  loading…")
	})
}

func TestAgentDialogKeys(t *testing.T) {
	t.Run("down/up move with wraparound", func(t *testing.T) {
		a := openAgentAt()
		if got := a.agentDlg.sel; got != 0 {
			t.Fatalf("initial sel = %d, want 0 (session agent build)", got)
		}
		a.handleKey(press(tea.KeyDown))
		a.handleKey(press(tea.KeyDown))
		if got := a.agentDlg.sel; got != 2 {
			t.Fatalf("after two downs sel = %d, want 2 (yolo)", got)
		}
		a.handleKey(press(tea.KeyUp))
		a.handleKey(press(tea.KeyUp))
		a.handleKey(press(tea.KeyUp)) // wraps to the last agent
		if got := a.agentDlg.sel; got != 2 {
			t.Fatalf("after wrap sel = %d, want 2", got)
		}
	})

	t.Run("enter opens the subchoice", func(t *testing.T) {
		a := openAgentAt()
		if a.agentDlg.subChoice {
			t.Fatal("subchoice must start closed")
		}
		a.handleKey(press(tea.KeyEnter))
		if !a.agentDlg.subChoice {
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
		if !a.dlg.has() {
			t.Fatal("dialog must stay open before the patch msg lands")
		}
	})

	t.Run("esc closes the subchoice, then the dialog", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press(tea.KeyEnter))
		a.handleKey(press(tea.KeyEscape))
		if a.agentDlg.subChoice || !a.dlg.has() {
			t.Fatalf("after esc: subChoice=%v dlg=%v, want subchoice closed and dialog open", a.agentDlg.subChoice, a.dlg.has())
		}
		a.handleKey(press(tea.KeyEscape))
		if a.dlg.has() || a.agentDlg != nil {
			t.Fatal("after second esc the dialog must be gone")
		}
	})

	t.Run("list keys never fall through to the prompt", func(t *testing.T) {
		a := openAgentAt()
		a.handleKey(press('z'))
		if a.prompt.input.Value() != "" {
			t.Fatalf("prompt input = %q, must stay empty while the dialog is open", a.prompt.input.Value())
		}
		if len(a.Cmds) != 0 {
			t.Fatalf("key z emitted %d cmds, want 0", len(a.Cmds))
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
		if a.dlg.has() || a.agentDlg != nil {
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
		if a.dlg.has() {
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
		if !a.dlg.has() {
			t.Fatal("dialog must stay open after a failed patch")
		}
	})

	t.Run("'a' with no session toasts no-session", func(t *testing.T) {
		a := agentFixture()
		a.route = routeHome
		a.cur = ""
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
	t.Run("ctrl+a opens the agent dialog", func(t *testing.T) {
		a := agentFixture()
		a.handleKey(pressCtrlA())
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgAgents || a.agentDlg == nil {
			t.Fatalf("after ctrl+a: top=%+v agentDlg=%v, want the agent dialog", d, a.agentDlg)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("ctrl+a emitted %d cmds, want the catalog fetch", len(a.Cmds))
		}
	})

	t.Run("/agents opens the agent dialog", func(t *testing.T) {
		a := agentFixture()
		a.runCommand("/agents")
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgAgents || a.agentDlg == nil {
			t.Fatalf("after /agents: top=%+v agentDlg=%v, want the agent dialog", d, a.agentDlg)
		}
	})

	t.Run("ctrl+a is ignored while a dialog is on top", func(t *testing.T) {
		a := agentFixture()
		a.dlg.push(dialog{kind: dlgQuit})
		a.handleKey(pressCtrlA())
		d, _ := a.dlg.top()
		if d.kind != dlgQuit || a.agentDlg != nil {
			t.Fatalf("ctrl+a must not stack dialogs: top=%+v agentDlg=%v", d, a.agentDlg)
		}
	})
}

// TestTUIAgentDialog is the teatest scenario: open the agent dialog with
// ctrl+a, select the yolo agent, and set it for this session with [a].
func TestTUIAgentDialog(t *testing.T) {
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

	tm.Send(pressCtrlA())
	teatest.WaitFor(t, tm.Output(), hasAgentDialog, teatest.WithDuration(5*time.Second))

	tm.Send(press(tea.KeyDown)) // plan
	tm.Send(press(tea.KeyDown)) // yolo
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
		strings.Contains(s, "build*") &&
		strings.Contains(s, "The default agent.") &&
		strings.Contains(s, "yolo") &&
		strings.Contains(s, "Yolo agent. Permits everything")
}
