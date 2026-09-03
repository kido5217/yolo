package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestKeymapDefinitionsVerbatim(t *testing.T) {
	if len(Definitions) != 185 {
		t.Fatalf("Definitions = %d entries, want 185 (the 184 upstream + the yolo prompt_soft_newline)", len(Definitions))
	}
	// The upstream defaults (keybind.ts) — spot checks across the value
	// shapes (the verbatim port bar).
	cases := map[string]string{
		"leader":             "ctrl+x",
		"command_list":       "ctrl+p",
		"session_interrupt":  "escape",
		"session_rename":     "ctrl+r",
		"session_new":        "<leader>n",
		"messages_page_up":   "pageup,ctrl+alt+b",
		"messages_page_down": "pagedown,ctrl+alt+f",
		"app_exit":           "ctrl+c,ctrl+d,<leader>q",
		"which_key_toggle":   "ctrl+alt+k",
		"theme_switch_mode":  "none",
		"theme_mode_lock":    "none",
		"model_list":         "<leader>m",
		"agent_list":         "<leader>a",
		"input_newline":      "shift+return,ctrl+return,alt+return,ctrl+j",
	}
	for name, want := range cases {
		if got := Definitions[name].Default; got != want {
			t.Errorf("Definitions[%q].Default = %v, want %q", name, got, want)
		}
	}
	// The only object-shaped default (keybind.ts:162).
	paste, ok := Definitions["input_paste"].Default.(map[string]any)
	if !ok || paste["key"] != "ctrl+v" || paste["preventDefault"] != false {
		t.Errorf("input_paste default = %v, want {key: ctrl+v, preventDefault: false}", Definitions["input_paste"].Default)
	}
	// The yolo-specific display entry (deviation 208): the V1 soft-enter
	// pin has no upstream referent.
	if got := Definitions["prompt_soft_newline"].Default; got != "\\+enter" {
		t.Errorf("prompt_soft_newline = %v, want the V1 soft-enter sentinel", got)
	}
	// Every entry carries a description (the upstream Descriptions,
	// keybind.ts:253-255).
	for name, def := range Definitions {
		if def.Description == "" {
			t.Errorf("Definitions[%q].Description is empty", name)
		}
	}
	// The ported CommandMap is the upstream's 163-entry binding→command map
	// (keybind.ts:256-420) — verbatim. The 21 non-command bindings (leader,
	// the 13 dialog.* + 5 prompt.autocomplete.* navigation bindings,
	// permission.prompt.fullscreen, plugins.toggle) have no command and are
	// absent from the upstream CommandMap by design, so the assertion is
	// scoped to the CommandMap's own set (not the 184 Definitions names);
	// every ported command key must be a ported Definitions name.
	if len(CommandMap) != 163 {
		t.Fatalf("CommandMap = %d entries, want 163 (the upstream set)", len(CommandMap))
	}
	for name := range CommandMap {
		if _, ok := Definitions[name]; !ok {
			t.Errorf("CommandMap[%q] has no Definitions entry", name)
		}
	}
}

func TestKeymapResolveValue(t *testing.T) {
	tests := []struct {
		name string
		in   BindingValue
		want []string
		err  bool
	}{
		{"string", "ctrl+p", []string{"ctrl+p"}, false},
		{"comma list", "ctrl+c,ctrl+d,<leader>q", []string{"ctrl+c", "ctrl+d", "<leader>q"}, false},
		{"comma with none", "a,none,b", []string{"a", "b"}, false},
		{"none string", "none", nil, false},
		{"false", false, nil, false},
		{"nil", nil, nil, false},
		{"list", []any{"a", "b"}, []string{"a", "b"}, false},
		{"keystroke object", map[string]any{"name": "m", "ctrl": true}, []string{"ctrl+m"}, false},
		{"spec object", map[string]any{"key": "ctrl+v", "preventDefault": false}, []string{"ctrl+v"}, false},
		{"spec object keystroke key", map[string]any{"key": map[string]any{"name": "k", "shift": true}}, []string{"shift+k"}, false},
		{"number", 42, nil, true},
		{"empty map", map[string]any{}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveValue(tt.in)
			if (err != nil) != tt.err {
				t.Fatalf("err = %v, want err=%v", err, tt.err)
			}
			if tt.err {
				return
			}
			if len(got.seqs) != len(tt.want) {
				t.Fatalf("seqs = %v, want %v", got.seqs, tt.want)
			}
			for i := range tt.want {
				if got.seqs[i] != tt.want[i] {
					t.Fatalf("seqs = %v, want %v", got.seqs, tt.want)
				}
			}
		})
	}
}

