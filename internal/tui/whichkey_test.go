package tui

import (
	"strings"
	"testing"
)

func TestWhichKeyEntriesBase(t *testing.T) {
	a := testApp() // home route, base mode
	a.pendingLeader = true
	v := stripANSI(a.whichKeyView(80))
	if v == "" {
		t.Fatal("the overlay must render while the leader is pending")
	}
	for _, want := range []string{"Leader", "Agent", "App", "Model", "Session", "Status", "Theme"} {
		if !strings.Contains(v, want) {
			t.Errorf("overlay missing group %q:\n%s", want, v)
		}
	}
	for _, want := range []string{
		"Exit the application", "List available models", "List agents",
		"View status", "List available themes", "Create a new session", "List all sessions",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("overlay missing label %q:\n%s", want, v)
		}
	}
}

func TestWhichKeyHiddenWhenNotPending(t *testing.T) {
	a := testApp()
	if v := a.whichKeyView(80); v != "" {
		t.Fatalf("the overlay must be empty when the leader is not pending, got:\n%s", stripANSI(v))
	}
}

func TestWhichKeyHiddenInModal(t *testing.T) {
	a := testApp()
	a.pendingLeader = true
	a.openModelDialog()
	if d, ok := a.dlg.top(); !ok || !d.modal {
		t.Fatal("the model dialog must be a modal (the overlay preconditions)")
	}
	if v := a.whichKeyView(80); v != "" {
		t.Fatalf("the overlay must be empty while a modal is open, got:\n%s", stripANSI(v))
	}
}

func TestWhichKeyRegistryDriven(t *testing.T) {
	a := testApp()
	a.pendingLeader = true
	if err := a.keymap.Set("model_list", "none"); err != nil {
		t.Fatal(err)
	}
	v := stripANSI(a.whichKeyView(80))
	if strings.Contains(v, "List available models") {
		t.Fatalf("a disabled binding's entry must not render:\n%s", v)
	}
}
