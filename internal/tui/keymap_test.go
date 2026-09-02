package tui

import (
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
