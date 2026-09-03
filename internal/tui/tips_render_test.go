package tui

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// TestTipsVisibilityMatrix pins the ported visibility (upstream
// tips.tsx:40-47): first = no sessions, connected = a non-opencode
// provider or an opencode model with input cost, hidden gates all.
func TestTipsVisibilityMatrix(t *testing.T) {
	kido := []protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{"q": {ID: "q", Cost: protocol.ModelCost{Input: 0.5}}}}}
	opencodeZero := []protocol.Provider{{ID: "opencode", Models: map[string]protocol.Model{"z": {ID: "z", Cost: protocol.ModelCost{}}}}}
	opencodePaid := []protocol.Provider{{ID: "opencode", Models: map[string]protocol.Model{"z": {ID: "z", Cost: protocol.ModelCost{Input: 1}}}}}

	a := testApp() // 0 sessions, 0 providers
	if a.tipsFirst() != true || a.tipsConnected() != false {
		t.Fatalf("fresh = first + !connected (got %v/%v)", a.tipsFirst(), a.tipsConnected())
	}
	if !a.tipsVisible() {
		t.Fatal("fresh boot: !connected must show the tips (the NO_MODELS nudge)")
	}
	a.store.Sessions = []protocol.Session{{Title: "s1"}}
	if !a.tipsVisible() {
		t.Fatal("a session + no providers: the tips stay visible (NO_MODELS)")
	}
	a.store.Providers = kido
	if !a.tipsVisible() {
		t.Fatal("a session + a non-opencode provider: visible")
	}
	b := testApp() // 0 sessions + kido: first + connected → hidden
	b.store.Providers = kido
	if b.tipsVisible() {
		t.Fatal("first + connected must hide the tips")
	}
	c := testApp()
	c.store.Providers = opencodeZero
	if c.tipsConnected() {
		t.Fatal("opencode-only with zero input cost must be !connected")
	}
	d := testApp()
	d.store.Providers = opencodePaid
	if !d.tipsConnected() {
		t.Fatal("opencode with an input-cost model must be connected")
	}
	a.tipsHidden = true
	if a.tipsVisible() {
		t.Fatal("the hidden flag must gate all visibility")
	}
}