func TestKeymapKeyMatchesSeq(t *testing.T) {
	tests := []struct {
		name string
		k    tea.KeyPressMsg
		seq  string
		want bool
	}{
		{"plain char", press('m'), "m", true},
		{"wrong char", press('m'), "n", false},
		{"ctrl+p", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}, "ctrl+p", true},
		{"no ctrl", press('p'), "ctrl+p", false},
		{"two mods", tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl | tea.ModAlt}, "ctrl+alt+b", true},
		{"seq mod order reversed", tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl | tea.ModAlt}, "alt+ctrl+b", true},
		{"extra mod", tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl | tea.ModAlt | tea.ModShift}, "ctrl+alt+b", false},
		{"enter=return", tea.KeyPressMsg{Code: tea.KeyEnter}, "return", true},
		{"return=enter", tea.KeyPressMsg{Code: tea.KeyEnter}, "enter", true},
		{"esc=escape", tea.KeyPressMsg{Code: tea.KeyEscape}, "escape", true},
		{"escape=esc", tea.KeyPressMsg{Code: tea.KeyEscape}, "esc", true},
		{"pgup=pageup", tea.KeyPressMsg{Code: tea.KeyPgUp}, "pageup", true},
		{"pageup=pgup", tea.KeyPressMsg{Code: tea.KeyPgUp}, "pgup", true},
		{"pgdown=pagedown", tea.KeyPressMsg{Code: tea.KeyPgDown}, "pagedown", true},
		{"shift+a", tea.KeyPressMsg{Code: 'a', Mod: tea.ModShift}, "shift+a", true},
		{"uppercase default seq", tea.KeyPressMsg{Code: 'E'}, "E", true},
		{"backspace", press(tea.KeyBackspace), "backspace", true},
		{"leader token does not match raw", tea.KeyPressMsg{Code: 'm'}, "<leader>m", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keyMatchesSeq(tt.k, tt.seq); got != tt.want {
				t.Fatalf("keyMatchesSeq(%v, %q) = %v, want %v", tt.k, tt.seq, got, tt.want)
			}
		})
	}
}

func TestKeymapFormatKeySequence(t *testing.T) {
	// The display aliases (the upstream keyNameAliases + the yolo
	// escape→esc — deviation 214).
	if got := formatKeySequence("pageup", "ctrl+x"); got != "pgup" {
		t.Errorf("formatKeySequence(pageup) = %q, want pgup", got)
	}
	if got := formatKeySequence("pagedown", "ctrl+x"); got != "pgdn" {
		t.Errorf("formatKeySequence(pagedown) = %q, want pgdn", got)
	}
	if got := formatKeySequence("escape", "ctrl+x"); got != "esc" {
		t.Errorf("formatKeySequence(escape) = %q, want esc", got)
	}
	if got := formatKeySequence("delete", "ctrl+x"); got != "del" {
		t.Errorf("formatKeySequence(delete) = %q, want del", got)
	}
	// The <leader> token expands to the resolved leader key.
	if got := formatKeySequence("<leader>t", "ctrl+x"); got != "ctrl+x t" {
		t.Errorf("formatKeySequence(<leader>t) = %q, want ctrl+x t", got)
	}
	// The modifier alias meta→alt.
	if got := formatKeySequence("meta+k", "ctrl+x"); got != "alt+k" {
		t.Errorf("formatKeySequence(meta+k) = %q, want alt+k", got)
	}
	// A plain seq passes through (the alias table applies to the base +
	// the modifier positions only).
	if got := formatKeySequence("ctrl+p", "ctrl+x"); got != "ctrl+p" {
		t.Errorf("formatKeySequence(ctrl+p) = %q, want ctrl+p", got)
	}
}

