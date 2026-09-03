package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
)

func openStatusDlg() *recApp {
	a := testApp()
	a.store.Providers = []protocol.Provider{
		{ID: "kido", Name: "Kido", Auth: &protocol.ProviderAuth{Status: "loaded"}},
		{ID: "anthropic", Name: "Anthropic", Auth: &protocol.ProviderAuth{RequiresKey: true, Status: "missing"}},
	}
	a.store.Agents = agentFixture() // build, plan, yolo (model_test.go)
	a.openStatusDialog()
	return a
}

func TestStatusDialogRender(t *testing.T) {
	t.Run("providers + agents sections with the status details", func(t *testing.T) {
		a := openStatusDlg()
		got := stripANSI(a.statusView(80, 24, a.theme))
		if !strings.Contains(got, "Status") || !strings.Contains(got, "esc") {
			t.Fatalf("header missing:\n%s", got)
		}
		if !strings.Contains(got, "2 Providers") || !strings.Contains(got, "3 Agents") {
			t.Fatalf("count headers missing:\n%s", got)
		}
		for _, tok := range []string{"Kido", "● loaded", "Anthropic", "○ missing", "build", "plan", "yolo"} {
			if !strings.Contains(got, tok) {
				t.Fatalf("token %q missing:\n%s", tok, got)
			}
		}
	})

	t.Run("empty sections render the No-X fallbacks", func(t *testing.T) {
		a := testApp()
		a.openStatusDialog()
		got := stripANSI(a.statusView(80, 24, a.theme))
		if !strings.Contains(got, "No Providers") || !strings.Contains(got, "No Agents") {
			t.Fatalf("fallbacks missing:\n%s", got)
		}
	})

	t.Run("only esc/ctrl+c close; other keys are ignored", func(t *testing.T) {
		a := openStatusDlg()
		if cmds := a.handleKey(press('x')); len(cmds) != 0 || a.dlg.empty() {
			t.Fatalf("a plain key must be ignored: cmds=%d empty=%v", len(cmds), a.dlg.empty())
		}
		a.handleKey(press(tea.KeyEscape))
		if !a.dlg.empty() {
			t.Fatal("esc must close the status dialog")
		}
	})
}