// TestTipTextSubstitutions pins the token substitution: <binding> →
// keymap.Format (registry-driven, remap-sensitive, "none" when
// disabled), {theme_count} → the theme count, and the NO_MODELS
// force when !connected.
func TestTipTextSubstitutions(t *testing.T) {
	a := testApp() // 0 providers → !connected → the NO_MODELS force
	if got := a.tipText(); got != noModelsTip {
		t.Fatalf("tipText = %q, want the NO_MODELS force", got)
	}

	b := testApp()
	b.store.Providers = []protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{"q": {ID: "q", Cost: protocol.ModelCost{Input: 0.5}}}}}
	idx := -1
	for i, s := range tips {
		if strings.Contains(s, "<session_new>") {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("no tip uses <session_new>")
	}
	b.tipIdx = idx
	got := b.tipText()
	if !strings.Contains(got, b.keymap.Format("session_new")) {
		t.Fatalf("tipText missing the session_new binding form: %q", got)
	}
	if strings.Contains(got, "<session_new>") {
		t.Fatalf("unsubstituted token: %q", got)
	}
	// remap → the display follows the registry
	if err := b.keymap.Set("session_new", "ctrl+n"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := b.tipText(); !strings.Contains(got, "ctrl+n") {
		t.Fatalf("remapped binding not reflected: %q", got)
	}
	// {theme_count}
	tcIdx := -1
	for i, s := range tips {
		if strings.Contains(s, "{theme_count}") {
			tcIdx = i
		}
	}
	b.tipIdx = tcIdx
	if got := b.tipText(); !strings.Contains(got, strconv.Itoa(themeCount)) {
		t.Fatalf("theme_count not substituted: %q", got)
	}
}

// TestTipLinesWrap pins the tagged-word wrap + the in-sequence run merge
// (the rowLines port): a tip that fits renders one line with the runs in
// SEQUENCE (prefix → muted → text → muted …); a wrapped line keeps the
// order per visual line.
func TestTipLinesWrap(t *testing.T) {
	parts := parseTip("Run {highlight}/connect{/highlight} to add an AI provider and start coding")
	lines := tipLines("● Tip ", parts, 80)
	if len(lines) != 1 {
		t.Fatalf("fitted tip = %d lines, want 1", len(lines))
	}
	joined := ""
	for _, r := range lines[0].runs {
		joined += r.text
	}
	if got := stripANSI(joined); got != "● Tip Run /connect to add an AI provider and start coding" {
		t.Fatalf("joined runs = %q", got)
	}
	kinds := []int{}
	for _, r := range lines[0].runs {
		kinds = append(kinds, r.kind)
	}
	// prefix, muted("Run "), text("/connect"), muted(" to add …")
	if len(kinds) != 4 || kinds[0] != 0 || kinds[1] != 1 || kinds[2] != 2 || kinds[3] != 1 {
		t.Fatalf("run kinds = %v, want [0 1 2 1]", kinds)
	}
	// wrap: narrow width → multiple lines, each in-sequence
	lines = tipLines("● Tip ", parts, 20)
	if len(lines) < 3 {
		t.Fatalf("wrapped tip = %d lines, want >= 3", len(lines))
	}
	for _, l := range lines {
		if len(l.runs) == 0 {
			t.Fatal("an empty visual line")
		}
	}
}

// TestTipsToggle pins the <leader>h toggle: the dispatch flips the flag
// (persisted over the theme KV when the engine is present) and the group
// wiring reaches dispatchCommand.
func TestTipsToggle(t *testing.T) {
	a := testApp()
	a.store.Sessions = []protocol.Session{{Title: "s1"}}
	if !a.tipsVisible() {
		t.Fatal("visible pre-toggle (a session, no providers → NO_MODELS)")
	}
	if cmds := a.dispatchCommand("tips_toggle"); cmds != nil {
		t.Fatalf("the toggle must not emit cmds (got %d)", len(cmds))
	}
	if !a.tipsHidden || a.tipsVisible() {
		t.Fatal("the toggle must hide the tips")
	}
	a.dispatchCommand("tips_toggle")
	if a.tipsHidden || !a.tipsVisible() {
		t.Fatal("the second toggle must restore visibility")
	}
	// the BaseMode group carries the binding (the <leader>h reachability)
	found := false
	for _, name := range contextGroups[BaseMode] {
		if name == "tips_toggle" {
			found = true
		}
	}
	if !found {
		t.Fatal("tips_toggle must be in the BaseMode context group")
	}
}

// TestTipsTogglePersists pins the KV round-trip (the S5.2 seam): the toggle
// writes tips_hidden over the engine KV; a second app over the SAME KV file
// (the themecmds_test.go fresh-engine idiom: e.Close() drains + flushes, then
// theme.New over the same dir) reloads it in NewApp's loadTipsHidden.
func TestTipsTogglePersists(t *testing.T) {
	a, e := themeApp(t)
	a.tipsHidden = false
	a.dispatchCommand("tips_toggle")
	if !a.tipsHidden {
		t.Fatal("toggle must set the flag")
	}
	_ = e.Close() // drains the writer + final flush (idempotent; themeApp's cleanup re-closes)
	dir := filepath.Dir(kvPathOf(e))
	e2, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New (second): %v", err)
	}
	t.Cleanup(func() { _ = e2.Close() })
	if err := e2.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve (second): %v", err)
	}
	b := &recApp{App: NewApp(client.New("http://127.0.0.1:9", ""), store.State{}, "", e2)}
	b.emitSink = func(cmds ...tea.Cmd) { b.Cmds = append(b.Cmds, cmds...) }
	b.store.Sessions = []protocol.Session{{Title: "s1"}}
	if !b.tipsHidden {
		t.Fatal("the hidden flag must persist across restart (KV)")
	}
}

