package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
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
	// no-category options: the S2.6 category headers must not shift the
	// pinned row lines (upstream flat list — deviation 175)
	opts := []selectOption{
		{title: "Alpha", description: "first"},
		{title: "Beta", description: "second"},
		{title: "Gamma", description: "third"},
	}
	m := selectNew("Test", "Search", opts, nil, nil, nil)
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

func TestSelectCategoriesRender(t *testing.T) {
	a := testApp()
	m := selectNew("Test", "Search", []selectOption{
		{title: "Alpha", category: "Group A"},
		{title: "Beta", category: "Group A"},
		{title: "Gamma", category: "Group B"},
	}, nil, nil, nil)
	lines := strings.Split(m.view(60, 24, a.theme), "\n")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Group A") || !strings.Contains(joined, "Group B") {
		t.Fatalf("category headers missing:\n%s", joined)
	}
	// the blank row separates the groups (Group A's last row, blank, header)
	iA := -1
	iB := -1
	for i, l := range lines {
		if l == "   Group A" {
			iA = i
		}
		if l == "   Group B" {
			iB = i
		}
	}
	if iA == -1 || iB == -1 || lines[iB-1] != "" {
		t.Fatalf("header layout wrong (iA=%d iB=%d):\n%s", iA, iB, joined)
	}
	// filtering hides the headers (upstream flat)
	m.input.SetValue("a")
	joined = strings.Join(strings.Split(m.view(60, 24, a.theme), "\n"), "\n")
	if strings.Contains(joined, "Group A") {
		t.Fatalf("headers must be hidden while filtering:\n%s", joined)
	}
}

func TestSelectDetailsAndFooter(t *testing.T) {
	a := testApp()
	m := selectNew("Test", "Search", []selectOption{
		{title: "Alpha", details: []string{"detail one", strings.Repeat("long detail ", 20)}, footer: "f1"},
	}, nil, nil, nil)
	lines := strings.Split(m.view(60, 24, a.theme), "\n")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "detail one") {
		t.Fatalf("detail row missing:\n%s", joined)
	}
	// the long detail is truncateMiddle'd to fit the row width
	for _, l := range lines {
		plain := stripANSI(l)
		if strings.Contains(plain, "long detail") && runeWidth(plain) > 60 {
			t.Fatalf("detail not clipped to the row width: %q", plain)
		}
	}
	// the footer tail sits at the right edge of its row
	for _, l := range lines {
		plain := strings.TrimRight(stripANSI(l), " ")
		if strings.Contains(plain, "Alpha") && strings.HasSuffix(plain, "f1") {
			return
		}
	}
	t.Fatalf("footer tail missing:\n%s", joined)
}

func TestSelectScrollWindowCountsRows(t *testing.T) {
	a := testApp()
	opts := make([]selectOption, 0, 20)
	for i := 0; i < 20; i++ {
		opts = append(opts, selectOption{
			title:   "Option " + string(rune('A'+i/26)) + string(rune('a'+i%26)),
			details: []string{"d"},
		})
	}
	m := selectNew("Test", "Search", opts, nil, nil, nil)
	// h=40 → visible = 40/2-6 = 14 rows; each option = 2 rows (row + detail)
	m.view(60, 40, a.theme)
	if m.top != 0 {
		t.Fatalf("initial top = %d, want 0", m.top)
	}
	for i := 0; i < 19; i++ { // walk the selection past the window
		m.move(1)
		m.view(60, 40, a.theme)
	}
	// 20 options × 2 rows = 40 rows; sel 19 → row 38-39 → top = 38-14+1 = 25
	if m.top < 20 {
		t.Fatalf("scroll did not follow the selection: top=%d", m.top)
	}
}

var (
	selTabMsg      = tea.KeyPressMsg{Code: tea.KeyTab}
	selShiftTabMsg = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	selPgDnMsg     = tea.KeyPressMsg{Code: tea.KeyPgDown}
	selPgUpMsg     = tea.KeyPressMsg{Code: tea.KeyPgUp}
)

func TestSelectActions(t *testing.T) {
	a := testApp()
	favs, runs := 0, 0
	m := selectNew("Test", "Search", selTestOptions(), nil, nil, nil).
		WithActions([]selectAction{
			{key: key.NewBinding(key.WithKeys("f")), title: "Favorite", run: func(*App) { favs++ }},
			{key: key.NewBinding(key.WithKeys("r")), title: "Remove", run: func(*App) { runs++ }},
		})
	pushSelectModal(t, a, m)
	a.handleKey(press('f'))
	if favs != 1 {
		t.Fatalf("action key: favs=%d, want 1", favs)
	}
	a.handleKey(selTabMsg)
	if m.focAct != 0 {
		t.Fatalf("tab focus = %d, want 0", m.focAct)
	}
	a.handleKey(enterKey)
	if favs != 2 || runs != 0 {
		t.Fatalf("enter on the focused action must run it: favs=%d runs=%d", favs, runs)
	}
	a.handleKey(selShiftTabMsg) // wraps to the last action
	if m.focAct != 1 {
		t.Fatalf("shift+tab wrap = %d, want 1", m.focAct)
	}
}

func TestSelectFooterHints(t *testing.T) {
	a := testApp()
	m := selectNew("Test", "Search", selTestOptions(), nil, nil, nil).
		WithHints([]footerHint{{key: "ctrl+x", desc: "remove"}})
	lines := strings.Split(m.view(60, 24, a.theme), "\n")
	last := stripANSI(lines[len(lines)-1])
	if !strings.Contains(last, "ctrl+x") || !strings.Contains(last, "remove") {
		t.Fatalf("hint row = %q", last)
	}
}

func TestSelectScrollAcceleration(t *testing.T) {
	a := testApp()
	opts := make([]selectOption, 40)
	for i := range opts {
		opts[i] = selectOption{title: fmtOption(i)}
	}
	m := selectNew("Test", "Search", opts, nil, nil, nil)
	pushSelectModal(t, a, m)
	// h=40 → visible 14 rows; 40 options = 40 rows → the window can scroll
	a.handleKey(selPgDnMsg)
	m.view(60, 40, a.theme)
	if m.top != 10 {
		t.Fatalf("pgdn: top=%d, want 10 (±10 rows)", m.top)
	}
	a.handleKey(selPgDnMsg)
	m.view(60, 40, a.theme)
	if m.top != 20 {
		t.Fatalf("pgdn twice: top=%d, want 20", m.top)
	}
	a.handleKey(selPgUpMsg)
	m.view(60, 40, a.theme)
	if m.top != 10 {
		t.Fatalf("pgup: top=%d, want 10", m.top)
	}
}

func fmtOption(i int) string {
	return "Option " + string(rune('a'+i/26)) + string(rune('a'+i%26))
}
