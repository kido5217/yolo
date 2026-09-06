package tui

// home_box_test.go — 0.8.0 Task 5 (prompt box: placeholder, meta line, hint
// line): the placeholder pools + re-roll, the homeMeta segments, the
// boxHighlight token, the boxInputLine width-exact window, the
// homeHintLine segments, and the applyPromptChrome route sizing. Whitebox,
// zero theme (plain text — the house zero-theme convention).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// metaCatalog is the homeMeta test seam catalog: the kido provider with the
// q model (Name "Qwen") — the mock contract (provider ID "kido", catalog
// model name "Qwen").
func metaCatalog() store.State {
	return store.State{Providers: []protocol.Provider{{
		ID:   "kido",
		Name: "Kido",
		Models: map[string]protocol.Model{
			"q": {ID: "q", Name: "Qwen"},
		},
	}}}
}

// TestHomeMeta pins the meta-line segments over seeded stores (decision 6:
// NO auto word; provider = the ref's provider ID, not the catalog name).
func TestHomeMeta(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		configure func(a *recApp)
		agent     string
		model     string
		provider  string
	}{
		{
			name: "config model ref resolves the catalog name",
			configure: func(a *recApp) {
				a.store = metaCatalog()
				a.store.Config = map[string]any{"model": "kido/q"}
			},
			agent: "Build", model: "Qwen", provider: "kido",
		},
		{
			name: "no config model falls back to the first provider's first model",
			configure: func(a *recApp) {
				a.store = metaCatalog()
			},
			agent: "Build", model: "Qwen", provider: "kido",
		},
		{
			name: "a ref model missing from the catalog falls back to the ref's modelID",
			configure: func(a *recApp) {
				a.store = metaCatalog()
				a.store.Config = map[string]any{"model": "kido/other"}
			},
			agent: "Build", model: "other", provider: "kido",
		},
		{
			name: "an unparseable config model is the raw ref segment",
			configure: func(a *recApp) {
				a.store = metaCatalog()
				a.store.Config = map[string]any{"model": "nonsense"}
			},
			agent: "Build", model: "nonsense", provider: "",
		},
		{
			name: "no providers omits the model and provider segments",
			configure: func(a *recApp) {
				a.store = store.State{}
			},
			agent: "Build", model: "", provider: "",
		},
		{
			name: "the config agent defaults the agent segment",
			configure: func(a *recApp) {
				a.store = metaCatalog()
				a.store.Config = map[string]any{"agent": "plan"}
			},
			agent: "Plan", model: "Qwen", provider: "kido",
		},
		{
			name: "the pending agent pins the agent segment",
			configure: func(a *recApp) {
				a.store = metaCatalog()
				a.store.Config = map[string]any{"agent": "plan"}
				a.pendingAgent = "yolo"
			},
			agent: "Yolo", model: "Qwen", provider: "kido",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := testApp()
			tc.configure(a)
			agent, model, provider := a.homeMeta()
			if agent != tc.agent || model != tc.model || provider != tc.provider {
				t.Fatalf("homeMeta = (%q, %q, %q), want (%q, %q, %q)", agent, model, provider, tc.agent, tc.model, tc.provider)
			}
		})
	}
}

// TestBoxHighlight pins the border/agent-segment token over the three
// states (leader pending wins over shell).
func TestBoxHighlight(t *testing.T) {
	t.Parallel()
	a := testApp()
	if got := a.boxHighlight(); got != "secondary" {
		t.Fatalf("boxHighlight (normal) = %q, want %q", got, "secondary")
	}
	a.prompt.mode = "shell"
	if got := a.boxHighlight(); got != "primary" {
		t.Fatalf("boxHighlight (shell) = %q, want %q", got, "primary")
	}
	a.pendingLeader = true
	if got := a.boxHighlight(); got != "border" {
		t.Fatalf("boxHighlight (leader pending) = %q, want %q (leader wins)", got, "border")
	}
}

