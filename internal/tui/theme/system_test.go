package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// hex16Light is the light-terminal palette fixture variant; the standard
// hex16 (palette_test.go) covers the dark fixtures.
var hex16Light = [16]string{
	"#000000", "#7f0000", "#007f00", "#7f7f00", "#00007f", "#7f007f", "#007f7f", "#e5e5e5",
	"#e5e5e5", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff",
}

// TestGenerateSystemGolden: 4 palette fixtures × {dark, light} against the
// upstream-oracle golden (spec §3 system theme; spec §7 item 1).
func TestGenerateSystemGolden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "system-golden.json"))
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	golden := map[string]*goldenEntry{}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("golden: %v", err)
	}
	if len(golden) != 8 {
		t.Fatalf("golden entries = %d, want 8", len(golden))
	}
	fixtures := map[string]TerminalColors{
		"black":     {Palette: hex16, DefaultForeground: "#ffffff", DefaultBackground: "#000000"},
		"mid-dark":  {Palette: hex16, DefaultForeground: "#d4d4d4", DefaultBackground: "#1e1e1e"},
		"white":     {Palette: hex16Light, DefaultForeground: "#000000", DefaultBackground: "#ffffff"},
		"mid-light": {Palette: hex16Light, DefaultForeground: "#1a1a1a", DefaultBackground: "#f0f0f0"},
	}
	for name, colors := range fixtures {
		for _, mode := range []string{"dark", "light"} {
			key := "system." + name + "." + mode
			t.Run(name+"-"+mode, func(t *testing.T) {
				t.Parallel()
				want := golden[key]
				if want == nil {
					t.Fatalf("golden missing %s", key)
				}
				tj := GenerateSystem(colors, mode)
				got, err := ResolveTheme(tj, mode)
				if err != nil {
					t.Fatalf("%s: resolve: %v", key, err)
				}
				if got.ThinkingOpacity != want.ThinkingOpacity {
					t.Errorf("%s: thinkingOpacity = %v, want %v", key, got.ThinkingOpacity, want.ThinkingOpacity)
				}
				for token, wantHex := range want.Colors {
					c, ok := got.Colors[token]
					if !ok {
						t.Errorf("%s: token %s missing", key, token)
						continue
					}
					if c.Hex() != wantHex {
						t.Errorf("%s: %s = %s, want %s", key, token, c.Hex(), wantHex)
					}
				}
			})
		}
	}
}

// TestGenerateSystemPaletteFallbacks: missing palette entries fall back to
// the ANSI table; missing default bg/fg fall back to palette[0]/palette[7].
func TestGenerateSystemPaletteFallbacks(t *testing.T) {
	t.Parallel()
	var empty TerminalColors // all "" — pure ANSI fallback
	tj := GenerateSystem(empty, "dark")
	got, err := ResolveTheme(tj, "dark")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if c, _ := got.Color("error"); c != AnsiToRGBA(1) {
		t.Errorf("error (ansi 1 fallback) = %v", c)
	}
	if c, _ := got.Color("text"); c != AnsiToRGBA(7) {
		t.Errorf("text (fg = palette[7] fallback) = %v", c)
	}
	if c, _ := got.Color("background"); c != (RGBA{0, 0, 0, 0}) {
		t.Errorf("background (transparent, bg = palette[0]) = %v", c)
	}
}

// TestTerminalModeGolden: the luminance rule with the upstream boundary —
// 0.299r+0.587g+0.114b > 0.5 (on 0-255: > 127.5) → light.
func TestTerminalModeGolden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "terminal-mode-golden.json"))
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	want := map[string]string{}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("golden: %v", err)
	}
	for bgHex, wantMode := range want {
		t.Run(bgHex, func(t *testing.T) {
			t.Parallel()
			got := TerminalMode(bgHex)
			if wantMode == "" && got != "" {
				t.Errorf("TerminalMode(%q) = %q, want empty", bgHex, got)
				return
			}
			if wantMode != "" && got != wantMode {
				t.Errorf("TerminalMode(%q) = %q, want %q", bgHex, got, wantMode)
			}
		})
	}
	if got := TerminalMode(""); got != "" {
		t.Errorf("TerminalMode(\"\") = %q, want empty", got)
	}
}