// TestTipsHomeEntryRepick pins the per-home-entry re-pick (the upstream
// per-mount Math.random, no timer): each entry re-rolls with the seeded
// tipRand; the render picks tips[tipIdx % len].
func TestTipsHomeEntryRepick(t *testing.T) {
	a := testApp()
	a.store.Sessions = []protocol.Session{{Title: "s1"}}
	a.store.Providers = []protocol.Provider{{ID: "kido", Models: map[string]protocol.Model{"q": {ID: "q", Cost: protocol.ModelCost{Input: 0.5}}}}}
	var picks []float64
	i := 0
	a.tipRand = func() float64 {
		defer func() { i++ }()
		return float64(i) / float64(len(tips)) // 0, 1/n, 2/n, …
	}
	a.repickTip()
	first := a.tipIdx
	a.repickTip()
	if a.tipIdx == first {
		t.Fatal("a home entry must re-pick (a fresh random tip)")
	}
	if a.tipIdx < 0 || a.tipIdx >= len(tips) {
		t.Fatalf("tipIdx out of range: %d", a.tipIdx)
	}
	_ = picks
}

// TestHomeTipsLineRender pins the seam + the line shape (the ● Tip prefix
// in the warning tone, the parts wrapped at w) and the hidden/first
// gating through homeTipsLine.
func TestHomeTipsLineRender(t *testing.T) {
	a := testApp() // fresh: visible (NO_MODELS), tipIdx seeded
	line := a.homeTipsLine(80)
	if !strings.HasPrefix(stripANSI(line), "● Tip ") {
		t.Fatalf("tips line = %q, want the '● Tip ' prefix", line)
	}
	// hidden → the line is omitted
	a.tipsHidden = true
	if a.homeTipsLine(80) != "" {
		t.Fatal("hidden must omit the line")
	}
	// the renderClamped seam: a direct-construct homeModel (nil seams)
	// must not panic (the home_theme_test.go zero-theme idiom — the
	// point: renderClamped nil-guards the tips seam (and the future
	// footer seam); assert no panic + the plain layout).
	var zero homeModel
	if got := stripANSI(zero.renderClamped(&store.State{}, 80, theme.Theme{}, -1)); !strings.Contains(got, "New session") {
		t.Fatalf("nil-seam renderClamped = %q, want the plain layout (no panic)", got)
	}
}

// TestTipsTeatestPresence: home (fresh, the NO_MODELS nudge shows — first +
// !connected over the empty store) → n (create + open) → esc (home) → the
// tips line shows (a session + !connected ⇒ visible; the prefix is pinned,
// the text random). The real-boot smoke the pinned
// TestTipsConnectedRealBoot sketch carried is COLLAPSED into this leg
// (deviation 241): the yolo store.Providers referent is only populated when
// the model/agent dialog opens (fetchCatalogCmd) — never by the home
// hydrate — so a post-boot store read is both racy (the static "New
// session" row renders before the hydrate lands) and !connected (the
// NO_MODELS nudge shows, not the pinned connected ⇒ hidden); the state
// machine is unit-pinned by TestTipsVisibilityMatrix. (One merged WaitFor
// for the multi-token terminal state — the buffer-drain rule; fake.New()
// with zero turns is the idle driver, the S5.3/4 suites' idiom for
// flow-only legs.)
func TestTipsTeatestPresence(t *testing.T) {
	drv := fake.New()
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	tm.Send(press(tea.KeyEscape))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		full := stripANSI(string(b))
		return hasLine("New session")(b) && strings.Contains(full, "● Tip ")
	}, teatest.WithDuration(5*time.Second))
	tm.Send(ctrlCKey)
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
