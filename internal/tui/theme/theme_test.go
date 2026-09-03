package theme

import (
	"reflect"
	"sort"
	"testing"
)

// The 33 upstream asset stems (kebab-case preserved). Set equality is locked:
// a rename or drop of any asset is a parity break (strict-copy bar, spec §1).
var wantThemeNames = []string{
	"aura", "ayu", "carbonfox", "catppuccin", "catppuccin-frappe",
	"catppuccin-macchiato", "cobalt2", "cursor", "dracula", "everforest",
	"flexoki", "github", "gruvbox", "kanagawa", "lucent-orng", "material",
	"matrix", "mercury", "monokai", "nightowl", "nord", "one-dark",
	"orng", "osaka-jade", "palenight", "rosepine", "solarized",
	"synthwave84", "tokyonight", "vercel", "vesper", "yolo", "zenburn",
}

func TestAllThemesEmbeds33UpstreamThemes(t *testing.T) {
	themes, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	got := make([]string, 0, len(themes))
	for name := range themes {
		got = append(got, name)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, wantThemeNames) {
		t.Fatalf("embedded theme names = %v, want %v", got, wantThemeNames)
	}
}

func TestAllThemesCached(t *testing.T) {
	first, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	for i := 0; i < 3; i++ {
		got, err := AllThemes()
		if err != nil {
			t.Fatalf("AllThemes (call %d): %v", i+2, err)
		}
		if reflect.ValueOf(got).Pointer() != reflect.ValueOf(first).Pointer() {
			t.Fatalf("AllThemes (call %d) returned a different map; the embedded assets are immutable and the parsed map must be cached", i+2)
		}
	}
}

func TestParseOpencodeThemeShape(t *testing.T) {
	themes, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	oc := themes["yolo"]
	if oc.Schema != "https://opencode.ai/theme.json" {
		t.Errorf("$schema = %q", oc.Schema)
	}
	if oc.Defs["darkStep9"] != "#fab283" {
		t.Errorf("defs.darkStep9 = %v", oc.Defs["darkStep9"])
	}
	v, ok := oc.Theme["primary"].(map[string]any)
	if !ok {
		t.Fatalf("theme.primary type = %T", oc.Theme["primary"])
	}
	if v["dark"] != "darkStep9" || v["light"] != "lightStep9" {
		t.Errorf("theme.primary = %v", v)
	}
}

func TestIsTheme(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want bool
	}{
		{"nil", nil, false},
		{"string", "kanagawa", false},
		{"slice", []any{"a"}, false},
		{"no-theme-key", map[string]any{"defs": map[string]any{}}, false},
		{"theme-not-object", map[string]any{"theme": "dark"}, false},
		{"ok", map[string]any{"theme": map[string]any{"primary": "#fff"}}, true},
	}
	for _, c := range cases {
		if got := IsTheme(c.v); got != c.want {
			t.Errorf("IsTheme(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