// TestPlaceholderText pins the mode pools + the shared index (the pool wraps
// by length — upstream's single store.placeholder counter).
func TestPlaceholderText(t *testing.T) {
	t.Parallel()
	a := testApp()
	if got := a.prompt.placeholderText(); got != placeholderNormal[0] {
		t.Fatalf("placeholderText (normal, idx 0) = %q, want %q", got, placeholderNormal[0])
	}
	a.prompt.placeholderIdx = 2
	if got := a.prompt.placeholderText(); got != placeholderNormal[2] {
		t.Fatalf("placeholderText (normal, idx 2) = %q, want %q", got, placeholderNormal[2])
	}
	a.prompt.mode = "shell"
	if got := a.prompt.placeholderText(); got != placeholderShell[2] {
		t.Fatalf("placeholderText (shell, idx 2) = %q, want %q", got, placeholderShell[2])
	}
}

// TestRollPlaceholder pins the re-roll over the app's random seam (the
// repickTip idiom): the index is the seam value times the ACTIVE-mode pool
// length.
func TestRollPlaceholder(t *testing.T) {
	t.Parallel()
	t.Run("normal pool", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.tipRand = func() float64 { return 0.9 }
		a.rollPlaceholder()
		if got := a.prompt.placeholderIdx; got != 2 {
			t.Fatalf("placeholderIdx = %d, want 2 (int(0.9 * 3))", got)
		}
	})
	t.Run("the shell pool", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.mode = "shell"
		a.tipRand = func() float64 { return 0.5 }
		a.rollPlaceholder()
		if got := a.prompt.placeholderIdx; got != 1 {
			t.Fatalf("placeholderIdx = %d, want 1 (int(0.5 * 3))", got)
		}
		if got := a.prompt.placeholderText(); got != placeholderShell[1] {
			t.Fatalf("placeholderText = %q, want %q", got, placeholderShell[1])
		}
	})
}

// TestBoxInputLine pins the box input line's width-exact window (zero theme
// — plain runs): the placeholder (no cursor cell), the fitting value, the
// end-anchored overflow (the cursor end), and the sliding window (the
// pinned min(posCols, valueW-innerW) rule).
func TestBoxInputLine(t *testing.T) {
	t.Parallel()
	// the 200x50 box interior: boxW 75 - 1 - 2*2 = 70 display cols.
	const innerW = 70
	at200 := func(t *testing.T) *recApp {
		t.Helper()
		a := testApp()
		a.size = tea.WindowSizeMsg{Width: mockW, Height: mockH}
		return a
	}
	t.Run("an empty value renders the placeholder width-exact (no cursor cell)", func(t *testing.T) {
		t.Parallel()
		a := at200(t)
		line := a.boxInputLine()
		ph := placeholderNormal[0]
		want := ph + strings.Repeat(" ", innerW-len(ph))
		if line != want {
			t.Fatalf("boxInputLine = %q, want %q", line, want)
		}
		if w := runeWidth(line); w != innerW {
			t.Fatalf("line width = %d, want %d", w, innerW)
		}
	})
	t.Run("a fitting value renders with the cursor in view", func(t *testing.T) {
		t.Parallel()
		a := at200(t)
		a.prompt.input.SetValue("hello world")
		a.prompt.input.SetCursor(6) // on the "w"
		line := a.boxInputLine()
		want := "hello world" + strings.Repeat(" ", innerW-11)
		if line != want {
			t.Fatalf("boxInputLine = %q, want %q", line, want)
		}
	})
	t.Run("a wider value end-anchors at the cursor end", func(t *testing.T) {
		t.Parallel()
		a := at200(t)
		v := strings.Repeat("a", 80)
		a.prompt.input.SetValue(v)
		a.prompt.input.SetCursor(len(v)) // the value end
		line := a.boxInputLine()
		// the window is the last innerW-1 cols + the end cursor space.
		want := strings.Repeat("a", innerW-1) + " "
		if line != want {
			t.Fatalf("boxInputLine = %q, want %q", line, want)
		}
		if w := runeWidth(line); w != innerW {
			t.Fatalf("line width = %d, want %d", w, innerW)
		}
	})
	t.Run("a wider value slides the window to contain the cursor", func(t *testing.T) {
		t.Parallel()
		a := at200(t)
		v := strings.Repeat("0123456789", 8) // 80 display cols
		a.prompt.input.SetValue(v)
		a.prompt.input.SetCursor(5) // posCols 5 < 80-70 -> start 5
		line := a.boxInputLine()
		want := v[5 : 5+innerW]
		if line != want {
			t.Fatalf("boxInputLine = %q, want %q (window starts at the cursor col)", line, want)
		}
	})
}

