package theme

import (
	"charm.land/lipgloss/v2"
	"fmt"
	"image/color"
	"testing"
)

// hexOf renders a lipgloss v2 color (image/color.Color) as 6-digit hex for
// assertions; the v1 API returned the lipgloss.Color string type directly.
func hexOf(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

func testTheme(t *testing.T) Theme {
	t.Helper()
	themes, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := ResolveTheme(themes["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	return Theme{R: r, Name: "opencode", Mode: "dark"}
}

func TestThemeForegroundAccessors(t *testing.T) {
	th := testTheme(t)
	cases := []struct {
		name string
		got  color.Color
		want string
	}{
		{"text", th.Text().GetForeground(), "#eeeeee"},
		{"textMuted", th.TextMuted().GetForeground(), "#808080"},
		{"primary", th.Primary().GetForeground(), "#fab283"},
		{"error", th.Error().GetForeground(), "#e06c75"},
		{"border", th.Border().GetForeground(), "#484848"},
		{"markdownHeading", th.MarkdownHeading().GetForeground(), "#9d7cd8"},
		{"syntaxKeyword", th.SyntaxKeyword().GetForeground(), "#9d7cd8"},
		{"diffAdded", th.DiffAdded().GetForeground(), "#4fd6be"},
	}
	for _, c := range cases {
		if got := hexOf(c.got); got != c.want {
			t.Errorf("%s fg = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestThemeBackgroundAccessors(t *testing.T) {
	th := testTheme(t)
	if got, want := hexOf(th.Background().GetBackground()), "#0a0a0a"; got != want {
		t.Errorf("background = %s, want %s", got, want)
	}
	if got, want := hexOf(th.BackgroundPanel().GetBackground()), "#141414"; got != want {
		t.Errorf("backgroundPanel = %s, want %s", got, want)
	}
}

func TestThemeTransparentTokenPaintsNothing(t *testing.T) {
	themes, _ := AllThemes()
	r, err := ResolveTheme(themes["lucent-orng"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := Theme{R: r, Name: "lucent-orng", Mode: "dark"}
	if bg, ok := th.R.Color("background"); !ok || bg.A != 0 {
		t.Fatalf("lucent-orng background = %+v, want alpha 0", bg)
	}
	if _, isNo := th.Background().GetBackground().(lipgloss.NoColor); !isNo {
		t.Errorf("transparent background must not paint, got %v", th.Background().GetBackground())
	}
}

func TestThemeSelectedForeground(t *testing.T) {
	// opaque background, no explicit selectedListItemText → background
	th := testTheme(t)
	if got, want := th.SelectedForeground().Hex(), FromHex("#0a0a0a").Hex(); got != want {
		t.Errorf("SelectedForeground (opaque) = %s, want %s", got, want)
	}
	// transparent background → contrast against the given bg. Synthetic
	// theme: transparent background and NO selectedListItemText — every
	// bundled transparent-bg theme (e.g. lucent-orng) defines an explicit
	// selectedListItemText, and upstream selectedForeground
	// (theme/index.ts:95-111) checks _hasSelectedListItemText FIRST, so
	// the contrast branch would never run on a bundled theme.
	syn := ThemeJson{Theme: map[string]any{"background": "transparent"}}
	r3, err := ResolveTheme(syn, "dark")
	if err != nil {
		t.Fatalf("ResolveTheme(synthetic): %v", err)
	}
	st := Theme{R: r3, Name: "synthetic-transparent", Mode: "dark"}
	if got := st.SelectedForeground(FromHex("#ffffff")); got != (Rgba{0, 0, 0, 255}) {
		t.Errorf("SelectedForeground (light bg) = %v, want black", got)
	}
	if got := st.SelectedForeground(FromHex("#000000")); got != (Rgba{255, 255, 255, 255}) {
		t.Errorf("SelectedForeground (dark bg) = %v, want white", got)
	}
	// explicit selectedListItemText wins (orng defines one)
	themes, _ := AllThemes()
	r2, _ := ResolveTheme(themes["orng"], "dark")
	ot := Theme{R: r2, Name: "orng", Mode: "dark"}
	want, _ := r2.Color("selectedListItemText")
	if got := ot.SelectedForeground(); got != want {
		t.Errorf("SelectedForeground (explicit) = %v, want %v", got, want)
	}
}