// pressLeader is the default leader keypress (ctrl+x, LeaderDefault).
func pressLeader() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl} }

func TestKeymapNew(t *testing.T) {
	// The unknown-key error (the ported parse).
	if _, err := NewKeymap(map[string]any{"nope": "ctrl+z"}); err == nil ||
		!strings.Contains(err.Error(), "unrecognized keybind(s): nope") {
		t.Fatalf("unknown key err = %v, want the unrecognized message", err)
	}
	// The present name is overridden; the absent name keeps its default.
	km, err := NewKeymap(map[string]any{"command_list": "ctrl+k"})
	if err != nil {
		t.Fatal(err)
	}
	if !km.Match("command_list", tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}) {
		t.Fatal("the override command_list=ctrl+k must match ctrl+k")
	}
	if km.Match("command_list", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}) {
		t.Fatal("the override must REPLACE the default (ctrl+p no longer matches)")
	}
	if !km.Match("leader", pressLeader()) {
		t.Fatal("the leader default must survive an unrelated override")
	}
}

func TestKeymapSet(t *testing.T) {
	km, _ := NewKeymap(nil)
	if err := km.Set("command_list", "ctrl+j"); err != nil {
		t.Fatal(err)
	}
	if !km.Match("command_list", tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}) {
		t.Fatal("Set must take effect immediately")
	}
	if km.Match("command_list", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}) {
		t.Fatal("the old binding must no longer match after Set")
	}
	if err := km.Set("nope", "ctrl+z"); err == nil {
		t.Fatal("Set on an unknown name must error")
	}
	if err := km.Set("command_list", "none"); err != nil {
		t.Fatal(err)
	}
	if km.Match("command_list", tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}) {
		t.Fatal("a none Set must disable the binding")
	}
}

func TestKeymapMatchPending(t *testing.T) {
	km, _ := NewKeymap(nil)
	if !km.MatchPending("model_list", press('m')) {
		t.Fatal("model_list <leader>m must match the continuation 'm'")
	}
	if km.MatchPending("model_list", press('a')) {
		t.Fatal("model_list must not match the continuation 'a'")
	}
	if km.MatchPending("command_list", press('p')) {
		t.Fatal("command_list (ctrl+p, no <leader>) must have no continuation")
	}
}

func TestKeymapModes(t *testing.T) {
	km, _ := NewKeymap(nil)
	if got := km.Current(); got != BaseMode {
		t.Fatalf("Current() = %q, want base (the empty stack)", got)
	}
	release := km.Push("session")
	if got := km.Current(); got != "session" {
		t.Fatalf("Current() after push = %q, want session", got)
	}
	release()
	if got := km.Current(); got != BaseMode {
		t.Fatalf("Current() after release = %q, want base", got)
	}
	// The identity splice: two pushes of the SAME mode, releasing the first
	// leaves the second (identity, not mode-name, matching).
	r1 := km.Push("session")
	r2 := km.Push("session")
	r1()
	if got := km.Current(); got != "session" {
		t.Fatalf("Current() after releasing the first of two = %q, want session (identity splice)", got)
	}
	r2()
}

func TestKeymapFormat(t *testing.T) {
	km, _ := NewKeymap(nil)
	if got := km.Format("app_exit"); got != "ctrl+c / ctrl+d / ctrl+x q" {
		t.Fatalf("Format(app_exit) = %q, want the comma-list display", got)
	}
	if got := km.Format("help_show"); got != "none" {
		t.Fatalf("Format(help_show) = %q, want none", got)
	}
	if got := km.Format("model_list"); got != "ctrl+x m" {
		t.Fatalf("Format(model_list) = %q, want ctrl+x m", got)
	}
}