// TestHomeHintLine pins the hint row: the normal-mode segments (shortcut fg
// text + muted words + the 2-col gap), the user-override tracking, the
// disabled-segment drop, and the shell-mode line.
func TestHomeHintLine(t *testing.T) {
	t.Parallel()
	t.Run("normal mode defaults", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		if got := a.homeHintLine(); got != "tab agents  ctrl+p commands" {
			t.Fatalf("homeHintLine = %q, want %q", got, "tab agents  ctrl+p commands")
		}
	})
	t.Run("the hint tracks a user override", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		if err := a.SetKeybinds(map[string]any{"agent_cycle": "ctrl+e"}); err != nil {
			t.Fatalf("SetKeybinds: %v", err)
		}
		if got := a.homeHintLine(); got != "ctrl+e agents  ctrl+p commands" {
			t.Fatalf("homeHintLine = %q, want %q", got, "ctrl+e agents  ctrl+p commands")
		}
	})
	t.Run("a disabled binding drops its segment", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		if err := a.SetKeybinds(map[string]any{"command_list": "none"}); err != nil {
			t.Fatalf("SetKeybinds: %v", err)
		}
		if got := a.homeHintLine(); got != "tab agents" {
			t.Fatalf("homeHintLine = %q, want %q", got, "tab agents")
		}
	})
	t.Run("both disabled renders a blank row", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		if err := a.SetKeybinds(map[string]any{"command_list": "none", "agent_cycle": "none"}); err != nil {
			t.Fatalf("SetKeybinds: %v", err)
		}
		if got := a.homeHintLine(); got != "" {
			t.Fatalf("homeHintLine = %q, want blank", got)
		}
	})
	t.Run("shell mode", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.prompt.mode = "shell"
		if got := a.homeHintLine(); got != "esc exit shell mode" {
			t.Fatalf("homeHintLine (shell) = %q, want %q", got, "esc exit shell mode")
		}
	})
}

// TestApplyPromptChrome pins the route sizing + the placeholder state: the
// home route sizes the box interior + the placeholder; the session route
// keeps the w-3 line + clears the placeholder; a mode switch re-sets the
// placeholder in place.
func TestApplyPromptChrome(t *testing.T) {
	t.Parallel()
	t.Run("home sizes the box interior and sets the placeholder", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.size = tea.WindowSizeMsg{Width: mockW, Height: mockH}
		a.applyPromptChrome()
		if got := a.prompt.input.Width(); got != 70 {
			t.Fatalf("input width = %d, want 70 (the 200x50 box interior)", got)
		}
		if got := a.prompt.input.Placeholder; got != placeholderNormal[0] {
			t.Fatalf("placeholder = %q, want %q", got, placeholderNormal[0])
		}
	})
	t.Run("the session route keeps the w-3 line and clears the placeholder", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.route = routeSession
		a.size = tea.WindowSizeMsg{Width: mockW, Height: mockH}
		a.prompt.input.Placeholder = placeholderNormal[0] // a stale home placeholder
		a.applyPromptChrome()
		if got := a.prompt.input.Width(); got != mockW-3 {
			t.Fatalf("input width = %d, want %d (w-3)", got, mockW-3)
		}
		if got := a.prompt.input.Placeholder; got != "" {
			t.Fatalf("placeholder = %q, want cleared (no leak onto the session line)", got)
		}
	})
	t.Run("a home resize re-sizes the box interior", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.size = tea.WindowSizeMsg{Width: 100, Height: 30}
		a.applyPromptChrome()
		// boxW min(75, 96) = 75 -> innerW 70 (the 80..~110 range).
		if got := a.prompt.input.Width(); got != 70 {
			t.Fatalf("input width = %d, want 70", got)
		}
	})
	t.Run("the shell mode switches the placeholder in place", func(t *testing.T) {
		t.Parallel()
		a := testApp()
		a.size = tea.WindowSizeMsg{Width: mockW, Height: mockH}
		a.applyPromptChrome()
		a.prompt.mode = "shell"
		a.applyPromptChrome()
		if got := a.prompt.input.Placeholder; got != placeholderShell[0] {
			t.Fatalf("placeholder = %q, want %q", got, placeholderShell[0])
		}
		if got := a.prompt.input.Width(); got != 70 {
			t.Fatalf("input width = %d, want 70 (unchanged)", got)
		}
	})
}
