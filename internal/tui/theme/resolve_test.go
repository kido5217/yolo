package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type goldenEntry struct {
	Colors                  map[string]string `json:"-"`
	ThinkingOpacity         float64           `json:"thinkingOpacity"`
	HasSelectedListItemText bool              `json:"_hasSelectedListItemText"`
}

// UnmarshalJSON splits the flat token map from the two bookkeeping fields.
func (e *goldenEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.Colors = make(map[string]string, len(raw))
	for k, v := range raw {
		switch k {
		case "thinkingOpacity":
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("golden %q: thinkingOpacity is %T, want number", k, v)
			}
			e.ThinkingOpacity = f
		case "_hasSelectedListItemText":
			b, ok := v.(bool)
			if !ok {
				return fmt.Errorf("golden %q: _hasSelectedListItemText is %T, want bool", k, v)
			}
			e.HasSelectedListItemText = b
		default:
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("golden %q: color value is %T, want string", k, v)
			}
			e.Colors[k] = s
		}
	}
	return nil
}

func loadGolden(t *testing.T) map[string]*goldenEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "theme-golden.json"))
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	got := map[string]*goldenEntry{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("golden: %v", err)
	}
	return got
}

// TestResolveThemeGoldenMatrix: 33 themes × {dark, light}, every token hex +
// thinkingOpacity + _hasSelectedListItemText must equal the upstream-oracle
// golden (spec §7 verification item 1).
func TestResolveThemeGoldenMatrix(t *testing.T) {
	themes, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	golden := loadGolden(t)
	if len(golden) != 33*2 {
		t.Fatalf("golden entries = %d, want 66", len(golden))
	}
	for name, tj := range themes {
		for _, mode := range []string{"dark", "light"} {
			key := name + "." + mode
			t.Run(name+"-"+mode, func(t *testing.T) {
				t.Parallel()
				want, ok := golden[key]
				if !ok {
					t.Errorf("golden missing %s", key)
					return
				}
				got, err := ResolveTheme(tj, mode)
				if err != nil {
					t.Errorf("%s: %v", key, err)
					return
				}
				if got.ThinkingOpacity != want.ThinkingOpacity {
					t.Errorf("%s: thinkingOpacity = %v, want %v", key, got.ThinkingOpacity, want.ThinkingOpacity)
				}
				if got.HasSelectedListItemText != want.HasSelectedListItemText {
					t.Errorf("%s: _hasSelectedListItemText = %v, want %v",
						key, got.HasSelectedListItemText, want.HasSelectedListItemText)
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
				for token := range got.Colors {
					if _, ok := want.Colors[token]; !ok {
						t.Errorf("%s: unexpected token %s", key, token)
					}
				}
			})
		}
	}
}

// TestResolveEdgeCases: value shapes the 33 assets never exercise — ANSI
// ints, "none", circular refs, missing refs (upstream throw, Go returns
// errors with the upstream message).
func TestResolveEdgeCases(t *testing.T) {
	t.Run("ansi-int", func(t *testing.T) {
		t.Parallel()
		tj := ThemeJSON{Theme: map[string]any{"primary": float64(196), "background": "#000000"}}
		got, err := ResolveTheme(tj, "dark")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if want := FromHex("#ff0000"); got.Colors["primary"] != want {
			t.Errorf("primary = %v, want %v", got.Colors["primary"], want)
		}
	})
	t.Run("ansi-cube-and-ramp", func(t *testing.T) {
		t.Parallel()
		if got, want := AnsiToRGBA(16), FromHex("#000000"); got != want {
			t.Errorf("ansi 16 = %v, want %v", got, want)
		}
		if got, want := AnsiToRGBA(195), FromHex("#d7ffff"); got != want {
			t.Errorf("ansi 195 = %v, want %v", got, want)
		}
		if got, want := AnsiToRGBA(231), FromHex("#ffffff"); got != want {
			t.Errorf("ansi 231 = %v, want %v", got, want)
		}
		if got, want := AnsiToRGBA(255), FromHex("#eeeeee"); got != want {
			t.Errorf("ansi 255 = %v, want %v", got, want)
		}
		if got, want := AnsiToRGBA(256), FromHex("#000000"); got != want {
			t.Errorf("ansi 256 (invalid) = %v, want %v", got, want)
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		tj := ThemeJSON{Theme: map[string]any{"primary": "none", "background": "#000000"}}
		got, err := ResolveTheme(tj, "dark")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if want := (RGBA{0, 0, 0, 0}); got.Colors["primary"] != want {
			t.Errorf("primary = %v, want transparent", got.Colors["primary"])
		}
	})
	t.Run("circular-ref", func(t *testing.T) {
		t.Parallel()
		tj := ThemeJSON{Defs: map[string]any{"a": "b"}, Theme: map[string]any{"primary": "a", "background": "#000000"}}
		tj.Theme = map[string]any{"a": "b", "b": "a", "background": "#000000"}
		if _, err := ResolveTheme(tj, "dark"); err == nil {
			t.Fatal("circular ref must error")
		}
	})
	t.Run("missing-ref", func(t *testing.T) {
		t.Parallel()
		tj := ThemeJSON{Theme: map[string]any{"primary": "ghost", "background": "#000000"}}
		if _, err := ResolveTheme(tj, "dark"); err == nil {
			t.Fatal("missing ref must error")
		}
	})
	t.Run("selectedListItemText-fallback-to-background", func(t *testing.T) {
		t.Parallel()
		tj := ThemeJSON{Theme: map[string]any{"background": "#112233"}}
		got, err := ResolveTheme(tj, "dark")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if got.HasSelectedListItemText {
			t.Error("HasSelectedListItemText must be false")
		}
		if want := FromHex("#112233"); got.Colors["selectedListItemText"] != want {
			t.Errorf("selectedListItemText = %v, want %v (background)", got.Colors["selectedListItemText"], want)
		}
		if want := FromHex("#112233"); got.Colors["backgroundMenu"] != want {
			t.Errorf("backgroundMenu = %v, want %v (backgroundElement fallback of absent element = background)",
				got.Colors["backgroundMenu"], want)
		}
	})
}