func TestKeymapDispatch(t *testing.T) {
	t.Run("ctrl+c opens the quit dialog (app_exit)", func(t *testing.T) {
		a := testApp()
		a.handleKey(ctrlCKey)
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgQuit {
			t.Fatalf("after ctrl+c: top=%+v (ok=%v), want the quit dialog", d, ok)
		}
	})

	t.Run("ctrl+p opens the palette (the S4.4 remap lands)", func(t *testing.T) {
		a := testApp()
		a.handleKey(pressCtrlP())
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgPalette || a.pendingLeader {
			t.Fatalf("after ctrl+p: top=%+v (ok=%v) pending=%v, want the palette", d, ok, a.pendingLeader)
		}
	})

	t.Run("leader+m opens the model dialog", func(t *testing.T) {
		a := modelFixture()
		a.handleKey(pressLeader())
		a.Cmds = nil
		a.handleKey(press('m'))
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgModel || d.model == nil {
			t.Fatalf("after leader+m: top=%+v, want the model dialog", d)
		}
	})

	t.Run("leader+a opens the agent dialog", func(t *testing.T) {
		a := agentApp()
		a.handleKey(pressLeader())
		a.Cmds = nil
		a.handleKey(press('a'))
		d, ok := a.dlg.top()
		if !ok || d.kind != dlgAgents || d.agent == nil {
			t.Fatalf("after leader+a: top=%+v, want the agent dialog", d)
		}
	})

	t.Run("a non-matching second key clears the leader and is not lost", func(t *testing.T) {
		a := testApp()
		a.handleKey(pressLeader())
		a.Cmds = nil
		a.handleKey(press('z'))
		if a.pendingLeader {
			t.Fatal("the leader must clear on a non-matching second key")
		}
		if a.prompt.input.Value() != "z" {
			t.Fatalf("prompt = %q, want z (the key was not lost)", a.prompt.input.Value())
		}
	})

	t.Run("leader is ignored while a dialog is on top", func(t *testing.T) {
		a := modelFixture()
		a.dlg.push(dialog{kind: dlgQuit})
		a.handleKey(pressLeader())
		if a.pendingLeader {
			t.Fatal("the leader must not arm while a dialog is open")
		}
	})
}

func TestAppSetKeybinds(t *testing.T) {
	a := testApp()
	if got := a.keymap.Format("command_list"); got != "ctrl+p" {
		t.Fatalf("default command_list = %q, want ctrl+p", got)
	}
	if err := a.SetKeybinds(map[string]any{"command_list": "ctrl+k"}); err != nil {
		t.Fatal(err)
	}
	if got := a.keymap.Format("command_list"); got != "ctrl+k" {
		t.Fatalf("command_list after SetKeybinds = %q, want ctrl+k", got)
	}
	if !a.keymap.Match("command_list", tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}) {
		t.Fatal("the SetKeybinds override must match ctrl+k")
	}
	if err := a.SetKeybinds(map[string]any{"nope": "ctrl+z"}); err == nil {
		t.Fatal("SetKeybinds on an unknown key must error (a config error)")
	}
}

// TestLeaderReArmIgnoresStaleTimeout pins the timeout generation guard:
// re-arming the leader inside the pending window starts a fresh timeout,
// and the first (stale) tick — which bubbletea v2 cannot cancel — must not
// clear the re-armed pending state (a stale clear would drop the combo key
// at the old deadline).
func TestLeaderReArmIgnoresStaleTimeout(t *testing.T) {
	a := testApp()
	a.handleKey(pressLeader())
	stale := a.leaderGen
	a.handleKey(pressLeader())
	if !a.pendingLeader {
		t.Fatal("a leader keypress while pending must re-arm the pending state")
	}
	if stale == a.leaderGen {
		t.Fatal("re-arming the leader must advance the timeout generation")
	}
	a.Update(leaderTimeoutMsg{gen: stale})
	if !a.pendingLeader {
		t.Fatal("a stale timeout must not clear the re-armed pending leader")
	}
	a.Update(leaderTimeoutMsg{gen: a.leaderGen})
	if a.pendingLeader {
		t.Fatal("the current timeout must clear the pending leader")
	}
}
