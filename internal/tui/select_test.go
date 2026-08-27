package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func selTestOptions() []selectOption {
	return []selectOption{
		{title: "Alpha", description: "first", category: "Group A"},
		{title: "Beta", description: "second", category: "Group A"},
		{title: "Gamma", description: "third", category: "Group B"},
		{title: "Broken", disabled: true},
	}
}

// pushSelectModal pushes a select as the top modal (the production openers
// land in S2.9/S2.10; the stack contract is S2.2's).
func pushSelectModal(t *testing.T, a *recApp, m *selectModel) {
	t.Helper()
	a.pushModal(dialog{kind: dlgModel, sel: m}, dlgMedium, nil)
}

func TestSelectNavigationWrap(t *testing.T) {
	a := testApp()
	moved := []string{}
	m := selectNew("Test", "Search", selTestOptions(), nil,
		func(*App, selectOption) {},
		func(o selectOption) { moved = append(moved, o.title) })
	pushSelectModal(t, a, m)
	a.handleKey(downKey)
	a.handleKey(downKey)
	a.handleKey(downKey) // wraps to 0 (3 enabled options — "Broken" is disabled)
	a.handleKey(upKey)   // wraps to 2
	if m.sel != 2 {
		t.Fatalf("sel = %d, want 2 (wrap)", m.sel)
	}
	if strings.Join(moved, ",") != "Beta,Gamma,Alpha,Gamma" {
		t.Fatalf("onMove = %v", moved)
	}
}

func TestSelectEnterAndJump(t *testing.T) {
	a := testApp()
	var picked selectOption
	m := selectNew("Test", "Search", selTestOptions(), nil,
		func(_ *App, o selectOption) { picked = o }, nil)
	pushSelectModal(t, a, m)
	a.handleKey(homeKeyTest)
	a.handleKey(enterKey)
	if picked.title != "Alpha" {
		t.Fatalf("picked = %q, want Alpha", picked.title)
	}
	a.handleKey(endKey)
	a.handleKey(enterKey)
	if picked.title != "Gamma" {
		t.Fatalf("picked = %q, want Gamma (end)", picked.title)
	}
}

func TestSelectFuzzyFilter(t *testing.T) {
	a := testApp()
	var picked selectOption
	m := selectNew("Test", "Search", selTestOptions(), nil,
		func(_ *App, o selectOption) { picked = o }, nil)
	pushSelectModal(t, a, m)
	a.handleKey(press('g'))
	if len(m.filtered()) != 3 || m.filtered()[0].title != "Gamma" {
		t.Fatalf("filtered = %v, want [Gamma, Alpha, Beta]", m.filtered())
	}
	a.handleKey(enterKey)
	if picked.title != "Gamma" {
		t.Fatalf("picked = %q, want Gamma", picked.title)
	}
}

func TestSelectFuzzyWeighting(t *testing.T) {
	// a title hit (×2) must outrank a category hit (×1) on the same needle
	opts := []selectOption{
		{title: "Quiet", category: ""},
		{title: "Other", category: "quiet group"},
	}
	m := selectNew("T", "S", opts, nil, nil, nil)
	l := m.filtered()
	_ = l
	m.filter = "quiet"
	l = m.filtered()
	if len(l) != 2 || l[0].title != "Quiet" {
		t.Fatalf("weighted order = %v, want [Quiet Other]", titlesOf(l))
	}
}

func TestSelectViewLayout(t *testing.T) {
	a := testApp()
	m := selectNew("Test", "Search", selTestOptions(), nil, nil, nil)
	out := strings.Split(m.view(60, 24, a.theme), "\n")
	if !strings.Contains(out[0], "Test") {
		t.Fatalf("title row = %q", out[0])
	}
	if !strings.Contains(stripANSI(out[1]), "Search") {
		t.Fatalf("filter row = %q, want the placeholder", out[1])
	}
	if !strings.Contains(out[2], "Alpha") || !strings.Contains(out[3], "Beta") || !strings.Contains(out[4], "Gamma") {
		t.Fatalf("option rows missing: %q", out[2:5])
	}
	if !strings.Contains(out[len(out)-1], "esc close") {
		t.Fatalf("hint row = %q", out[len(out)-1])
	}
	// a no-match needle renders the empty state
	m.input.SetValue("zzz")
	out = strings.Split(m.view(60, 24, a.theme), "\n")
	if !strings.Contains(out[len(out)-1], "No results found") {
		t.Fatalf("empty state missing: %q", out)
	}
}

func titlesOf(l []selectOption) []string {
	out := make([]string, len(l))
	for i, o := range l {
		out[i] = o.title
	}
	return out
}

var (
	downKey     = tea.KeyPressMsg{Code: tea.KeyDown}
	upKey       = tea.KeyPressMsg{Code: tea.KeyUp}
	homeKeyTest = tea.KeyPressMsg{Code: tea.KeyHome}
	endKey      = tea.KeyPressMsg{Code: tea.KeyEnd}
)
