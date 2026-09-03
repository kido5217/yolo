## S0 — Theme Engine + App-Shell Restyle (slice bead `yolo-oae.1`)

S0 is the one horizontal slice (spec §3): foundational theme infrastructure with
no user-visible output until Tasks 8–10 restyle the existing shell. **S0 dep
gate:** none beyond the `charmbracelet/x/term` promotion (Task 5).

Package layout (locked): `internal/tui/theme/{assets/,embed.go,theme.go,
resolve.go,styles.go,system.go,palette.go,discover.go,kv.go,engine.go,
syntax.go,testdata/}`. Files land with the task that owns them; one concern
per file. (`syntax.go` — the ported `generateSyntax`/`generateSubtleSyntax` —
is named in spec §3's S0 package layout, but the spec's binding S0 task list
has no syntax task and the binding S1 list owns the syntax/chroma work:
`syntax.go` lands in Task S1.2.)

### Task S0.1: Embed 33 upstream theme JSONs + `ThemeJson` model + parse tests (`yolo-oae.1.1`)

**Files:**
- Create: `internal/tui/theme/assets/*.json` — the 33 upstream assets, byte-verbatim
- Create: `internal/tui/theme/embed.go`
- Create: `internal/tui/theme/theme.go`
- Test: `internal/tui/theme/theme_test.go`

**Interfaces:**
- Consumes: upstream `packages/tui/src/theme/assets/*.json` (33 files).
- Produces: `theme.ThemeJson` (struct, JSON shape), `theme.AllThemes() (map[string]ThemeJson, error)` (name = asset file stem), `theme.IsTheme(v any) bool`, `theme.DefaultName = "opencode"`, `theme.assetsFS` (unexported `embed.FS`).

- [ ] **Step 1: Write the failing test**

Create `internal/tui/theme/theme_test.go`:

```go
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
	"opencode", "orng", "osaka-jade", "palenight", "rosepine", "solarized",
	"synthwave84", "tokyonight", "vercel", "vesper", "zenburn",
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

func TestParseOpencodeThemeShape(t *testing.T) {
	themes, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	oc := themes["opencode"]
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/theme/ -v`
Expected: FAIL — `no non-test Go files in internal/tui/theme` (package absent).

- [ ] **Step 3: Copy the assets byte-verbatim, then write the minimal implementation**

Copy + verify byte-identity against upstream:

```sh
mkdir -p internal/tui/theme/assets
cp /tmp/opencode-upstream/packages/tui/src/theme/assets/*.json internal/tui/theme/assets/
ls internal/tui/theme/assets | wc -l            # → 33
(cd /tmp/opencode-upstream/packages/tui/src/theme/assets && sha256sum *.json) > /tmp/upstream-assets.sha
(cd internal/tui/theme/assets && sha256sum -c --quiet /tmp/upstream-assets.sha) && echo VERBATIM
```

Create `internal/tui/theme/embed.go`:

```go
package theme

import (
	"embed"
)

// The 33 upstream theme assets, verbatim (strict-copy bar, spec §1).
//go:embed assets/*.json
var assetsFS embed.FS
```

Create `internal/tui/theme/theme.go`:

```go
// Package theme is the TUI-local theme engine: 33 embedded upstream themes,
// resolution, system-theme generation, terminal palette detection, custom
// discovery, and the selection chain (config > KV > default). TUI-local by
// design (root principle 4): no internal/* imports outside internal/tui —
// every filesystem path is injected by cmd/yolo.
package theme

import (
	"encoding/json"
	"fmt"
)

// DefaultName is the fallback active theme (upstream default, theme.tsx:96).
const DefaultName = "opencode"

// ThemeJson mirrors the upstream theme JSON shape (theme/index.ts:120):
// defs = named color constants; theme = semantic tokens, each a
// {dark,light} variant, a hex string, a defs/theme ref name, or an ANSI int.
type ThemeJson struct {
	Schema string         `json:"$schema,omitempty"`
	Defs   map[string]any `json:"defs,omitempty"`
	Theme  map[string]any `json:"theme"`
}

// AllThemes parses the 33 embedded upstream theme assets. Names are the asset
// file stems (kebab-case preserved: catppuccin-frappe, one-dark, ...).
func AllThemes() (map[string]ThemeJson, error) {
	entries, err := assetsFS.ReadDir("assets")
	if err != nil {
		return nil, fmt.Errorf("theme assets: %w", err)
	}
	out := make(map[string]ThemeJson, len(entries))
	for _, e := range entries {
		name := e.Name()[:len(e.Name())-len(".json")]
		data, err := assetsFS.ReadFile("assets/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("theme asset %s: %w", e.Name(), err)
		}
		var tj ThemeJson
		if err := json.Unmarshal(data, &tj); err != nil {
			return nil, fmt.Errorf("theme asset %s: %w", e.Name(), err)
		}
		if tj.Theme == nil {
			return nil, fmt.Errorf("theme asset %s: not a theme", e.Name())
		}
		out[name] = tj
	}
	return out, nil
}

// IsTheme is the upstream isTheme check (theme/index.ts:194): a non-array
// object with a non-array object "theme" member.
func IsTheme(v any) bool {
	obj, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, ok = obj["theme"].(map[string]any)
	return ok
}
```

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/theme/ -v` — Expected: PASS (3 tests).
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green; gofmt prints nothing.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/
git commit -m "feat: embed 33 upstream theme JSONs + ThemeJson model"
bd close yolo-oae.1.1 --reason "33 assets embedded verbatim (sha256-verified), ThemeJson + parse tests green" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.2: Golden matrix generation + `resolveTheme` port + 33×2 golden tests (`yolo-oae.1.2`)

**Files:**
- Create: `scripts/tui-theme-golden.mjs` — dev-only node script (the golden oracle; runs upstream's PURE resolution functions)
- Create: `internal/tui/theme/testdata/theme-golden.json` — generated output, checked in
- Create: `internal/tui/theme/resolve.go`
- Test: `internal/tui/theme/resolve_test.go`

**Interfaces:**
- Consumes: Task S0.1 `ThemeJson`/`AllThemes`.
- Produces: `theme.Rgba` (`uint8` RGBA; `Hex()` = `#rrggbbaa`), `theme.FromHex(string) Rgba`, `theme.AnsiToRgba(int) Rgba`, `theme.Resolved` (`Colors map[string]Rgba` incl. `selectedListItemText`/`backgroundMenu`, `ThinkingOpacity float64`, `HasSelectedListItemText bool`), `theme.ResolveTheme(ThemeJson, mode string) (Resolved, error)` (mode = `"dark"|"light"`).

**Strict-copy note (binding for the port):** upstream `RGBA` is float 0-1 but
every color in the matrix is int-derived (hex/ints/ANSI) or produced by
`tint`/grays, which are float ops on 0-255 values with `Math.round`/
`Math.floor` at the end. Storing `uint8` 0-255 and preserving the upstream
operation ORDER makes float results bit-identical (same IEEE 754 ops). The
node oracle and the Go port must disagree on NOTHING: the golden test is the
proof.

- [ ] **Step 1: Write the failing test + the oracle script, generate the golden**

Create `internal/tui/theme/resolve_test.go`:

```go
package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type goldenEntry struct {
	Colors                 map[string]string `json:"-"`
	ThinkingOpacity        float64           `json:"thinkingOpacity"`
	HasSelectedListItemText bool            `json:"_hasSelectedListItemText"`
}

// UnmarshalJSON splits the flat token map from the two bookkeeping fields.
func (e *goldenEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.Colors = make(map[string]string)
	for k, v := range raw {
		switch k {
		case "thinkingOpacity":
			e.ThinkingOpacity = v.(float64)
		case "_hasSelectedListItemText":
			e.HasSelectedListItemText = v.(bool)
		default:
			e.Colors[k] = v.(string)
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
			want, ok := golden[key]
			if !ok {
				t.Errorf("golden missing %s", key)
				continue
			}
			got, err := ResolveTheme(tj, mode)
			if err != nil {
				t.Errorf("%s: %v", key, err)
				continue
			}
			if got.ThinkingOpacity != want.ThinkingOpacity {
				t.Errorf("%s: thinkingOpacity = %v, want %v", key, got.ThinkingOpacity, want.ThinkingOpacity)
			}
			if got.HasSelectedListItemText != want.HasSelectedListItemText {
				t.Errorf("%s: _hasSelectedListItemText = %v, want %v", key, got.HasSelectedListItemText, want.HasSelectedListItemText)
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
		}
	}
}

// TestResolveEdgeCases: value shapes the 33 assets never exercise — ANSI
// ints, "none", circular refs, missing refs (upstream throw, Go returns
// errors with the upstream message).
func TestResolveEdgeCases(t *testing.T) {
	t.Run("ansi-int", func(t *testing.T) {
		tj := ThemeJson{Theme: map[string]any{"primary": float64(196), "background": "#000000"}}
		got, err := ResolveTheme(tj, "dark")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if want := FromHex("#ff0000"); got.Colors["primary"] != want {
			t.Errorf("primary = %v, want %v", got.Colors["primary"], want)
		}
	})
	t.Run("ansi-cube-and-ramp", func(t *testing.T) {
		if got, want := AnsiToRgba(16), FromHex("#000000"); got != want {
			t.Errorf("ansi 16 = %v, want %v", got, want)
		}
		if got, want := AnsiToRgba(195), FromHex("#d75f5f"); got != want {
			t.Errorf("ansi 195 = %v, want %v", got, want)
		}
		if got, want := AnsiToRgba(231), FromHex("#fffffe"); got != want {
			t.Errorf("ansi 231 = %v, want %v", got, want)
		}
		if got, want := AnsiToRgba(255), FromHex("#ffffff"); got != want {
			t.Errorf("ansi 255 = %v, want %v", got, want)
		}
		if got, want := AnsiToRgba(256), FromHex("#000000"); got != want {
			t.Errorf("ansi 256 (invalid) = %v, want %v", got, want)
		}
	})
	t.Run("none", func(t *testing.T) {
		tj := ThemeJson{Theme: map[string]any{"primary": "none", "background": "#000000"}}
		got, err := ResolveTheme(tj, "dark")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if want := Rgba{0, 0, 0, 0}; got.Colors["primary"] != want {
			t.Errorf("primary = %v, want transparent", got.Colors["primary"])
		}
	})
	t.Run("circular-ref", func(t *testing.T) {
		tj := ThemeJson{Defs: map[string]any{"a": "b"}, Theme: map[string]any{"primary": "a", "background": "#000000"}}
		tj.Theme = map[string]any{"a": "b", "b": "a", "background": "#000000"}
		if _, err := ResolveTheme(tj, "dark"); err == nil {
			t.Fatal("circular ref must error")
		}
	})
	t.Run("missing-ref", func(t *testing.T) {
		tj := ThemeJson{Theme: map[string]any{"primary": "ghost", "background": "#000000"}}
		if _, err := ResolveTheme(tj, "dark"); err == nil {
			t.Fatal("missing ref must error")
		}
	})
	t.Run("selectedListItemText-fallback-to-background", func(t *testing.T) {
		tj := ThemeJson{Theme: map[string]any{"background": "#112233"}}
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
			t.Errorf("backgroundMenu = %v, want %v (backgroundElement fallback of absent element = background)", got.Colors["backgroundMenu"], want)
		}
	})
}
```

Create the oracle `scripts/tui-theme-golden.mjs` (dev-only; NEVER in CI — spec §7):

```js
// Golden-matrix oracle for internal/tui/theme. Ports the upstream PURE
// resolution functions (packages/tui/src/theme/index.ts resolveTheme +
// generateSystem + @opentui/core 0.4.5 RGBA) so the Go port is verified
// bit-for-bit against upstream. Run at repo root:
//   node scripts/tui-theme-golden.mjs
// Writes internal/tui/theme/testdata/theme-golden.json (checked in).
import { readdirSync, readFileSync, writeFileSync } from "node:fs";

// --- @opentui/core 0.4.5 RGBA (int 0-255 representation; bit-identical) ---
function hexToRgb(hex) {
  hex = hex.replace(/^#/, "");
  if (hex.length === 3) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
  else if (hex.length === 4) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2] + hex[3] + hex[3];
  if (!/^[0-9A-Fa-f]{6}$/.test(hex) && !/^[0-9A-Fa-f]{8}$/.test(hex)) return [255, 0, 255, 255]; // upstream: magenta + console.warn
  const r = parseInt(hex.slice(0, 2), 16), g = parseInt(hex.slice(2, 4), 16), b = parseInt(hex.slice(4, 6), 16);
  const a = hex.length === 8 ? parseInt(hex.slice(6, 8), 16) : 255;
  return [r, g, b, a];
}
const toByte = (v) => Math.round(Math.max(0, Math.min(255, Number.isFinite(v) ? v : 0)));
const ANSI_16 = ["#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"];
function ansiToRgba(code) {
  if (code < 16) return hexToRgb(ANSI_16[code] ?? "#000000");
  if (code < 232) {
    const index = code - 16;
    const b = index % 6, g = Math.floor(index / 6) % 6, r = Math.floor(index / 36);
    const val = (x) => (x === 0 ? 0 : x * 40 + 55);
    return [val(r), val(g), val(b), 255];
  }
  if (code < 256) { const gray = (code - 232) * 10 + 8; return [gray, gray, gray, 255]; }
  return [0, 0, 0, 255];
}
const toHex8 = (c) => "#" + c.map((v) => toByte(v).toString(16).padStart(2, "0")).join("");

// --- theme/index.ts resolveTheme (lines 241-299) ---
function resolveTheme(theme, mode) {
  const defs = theme.defs ?? {};
  function resolveColor(c, chain = []) {
    if (typeof c === "string") {
      if (c === "transparent" || c === "none") return [0, 0, 0, 0];
      if (c.startsWith("#")) return hexToRgb(c);
      if (chain.includes(c)) throw new Error(`Circular color reference: ${[...chain, c].join(" -> ")}`);
      const next = defs[c] ?? theme.theme[c];
      if (next === undefined) throw new Error(`Color reference "${c}" not found in defs or theme`);
      return resolveColor(next, [...chain, c]);
    }
    if (typeof c === "number") return ansiToRgba(c);
    return resolveColor(c[mode], chain);
  }
  const resolved = {};
  for (const [key, value] of Object.entries(theme.theme)) {
    if (key === "selectedListItemText" || key === "backgroundMenu" || key === "thinkingOpacity") continue;
    resolved[key] = resolveColor(value);
  }
  const hasSelectedListItemText = theme.theme.selectedListItemText !== undefined;
  resolved.selectedListItemText = hasSelectedListItemText ? resolveColor(theme.theme.selectedListItemText) : resolved.background;
  resolved.backgroundMenu = theme.theme.backgroundMenu !== undefined ? resolveColor(theme.theme.backgroundMenu) : resolved.backgroundElement;
  return { resolved, thinkingOpacity: theme.theme.thinkingOpacity ?? 0.6, hasSelectedListItemText };
}

// --- theme/index.ts tint (346) + generateGrayScale (471) + generateMutedTextColor (525) + generateSystem (360) + terminalMode (353) ---
// tint preserves the upstream FLOAT 0-1 operation order exactly (base.r +
// (overlay.r - base.r) * alpha, then Math.round(r * 255)) so JS/Go results
// are bit-identical (same IEEE 754 ops).
const tint = (base, overlay, alpha) => [
  Math.round((base[0] / 255 + (overlay[0] / 255 - base[0] / 255) * alpha) * 255),
  Math.round((base[1] / 255 + (overlay[1] / 255 - base[1] / 255) * alpha) * 255),
  Math.round((base[2] / 255 + (overlay[2] / 255 - base[2] / 255) * alpha) * 255),
  255,
];
function generateGrayScale(bg, isDark) {
  const grays = {};
  const bgR = bg[0], bgG = bg[1], bgB = bg[2];
  const luminance = 0.299 * bgR + 0.587 * bgG + 0.114 * bgB;
  for (let i = 1; i <= 12; i++) {
    const factor = i / 12.0;
    let newR, newG, newB;
    if (isDark) {
      if (luminance < 10) {
        const grayValue = Math.floor(factor * 0.4 * 255);
        newR = grayValue; newG = grayValue; newB = grayValue;
      } else {
        const newLum = luminance + (255 - luminance) * factor * 0.4;
        const ratio = newLum / luminance;
        newR = Math.min(bgR * ratio, 255); newG = Math.min(bgG * ratio, 255); newB = Math.min(bgB * ratio, 255);
      }
    } else {
      if (luminance > 245) {
        const grayValue = Math.floor(255 - factor * 0.4 * 255);
        newR = grayValue; newG = grayValue; newB = grayValue;
      } else {
        const newLum = luminance * (1 - factor * 0.4);
        const ratio = newLum / luminance;
        newR = Math.max(bgR * ratio, 0); newG = Math.max(bgG * ratio, 0); newB = Math.max(bgB * ratio, 0);
      }
    }
    grays[i] = [Math.floor(newR), Math.floor(newG), Math.floor(newB), 255];
  }
  return grays;
}
function generateMutedTextColor(bg, isDark) {
  const bgLum = 0.299 * bg[0] + 0.587 * bg[1] + 0.114 * bg[2];
  let grayValue;
  if (isDark) {
    if (bgLum < 10) grayValue = 180;
    else grayValue = Math.min(Math.floor(160 + bgLum * 0.3), 200);
  } else {
    if (bgLum > 245) grayValue = 75;
    else grayValue = Math.max(Math.floor(100 - (255 - bgLum) * 0.2), 60);
  }
  return [grayValue, grayValue, grayValue, 255];
}
// colors: { palette: [16 hex], defaultBackground, defaultForeground } (int form: arrays)
function generateSystem(colors, mode) {
  const bg = colors.defaultBackground ?? hexToRgb(colors.palette[0]);
  const fg = colors.defaultForeground ?? hexToRgb(colors.palette[7]);
  const transparent = [bg[0], bg[1], bg[2], 0];
  const isDark = mode === "dark";
  const col = (i) => (colors.palette[i] ? hexToRgb(colors.palette[i]) : ansiToRgba(i));
  const grays = generateGrayScale(bg, isDark);
  const textMuted = generateMutedTextColor(bg, isDark);
  const ansiColors = { black: col(0), red: col(1), green: col(2), yellow: col(3), blue: col(4), magenta: col(5), cyan: col(6), white: col(7), redBright: col(9), greenBright: col(10) };
  const diffAlpha = isDark ? 0.22 : 0.14;
  const diffAddedBg = tint(bg, ansiColors.green, diffAlpha);
  const diffRemovedBg = tint(bg, ansiColors.red, diffAlpha);
  const diffContextBg = grays[2];
  const diffAddedLineNumberBg = tint(diffContextBg, ansiColors.green, diffAlpha);
  const diffRemovedLineNumberBg = tint(diffContextBg, ansiColors.red, diffAlpha);
  return { theme: {
    primary: ansiColors.cyan, secondary: ansiColors.magenta, accent: ansiColors.cyan,
    error: ansiColors.red, warning: ansiColors.yellow, success: ansiColors.green, info: ansiColors.cyan,
    text: fg, textMuted, selectedListItemText: bg,
    background: transparent, backgroundPanel: grays[2], backgroundElement: grays[3], backgroundMenu: grays[3],
    borderSubtle: grays[6], border: grays[7], borderActive: grays[8],
    diffAdded: ansiColors.green, diffRemoved: ansiColors.red, diffContext: grays[7], diffHunkHeader: grays[7],
    diffHighlightAdded: ansiColors.greenBright, diffHighlightRemoved: ansiColors.redBright,
    diffAddedBg, diffRemovedBg, diffContextBg, diffLineNumber: textMuted,
    diffAddedLineNumberBg, diffRemovedLineNumberBg,
    markdownText: fg, markdownHeading: fg, markdownLink: ansiColors.blue, markdownLinkText: ansiColors.cyan,
    markdownCode: ansiColors.green, markdownBlockQuote: ansiColors.yellow, markdownEmph: ansiColors.yellow,
    markdownStrong: fg, markdownHorizontalRule: grays[7], markdownListItem: ansiColors.blue,
    markdownListEnumeration: ansiColors.cyan, markdownImage: ansiColors.blue, markdownImageText: ansiColors.cyan,
    markdownCodeBlock: fg,
    syntaxComment: textMuted, syntaxKeyword: ansiColors.magenta, syntaxFunction: ansiColors.blue,
    syntaxVariable: fg, syntaxString: ansiColors.green, syntaxNumber: ansiColors.yellow,
    syntaxType: ansiColors.cyan, syntaxOperator: ansiColors.cyan, syntaxPunctuation: fg,
  } };
}
function terminalMode(colors) {
  const bg = colors.defaultBackground;
  if (!bg) return undefined;
  const [r, g, b] = hexToRgb(bg);
  return 0.299 * r + 0.587 * g + 0.114 * b > 0.5 ? "light" : "dark";
}

// --- main ---
const assets = "internal/tui/theme/assets";
const themes = {};
for (const f of readdirSync(assets).sort()) {
  if (!f.endsWith(".json")) continue;
  themes[f.replace(/\.json$/, "")] = JSON.parse(readFileSync(`${assets}/${f}`, "utf8"));
}
const out = {};
for (const [name, tj] of Object.entries(themes)) {
  for (const mode of ["dark", "light"]) {
    const { resolved, thinkingOpacity, hasSelectedListItemText } = resolveTheme(tj, mode);
    const entry = { thinkingOpacity, _hasSelectedListItemText: hasSelectedListItemText };
    for (const [k, v] of Object.entries(resolved)) entry[k] = toHex8(v);
    out[`${name}.${mode}`] = entry;
  }
}
// File 1: the 33x2 matrix (consumed by S0.2's golden test).
writeFileSync("internal/tui/theme/testdata/theme-golden.json", JSON.stringify(out, null, 2) + "\n");
console.log(`wrote internal/tui/theme/testdata/theme-golden.json (${Object.keys(out).length} entries)`);

// File 2: system-theme fixtures (consumed by S0.4): near-black, mid-dark,
// near-white, mid-light backgrounds — each exercises both grays/muted branches.
const XTERM = ["#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"];
const LIGHT16 = ["#000000", "#7f0000", "#007f00", "#7f7f00", "#00007f", "#7f007f", "#007f7f", "#e5e5e5", "#e5e5e5", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff"];
const FIXTURES = {
  "black": { palette: XTERM, defaultBackground: "#000000", defaultForeground: "#ffffff" },
  "mid-dark": { palette: XTERM, defaultBackground: "#1e1e1e", defaultForeground: "#d4d4d4" },
  "white": { palette: LIGHT16, defaultBackground: "#ffffff", defaultForeground: "#000000" },
  "mid-light": { palette: LIGHT16, defaultBackground: "#f0f0f0", defaultForeground: "#1a1a1a" },
};
const sysOut = {};
for (const [name, colors] of Object.entries(FIXTURES)) {
  for (const mode of ["dark", "light"]) {
    const { resolved, thinkingOpacity, hasSelectedListItemText } = resolveTheme(generateSystem(colors, mode), mode);
    const entry = { thinkingOpacity, _hasSelectedListItemText: hasSelectedListItemText };
    for (const [k, v] of Object.entries(resolved)) entry[k] = toHex8(v);
    sysOut[`system.${name}.${mode}`] = entry;
  }
}
writeFileSync("internal/tui/theme/testdata/system-golden.json", JSON.stringify(sysOut, null, 2) + "\n");
console.log(`wrote internal/tui/theme/testdata/system-golden.json (${Object.keys(sysOut).length} entries)`);

// File 3: terminalMode luminance boundaries (consumed by S0.4).
writeFileSync("internal/tui/theme/testdata/terminal-mode-golden.json", JSON.stringify({
  "#000000": terminalMode({ defaultBackground: "#000000" }),
  "#ffffff": terminalMode({ defaultBackground: "#ffffff" }),
  "#7f7f7f": terminalMode({ defaultBackground: "#7f7f7f" }),
  "#808080": terminalMode({ defaultBackground: "#808080" }),
  "missing": terminalMode({}),
}, null, 2) + "\n");
console.log("wrote internal/tui/theme/testdata/terminal-mode-golden.json");
```

- [ ] **Step 2: Generate the golden + verify the Go test fails**

Run at module root:

```sh
node scripts/tui-theme-golden.mjs        # writes internal/tui/theme/testdata/theme-golden.json
go test ./internal/tui/theme/ -run TestResolve -v
```

Expected: oracle prints `wrote … (66 + 8 entries + _terminalMode)` (66 = 33×2,
plus 8 system fixture entries); Go test FAILS — `undefined: ResolveTheme`
(and the golden file must be staged: `git add -N internal/tui/theme/testdata/`).

- [ ] **Step 3: Write the minimal implementation**

Create `internal/tui/theme/resolve.go`:

```go
package theme

import (
	"fmt"
	"strconv"
	"strings"
)

// Rgba is a 0-255 color with alpha. Upstream RGBA is float 0-1 but every
// color is int-derived (hex/int/ANSI) or produced by float ops on 0-255
// values rounded at the end (tint, grays), so uint8 storage is exact and the
// operation ORDER is preserved for bit-identical results (strict-copy bar).
type Rgba struct{ R, G, B, A uint8 }

// Hex is the golden-matrix form "#rrggbbaa".
func (c Rgba) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

func isHex(h string, n int) bool {
	if len(h) != n {
		return false
	}
	for _, r := range h {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// FromHex is the port of @opentui/core hexToRgb (0.4.5): strips "#", expands
// 3→6 and 4→8 digits, accepts 6/8-digit hex; invalid input → magenta
// (upstream additionally console.warns — a non-visual log side effect,
// skipped).
func FromHex(s string) Rgba {
	h := strings.TrimPrefix(s, "#")
	switch len(h) {
	case 3:
		h = h[0:1] + h[0:1] + h[1:2] + h[1:2] + h[2:3] + h[2:3]
	case 4:
		h = h[0:1] + h[0:1] + h[1:2] + h[1:2] + h[2:3] + h[2:3] + h[3:4] + h[3:4]
	}
	if !isHex(h, 6) && !isHex(h, 8) {
		return Rgba{255, 0, 255, 255}
	}
	a := 255
	if len(h) == 8 {
		a = hexByte(h[6:8])
	}
	return Rgba{hexByte(h[0:2]), hexByte(h[2:4]), hexByte(h[4:6]), a}
}

func hexByte(s string) uint8 {
	v, _ := strconv.ParseUint(s, 16, 8)
	return uint8(v)
}

// ansi16 is the upstream standard ANSI table (theme/index.ts:304-321).
var ansi16 = []string{
	"#000000", "#800000", "#008000", "#808000", "#000080", "#800080",
	"#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00",
	"#0000ff", "#ff00ff", "#00ffff", "#ffffff",
}

// AnsiToRgba is the port of upstream ansiToRgba (theme/index.ts:301):
// 0-15 standard, 16-231 the 6x6x6 cube, 232-255 the grayscale ramp,
// anything else black.
func AnsiToRgba(code int) Rgba {
	if code < 16 {
		hex := "#000000"
		if code >= 0 && code < len(ansi16) {
			hex = ansi16[code]
		}
		return FromHex(hex) // upstream: ansiColors[code] ?? "#000000"
	}
	if code < 232 {
		index := code - 16
		b := index % 6
		g := index / 6 % 6
		r := index / 36
		val := func(x int) int {
			if x == 0 {
				return 0
			}
			return x*40 + 55
		}
		return Rgba{uint8(val(r)), uint8(val(g)), uint8(val(b)), 255}
	}
	if code < 256 {
		gray := (code - 232)*10 + 8
		return Rgba{uint8(gray), uint8(gray), uint8(gray), 255}
	}
	return Rgba{0, 0, 0, 255}
}

// Resolved is the output of ResolveTheme: every token (incl. the two
// optional ones) mapped to its resolved color, plus the bookkeeping fields.
type Resolved struct {
	Colors                  map[string]Rgba
	ThinkingOpacity         float64
	HasSelectedListItemText bool
}

// Color returns the resolved token (ok=false when absent).
func (r Resolved) Color(name string) (Rgba, bool) {
	c, ok := r.Colors[name]
	return c, ok
}

// ResolveTheme is the port of upstream resolveTheme
// (theme/index.ts:241-299): defs refs, "#hex", "transparent"/"none",
// ANSI ints, {dark,light} variants; optional selectedListItemText
// (default: background), backgroundMenu (default: backgroundElement),
// thinkingOpacity (default: 0.6). Error messages keep the upstream wording.
func ResolveTheme(j ThemeJson, mode string) (Resolved, error) {
	defs := j.Defs
	var resolve func(c any, chain []string) (Rgba, error)
	resolve = func(c any, chain []string) (Rgba, error) {
		if rgb, ok := c.(Rgba); ok {
			return rgb, nil // generateSystem output values (upstream RGBA instanceof)
		}
		switch v := c.(type) {
		case string:
			if v == "transparent" || v == "none" {
				return Rgba{0, 0, 0, 0}, nil
			}
			if strings.HasPrefix(v, "#") {
				return FromHex(v), nil
			}
			for _, prev := range chain {
				if prev == v {
					return Rgba{}, fmt.Errorf("circular color reference: %s", strings.Join(append(chain, v), " -> "))
				}
			}
			next, ok := defs[v]
			if !ok {
				next, ok = j.Theme[v]
			}
			if !ok {
				return Rgba{}, fmt.Errorf("color reference %q not found in defs or theme", v)
			}
			return resolve(next, append(chain, v))
		case float64: // JSON numbers unmarshal to float64
			return AnsiToRgba(int(v)), nil
		case map[string]any:
			return resolve(v[mode], chain)
		}
		return Rgba{}, fmt.Errorf("unresolvable color value %v (%T)", c, c)
	}
	resolved := make(map[string]Rgba, len(j.Theme))
	for key, value := range j.Theme {
		switch key {
		case "selectedListItemText", "backgroundMenu", "thinkingOpacity":
			continue
		}
		c, err := resolve(value, nil)
		if err != nil {
			return Resolved{}, fmt.Errorf("token %s: %w", key, err)
		}
		resolved[key] = c
	}
	hasSLIT := j.Theme["selectedListItemText"] != nil
	if hasSLIT {
		c, err := resolve(j.Theme["selectedListItemText"], nil)
		if err != nil {
			return Resolved{}, fmt.Errorf("token selectedListItemText: %w", err)
		}
		resolved["selectedListItemText"] = c
	} else {
		resolved["selectedListItemText"] = resolved["background"]
	}
	if j.Theme["backgroundMenu"] != nil {
		c, err := resolve(j.Theme["backgroundMenu"], nil)
		if err != nil {
			return Resolved{}, fmt.Errorf("token backgroundMenu: %w", err)
		}
		resolved["backgroundMenu"] = c
	} else {
		resolved["backgroundMenu"] = resolved["backgroundElement"]
	}
	thinkingOpacity := 0.6
	if v, ok := j.Theme["thinkingOpacity"].(float64); ok {
		thinkingOpacity = v
	}
	return Resolved{
		Colors:                  resolved,
		ThinkingOpacity:         thinkingOpacity,
		HasSelectedListItemText: hasSLIT,
	}, nil
}
```

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/theme/ -v` — Expected: PASS (the 33×2 golden
matrix + edge cases + Task S0.1 tests).
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green; gofmt prints nothing. If the matrix disagrees with the
oracle: the Go port deviated from the upstream operation order — fix the Go
side, NEVER edit the golden to match a wrong port.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/ scripts/tui-theme-golden.mjs
git commit -m "feat: port resolveTheme + 33x2 golden matrix"
bd close yolo-oae.1.2 --reason "resolveTheme port verified bit-for-bit vs the node oracle (33x2 golden + edge cases)" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.3: `Theme` struct + lipgloss style accessors + tests (`yolo-oae.1.3`)

**Files:**
- Create: `internal/tui/theme/styles.go`
- Test: `internal/tui/theme/styles_test.go`

**Interfaces:**
- Consumes: Task S0.2 `Resolved`, `Rgba`.
- Produces: `theme.Theme` (`R Resolved`, `Name string`, `Mode string`) with
  one lipgloss accessors per token (components never see hex): `Text`,
  `TextMuted`, `Primary`, `Secondary`, `Accent`, `Error`, `Warning`,
  `Success`, `Info`, `Border`, `BorderActive`, `BorderSubtle`,
  `Background`, `BackgroundPanel`, `BackgroundElement`, `BackgroundMenu`,
  `MarkdownText`, `MarkdownHeading`, `MarkdownLink`, `MarkdownLinkText`,
  `MarkdownCode`, `MarkdownBlockQuote`, `MarkdownEmph`, `MarkdownStrong`,
  `MarkdownHorizontalRule`, `MarkdownListItem`, `MarkdownListEnumeration`,
  `MarkdownImage`, `MarkdownImageText`, `MarkdownCodeBlock`,
  `SyntaxComment`, `SyntaxKeyword`, `SyntaxFunction`, `SyntaxVariable`,
  `SyntaxString`, `SyntaxNumber`, `SyntaxType`, `SyntaxOperator`,
  `SyntaxPunctuation`, `DiffAdded`, `DiffRemoved`, `DiffContext`,
  `DiffHunkHeader`, `DiffHighlightAdded`, `DiffHighlightRemoved`,
  `DiffAddedBg`, `DiffRemovedBg`, `DiffContextBg`, `DiffLineNumber`,
  `DiffAddedLineNumberBg`, `DiffRemovedLineNumberBg` — plus
  `Color(name string) (Rgba, bool)` (test hook) and
  `SelectedForeground(bg ...Rgba) Rgba` (port of upstream
  `selectedForeground`, theme/index.ts:95-111).

**Alpha semantics (binding):** a token with `A == 0` (upstream
"transparent", e.g. lucent-orng) means "do not paint". Foreground accessors
apply the 6-digit hex; background accessors emit NO background style when
`A == 0` or the token is absent. **Profile note (log at S0 close-out,
render/low):** lipgloss renders colors through the terminal color profile
(truecolor → exact 24-bit SGR; 256-color profile → ANSI256 quantization)
whereas opentui always emits 24-bit SGR — the data is exact by construction;
the quantization is terminal-capability territory.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/theme/styles_test.go`:

```go
package theme

import (
	"charm.land/lipgloss/v2"
	"testing"
)

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
		got  lipgloss.Color
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
		if string(c.got) != c.want {
			t.Errorf("%s fg = %q, want %q", c.name, string(c.got), c.want)
		}
	}
}

func TestThemeBackgroundAccessors(t *testing.T) {
	th := testTheme(t)
	if got, want := string(th.Background().GetBackground()), "#0a0a0a"; got != want {
		t.Errorf("background = %q, want %q", got, want)
	}
	if got, want := string(th.BackgroundPanel().GetBackground()), "#141414"; got != want {
		t.Errorf("backgroundPanel = %q, want %q", got, want)
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
	if got := string(th.Background().GetBackground()); got != "" {
		t.Errorf("transparent background must not paint, got %q", got)
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/theme/ -run TestTheme -v`
Expected: FAIL — `undefined: Theme`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/tui/theme/styles.go` — the accessor list above, each
one-liner over two helpers:

```go
package theme

import (
	"charm.land/lipgloss/v2"
)

// Theme is a resolved theme + its name/mode, exposing lipgloss styles —
// components never see hex (spec §3).
type Theme struct {
	R    Resolved
	Name string
	Mode string // "dark" | "light"
}

// Color is the raw-token accessor (test hook + generic consumers).
func (t Theme) Color(name string) (Rgba, bool) { return t.R.Color(name) }

// fg returns a foreground style for the token; an absent token or a
// transparent (alpha 0) token yields an empty style (no foreground).
func (t Theme) fg(token string) lipgloss.Style {
	c, ok := t.R.Color(token)
	if !ok || c.A == 0 {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()[:7]))
}

// bg returns a background style for the token; absent/transparent → no
// background (alpha semantics, see the Task interface note).
func (t Theme) bg(token string) lipgloss.Style {
	c, ok := t.R.Color(token)
	if !ok || c.A == 0 {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Background(lipgloss.Color(c.Hex()[:7]))
}

// SelectedForeground is the port of upstream selectedForeground
// (theme/index.ts:95-111): explicit selectedListItemText wins; transparent
// background → contrast against bg (or primary) via the luminance rule
// (0.299r+0.587g+0.114b > 0.5 → black, else white); else background.
func (t Theme) SelectedForeground(bg ...Rgba) Rgba {
	if t.R.HasSelectedListItemText {
		c, _ := t.R.Color("selectedListItemText")
		return c
	}
	background, _ := t.R.Color("background")
	if background.A == 0 {
		target := background
		if len(bg) > 0 {
			target = bg[0]
		} else if c, ok := t.R.Color("primary"); ok {
			target = c
		}
		lum := 0.299*float64(target.R) + 0.587*float64(target.G) + 0.114*float64(target.B)
		if lum > 0.5*255 {
			return Rgba{0, 0, 0, 255}
		}
		return Rgba{255, 255, 255, 255}
	}
	return background
}
```

Then the accessor bodies (exact list from the Interfaces block), e.g.:

```go
func (t Theme) Text() lipgloss.Style            { return t.fg("text") }
func (t Theme) TextMuted() lipgloss.Style       { return t.fg("textMuted") }
func (t Theme) Primary() lipgloss.Style         { return t.fg("primary") }
func (t Theme) Secondary() lipgloss.Style       { return t.fg("secondary") }
func (t Theme) Accent() lipgloss.Style          { return t.fg("accent") }
func (t Theme) Error() lipgloss.Style           { return t.fg("error") }
func (t Theme) Warning() lipgloss.Style         { return t.fg("warning") }
func (t Theme) Success() lipgloss.Style         { return t.fg("success") }
func (t Theme) Info() lipgloss.Style            { return t.fg("info") }
func (t Theme) Border() lipgloss.Style          { return t.fg("border") }
func (t Theme) BorderActive() lipgloss.Style    { return t.fg("borderActive") }
func (t Theme) BorderSubtle() lipgloss.Style    { return t.fg("borderSubtle") }
func (t Theme) Background() lipgloss.Style            { return t.bg("background") }
func (t Theme) BackgroundPanel() lipgloss.Style       { return t.bg("backgroundPanel") }
func (t Theme) BackgroundElement() lipgloss.Style     { return t.bg("backgroundElement") }
func (t Theme) BackgroundMenu() lipgloss.Style        { return t.bg("backgroundMenu") }
func (t Theme) MarkdownText() lipgloss.Style           { return t.fg("markdownText") }
func (t Theme) MarkdownHeading() lipgloss.Style        { return t.fg("markdownHeading") }
func (t Theme) MarkdownLink() lipgloss.Style           { return t.fg("markdownLink") }
func (t Theme) MarkdownLinkText() lipgloss.Style       { return t.fg("markdownLinkText") }
func (t Theme) MarkdownCode() lipgloss.Style           { return t.fg("markdownCode") }
func (t Theme) MarkdownBlockQuote() lipgloss.Style     { return t.fg("markdownBlockQuote") }
func (t Theme) MarkdownEmph() lipgloss.Style           { return t.fg("markdownEmph") }
func (t Theme) MarkdownStrong() lipgloss.Style         { return t.fg("markdownStrong") }
func (t Theme) MarkdownHorizontalRule() lipgloss.Style { return t.fg("markdownHorizontalRule") }
func (t Theme) MarkdownListItem() lipgloss.Style       { return t.fg("markdownListItem") }
func (t Theme) MarkdownListEnumeration() lipgloss.Style { return t.fg("markdownListEnumeration") }
func (t Theme) MarkdownImage() lipgloss.Style          { return t.fg("markdownImage") }
func (t Theme) MarkdownImageText() lipgloss.Style      { return t.fg("markdownImageText") }
func (t Theme) MarkdownCodeBlock() lipgloss.Style      { return t.fg("markdownCodeBlock") }
func (t Theme) SyntaxComment() lipgloss.Style          { return t.fg("syntaxComment") }
func (t Theme) SyntaxKeyword() lipgloss.Style          { return t.fg("syntaxKeyword") }
func (t Theme) SyntaxFunction() lipgloss.Style         { return t.fg("syntaxFunction") }
func (t Theme) SyntaxVariable() lipgloss.Style         { return t.fg("syntaxVariable") }
func (t Theme) SyntaxString() lipgloss.Style           { return t.fg("syntaxString") }
func (t Theme) SyntaxNumber() lipgloss.Style           { return t.fg("syntaxNumber") }
func (t Theme) SyntaxType() lipgloss.Style             { return t.fg("syntaxType") }
func (t Theme) SyntaxOperator() lipgloss.Style         { return t.fg("syntaxOperator") }
func (t Theme) SyntaxPunctuation() lipgloss.Style      { return t.fg("syntaxPunctuation") }
func (t Theme) DiffAdded() lipgloss.Style              { return t.fg("diffAdded") }
func (t Theme) DiffRemoved() lipgloss.Style            { return t.fg("diffRemoved") }
func (t Theme) DiffContext() lipgloss.Style            { return t.fg("diffContext") }
func (t Theme) DiffHunkHeader() lipgloss.Style         { return t.fg("diffHunkHeader") }
func (t Theme) DiffHighlightAdded() lipgloss.Style     { return t.fg("diffHighlightAdded") }
func (t Theme) DiffHighlightRemoved() lipgloss.Style   { return t.fg("diffHighlightRemoved") }
func (t Theme) DiffAddedBg() lipgloss.Style            { return t.bg("diffAddedBg") }
func (t Theme) DiffRemovedBg() lipgloss.Style          { return t.bg("diffRemovedBg") }
func (t Theme) DiffContextBg() lipgloss.Style          { return t.bg("diffContextBg") }
func (t Theme) DiffLineNumber() lipgloss.Style         { return t.fg("diffLineNumber") }
func (t Theme) DiffAddedLineNumberBg() lipgloss.Style  { return t.bg("diffAddedLineNumberBg") }
func (t Theme) DiffRemovedLineNumberBg() lipgloss.Style { return t.bg("diffRemovedLineNumberBg") }
```

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/theme/ -v` — Expected: PASS.
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green; gofmt prints nothing.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/
git commit -m "feat: Theme struct + lipgloss style accessors"
bd close yolo-oae.1.3 --reason "57 token accessors + SelectedForeground port + alpha semantics tested" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.4: `generateSystem` port + tests (`yolo-oae.1.4`)

**Files:**
- Create: `internal/tui/theme/system.go`
- Test: `internal/tui/theme/system_test.go`

**Interfaces:**
- Consumes: Task S0.2 `Rgba`/`FromHex`/`AnsiToRgba`/`ResolveTheme`.
- Produces: `theme.TerminalColors` (`Palette [16]string`, `DefaultForeground`,
  `DefaultBackground` — `""` = unknown), `theme.GenerateSystem(TerminalColors,
  mode string) ThemeJson` (theme values are `Rgba` — consumed by
  `ResolveTheme`'s Rgba branch, mirroring upstream's RGBA-instance branch),
  `theme.Tint(base, overlay Rgba, alpha float64) Rgba`,
  `theme.TerminalMode(bgHex string) string` (`"dark"|"light"|""`).

**Strict-copy note (binding):** `Tint` and the gray-scale math must keep the
upstream FLOAT operation order (0-1 floats in `tint`; 0-255 float ops with
`Math.floor`/`Math.min`/`Math.max` in the grays) — the oracle
(`scripts/tui-theme-golden.mjs`) and this port must be bit-identical or the
golden test fails.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/theme/system_test.go`:

```go
package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
		"black":     {Palette: [16]string{"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"}, DefaultForeground: "#ffffff", DefaultBackground: "#000000"},
		"mid-dark":  {Palette: [16]string{"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"}, DefaultForeground: "#d4d4d4", DefaultBackground: "#1e1e1e"},
		"white":     {Palette: [16]string{"#000000", "#7f0000", "#007f00", "#7f7f00", "#00007f", "#7f007f", "#007f7f", "#e5e5e5", "#e5e5e5", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff"}, DefaultForeground: "#000000", DefaultBackground: "#ffffff"},
		"mid-light": {Palette: [16]string{"#000000", "#7f0000", "#007f00", "#7f7f00", "#00007f", "#7f007f", "#007f7f", "#e5e5e5", "#e5e5e5", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff"}, DefaultForeground: "#1a1a1a", DefaultBackground: "#f0f0f0"},
	}
	for name, colors := range fixtures {
		for _, mode := range []string{"dark", "light"} {
			key := "system." + name + "." + mode
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
		}
	}
}

// TestGenerateSystemPaletteFallbacks: missing palette entries fall back to
// the ANSI table; missing default bg/fg fall back to palette[0]/palette[7].
func TestGenerateSystemPaletteFallbacks(t *testing.T) {
	var empty TerminalColors // all "" — pure ANSI fallback
	tj := GenerateSystem(empty, "dark")
	got, err := ResolveTheme(tj, "dark")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if c, _ := got.Color("error"); c != AnsiToRgba(1) {
		t.Errorf("error (ansi 1 fallback) = %v", c)
	}
	if c, _ := got.Color("text"); c != AnsiToRgba(7) {
		t.Errorf("text (fg = palette[7] fallback) = %v", c)
	}
	if c, _ := got.Color("background"); c != Rgba{0, 0, 0, 0} {
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
		got := TerminalMode(bgHex)
		if wantMode == "" && got != "" {
			t.Errorf("TerminalMode(%q) = %q, want empty", bgHex, got)
			continue
		}
		if wantMode != "" && got != wantMode {
			t.Errorf("TerminalMode(%q) = %q, want %q", bgHex, got, wantMode)
		}
	}
	if got := TerminalMode(""); got != "" {
		t.Errorf("TerminalMode(\"\") = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/theme/ -run 'TestGenerateSystem|TestTerminalMode' -v`
Expected: FAIL — `undefined: GenerateSystem` / `undefined: TerminalColors`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/tui/theme/system.go` — a verbatim port of upstream
`generateSystem` (theme/index.ts:360-469) + `generateGrayScale` (471-523) +
`generateMutedTextColor` (525-554) + `tint` (346-351) + `terminalMode`
(353-358). The full function set:

```go
package theme

import (
	"math"
)

// TerminalColors is the palette result (port of @opentui/core
// TerminalColors): the 16-color palette + default fg/bg; "" = unknown.
type TerminalColors struct {
	Palette             [16]string
	DefaultForeground   string
	DefaultBackground   string
}

// Tint is the port of upstream tint (theme/index.ts:346): overlay blended
// onto base with alpha, in the upstream FLOAT 0-1 operation order.
func Tint(base, overlay Rgba, alpha float64) Rgba {
	r := float64(base.R)/255 + (float64(overlay.R)/255-float64(base.R)/255)*alpha
	g := float64(base.G)/255 + (float64(overlay.G)/255-float64(base.G)/255)*alpha
	b := float64(base.B)/255 + (float64(overlay.B)/255-float64(base.B)/255)*alpha
	return Rgba{uint8(math.Round(r * 255)), uint8(math.Round(g * 255)), uint8(math.Round(b * 255)), 255}
}

// generateGrayScale is the port of upstream generateGrayScale
// (theme/index.ts:471-523): 12 steps derived from the background luminance,
// branch on luminance < 10 (dark) / > 245 (light).
func generateGrayScale(bg Rgba, isDark bool) [13]Rgba {
	var grays [13]Rgba
	luminance := 0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)
	for i := 1; i <= 12; i++ {
		factor := float64(i) / 12.0
		var newR, newG, newB float64
		if isDark {
			if luminance < 10 {
				grayValue := math.Floor(factor * 0.4 * 255)
				newR, newG, newB = grayValue, grayValue, grayValue
			} else {
				newLum := luminance + (255-luminance)*factor*0.4
				ratio := newLum / luminance
				newR = math.Min(float64(bg.R)*ratio, 255)
				newG = math.Min(float64(bg.G)*ratio, 255)
				newB = math.Min(float64(bg.B)*ratio, 255)
			}
		} else {
			if luminance > 245 {
				grayValue := math.Floor(255 - factor*0.4*255)
				newR, newG, newB = grayValue, grayValue, grayValue
			} else {
				newLum := luminance * (1 - factor * 0.4)
				ratio := newLum / luminance
				newR = math.Max(float64(bg.R)*ratio, 0)
				newG = math.Max(float64(bg.G)*ratio, 0)
				newB = math.Max(float64(bg.B)*ratio, 0)
			}
		}
		grays[i] = Rgba{uint8(math.Floor(newR)), uint8(math.Floor(newG)), uint8(math.Floor(newB)), 255}
	}
	return grays
}

// generateMutedTextColor is the port of upstream generateMutedTextColor
// (theme/index.ts:525-554).
func generateMutedTextColor(bg Rgba, isDark bool) Rgba {
	bgLum := 0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)
	var grayValue float64
	if isDark {
		if bgLum < 10 {
			grayValue = 180
		} else {
			grayValue = math.Min(math.Floor(160+bgLum*0.3), 200)
		}
	} else {
		if bgLum > 245 {
			grayValue = 75
		} else {
			grayValue = math.Max(math.Floor(100-(255-bgLum)*0.2), 60)
		}
	}
	g := uint8(grayValue)
	return Rgba{g, g, g, 255}
}

// GenerateSystem is the port of upstream generateSystem
// (theme/index.ts:360-469): terminal palette + default fg/bg → generated
// ThemeJson. Theme values are Rgba (ResolveTheme's Rgba branch), mirroring
// upstream's RGBA-instance values; missing palette entries fall back to the
// ANSI table, missing default bg/fg to palette[0]/palette[7].
func GenerateSystem(colors TerminalColors, mode string) ThemeJson {
	bg := FromHex("")
	if colors.DefaultBackground != "" {
		bg = FromHex(colors.DefaultBackground)
	} else {
		bg = FromHex(colors.Palette[0])
	}
	fg := FromHex("")
	if colors.DefaultForeground != "" {
		fg = FromHex(colors.DefaultForeground)
	} else {
		fg = FromHex(colors.Palette[7])
	}
	transparent := Rgba{bg.R, bg.G, bg.B, 0}
	isDark := mode == "dark"
	col := func(i int) Rgba {
		if colors.Palette[i] != "" {
			return FromHex(colors.Palette[i])
		}
		return AnsiToRgba(i)
	}
	grays := generateGrayScale(bg, isDark)
	textMuted := generateMutedTextColor(bg, isDark)
	ansiColors := map[string]Rgba{
		"black": col(0), "red": col(1), "green": col(2), "yellow": col(3),
		"blue": col(4), "magenta": col(5), "cyan": col(6), "white": col(7),
		"redBright": col(9), "greenBright": col(10),
	}
	diffAlpha := 0.14
	if isDark {
		diffAlpha = 0.22
	}
	diffAddedBg := Tint(bg, ansiColors["green"], diffAlpha)
	diffRemovedBg := Tint(bg, ansiColors["red"], diffAlpha)
	diffContextBg := grays[2]
	diffAddedLineNumberBg := Tint(diffContextBg, ansiColors["green"], diffAlpha)
	diffRemovedLineNumberBg := Tint(diffContextBg, ansiColors["red"], diffAlpha)
	return ThemeJson{Theme: map[string]any{
		"primary": ansiColors["cyan"], "secondary": ansiColors["magenta"], "accent": ansiColors["cyan"],
		"error": ansiColors["red"], "warning": ansiColors["yellow"], "success": ansiColors["green"], "info": ansiColors["cyan"],
		"text": fg, "textMuted": textMuted, "selectedListItemText": bg,
		"background": transparent, "backgroundPanel": grays[2], "backgroundElement": grays[3], "backgroundMenu": grays[3],
		"borderSubtle": grays[6], "border": grays[7], "borderActive": grays[8],
		"diffAdded": ansiColors["green"], "diffRemoved": ansiColors["red"], "diffContext": grays[7], "diffHunkHeader": grays[7],
		"diffHighlightAdded": ansiColors["greenBright"], "diffHighlightRemoved": ansiColors["redBright"],
		"diffAddedBg": diffAddedBg, "diffRemovedBg": diffRemovedBg, "diffContextBg": diffContextBg, "diffLineNumber": textMuted,
		"diffAddedLineNumberBg": diffAddedLineNumberBg, "diffRemovedLineNumberBg": diffRemovedLineNumberBg,
		"markdownText": fg, "markdownHeading": fg, "markdownLink": ansiColors["blue"], "markdownLinkText": ansiColors["cyan"],
		"markdownCode": ansiColors["green"], "markdownBlockQuote": ansiColors["yellow"], "markdownEmph": ansiColors["yellow"],
		"markdownStrong": fg, "markdownHorizontalRule": grays[7], "markdownListItem": ansiColors["blue"],
		"markdownListEnumeration": ansiColors["cyan"], "markdownImage": ansiColors["blue"], "markdownImageText": ansiColors["cyan"],
		"markdownCodeBlock": fg,
		"syntaxComment": textMuted, "syntaxKeyword": ansiColors["magenta"], "syntaxFunction": ansiColors["blue"],
		"syntaxVariable": fg, "syntaxString": ansiColors["green"], "syntaxNumber": ansiColors["yellow"],
		"syntaxType": ansiColors["cyan"], "syntaxOperator": ansiColors["cyan"], "syntaxPunctuation": fg,
	}}
}

// TerminalMode is the port of upstream terminalMode
// (theme/index.ts:353-358): bg luminance > 0.5 (0-1) → "light", else
// "dark"; no bg → "".
func TerminalMode(bgHex string) string {
	if bgHex == "" {
		return ""
	}
	c := FromHex(bgHex)
	return luminanceMode(float64(c.R), float64(c.G), float64(c.B))
}

// luminanceMode is the shared luminance rule: 0.299r+0.587g+0.114b > 0.5.
// Inputs are 0-255; the comparison runs on the 0-1 scale, upstream-exact.
func luminanceMode(r, g, b float64) string {
	if 0.299*r/255+0.587*g/255+0.114*b/255 > 0.5 {
		return "light"
	}
	return "dark"
}
```

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/theme/ -v` — Expected: PASS.
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green; gofmt prints nothing.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/
git commit -m "feat: port generateSystem (terminal palette to theme)"
bd close yolo-oae.1.4 --reason "generateSystem + grays/muted/tint + terminalMode verified vs the node oracle (4 fixtures x 2)" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.5: OSC 11/10/4 palette detection + luminance mode + fast-fail (`yolo-oae.1.5`)

**Files:**
- Create: `internal/tui/theme/palette.go`
- Test: `internal/tui/theme/palette_test.go`
- Modify: `go.mod` / `go.sum` — promote `github.com/charmbracelet/x/term` v0.2.2 from indirect to direct (AFTER approval, Step 1)

**Interfaces:**
- Consumes: Task S0.4 `TerminalColors`; upstream OSC mechanics in
  `@opentui/core` 0.4.5 `src/lib/terminal-palette.ts` (reference at
  `/tmp/opencode/.opentui-core/package/chunk-node-q0cwyvm9.js:10050-10330`).
- Produces: `theme.PaletteOptions` (`ProbeTimeout`, `IdleTimeout`,
  `HardTimeout time.Duration` — defaults 100ms/100ms/100ms, spec-pinned (upstream
  300ms/300ms/5s; spec-reconciliation note above);
  `LegacyTmux bool`), `theme.DetectPalette(in io.Reader, out io.Writer, opts
  PaletteOptions) (TerminalColors, bool)` (bool = OSC supported; false →
  no system theme, the spec §3 fallback), `theme.DetectStd(ctx context.Context)
  (TerminalColors, bool)` (raw-mode `/dev/tty` wrapper, x/term).

**Dep proposal (Step 1 — the approval gate; spec §2 + root dependency policy):**
file the bead BEFORE touching go.mod:

```sh
bd create "dep proposal: promote charmbracelet/x/term v0.2.2 (OSC raw tty)" \
  --parent=yolo-oae.1 -t task -p 1 \
  --description="Promote github.com/charmbracelet/x/term v0.2.2 from indirect to direct require. Evidence (live-verified 2026-08-24): (1) already in the module graph at v0.2.2 — indirect via charm.land/bubbletea/v2 v2.0.9 (go.sum pin present); promotion adds ZERO new modules/transitive surface; (2) license: MIT (charmbracelet/x, same org as the allowlisted charm.land stack); (3) maintenance: active — the bubbletea v2 runtime dependency (MakeRaw/Restore/GetState used by bubbletea itself); (4) why stdlib/hand-rolling is inadequate: reading BEL-terminated OSC responses requires raw (non-canonical) tty mode — canonical-mode reads block until newline, and the stdlib has no termios raw-mode API (hand-rolling TCGETS/TCSETS ioctals duplicates x/term for one call site); (5) the spec's ecosystem survey already classifies the charm x/ modules as 'already in the transitive graph — usable, not new'. Request: user approval, then go mod tidy promotes the require." --json
```

Then **STOP and wait for explicit user approval** (chat or bead reply). Only
after approval run `go mod tidy` (it flips the require to direct; no version
change — v0.2.2 stays pinned) and verify: `grep 'charmbracelet/x/term' go.mod`
shows it without `// indirect`.

**Strict-copy note (binding):** the port keeps the upstream mechanics verbatim
— support probe `ESC ] 4 ; 0 ; ? BEL` (100 ms — spec-pinned, see
spec-reconciliation note); palette queries `ESC ] 4 ; {0..15} ; ? BEL`
(tmux-wrapped in legacy tmux); special-color queries
`ESC ] {10..17,19} ; ? BEL` (10-12 only in legacy tmux; NOT tmux-wrapped);
response regexes `ESC]4;(\d+);(?:rgb:([0-9a-fA-F]+)/...|#([0-9a-fA-F]{6}))(?:BEL|ESC\)`
(scaling `rgb:` components to 8 bits: `round(val/maxIn*255)`); per-query idle
timer (100 ms, reset per response) + hard timeout (100 ms, measured from the
query phase); input buffer capped 8192→keep-last-4096; no TTY / no probe
response → `false` (→ no system theme, active falls back to `"opencode"` —
spec §3, upstream's own catch path, theme.tsx:155-178).

**Spec-reconciliation note (decision):** spec §3 pins "~100 ms timeout; no
response → no system theme" and risk 3 pins "hard 100 ms fallback"; upstream's
own constants are 300 ms probe / 300 ms idle / 5 s hard. This task pins the
SPEC values (100/100/100 — worst-case startup block ~200 ms; partial palette
answers are still upstream-semantics-usable since only `palette[0]` gates the
system theme) and appends DEVIATIONS.md entry 122 (behavior/low, sanctioned
by the approved spec) in Step 6 — root principle 2: logged in the same commit
that lands the change. `PaletteOptions` keeps all three timeouts overridable;
flipping back to the upstream constants is a one-line change.

- [ ] **Step 1: File the dep proposal bead, then STOP for approval**

Run the `bd create` above. **STOP** — report the bead id and the evidence
summary; wait for the user's approval. (On approval: `go mod tidy`, verify the
go.mod flip, and continue to Step 2. On rejection: STOP and report — the task
is blocked, the slice gate needs a design change the user must call.)

- [ ] **Step 2: Write the failing test**

Create `internal/tui/theme/palette_test.go` (hermetic — scripted I/O, fast
timeouts):

```go
package theme

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fastOpts() PaletteOptions {
	return PaletteOptions{ProbeTimeout: 50 * time.Millisecond, IdleTimeout: 50 * time.Millisecond, HardTimeout: 200 * time.Millisecond}
}

func resp4(i int, hex string) string { return "\x1b]4;" + strconv.Itoa(i) + ";" + hex + "\x07" }
func respN(i int, hex string) string { return "\x1b]" + strconv.Itoa(i) + ";" + hex + "\x07" }

// fullResponses returns a probe response + all 16 palette + all 9 special
// responses (the complete terminal answer).
func fullResponses() string {
	var b strings.Builder
	b.WriteString(resp4(0, "#111111")) // probe answer (re-consumed by the probe phase)
	for i := 0; i < 16; i++ {
		b.WriteString(resp4(i, hex16[i]))
	}
	for _, i := range []int{10, 11, 12, 13, 14, 15, 16, 17, 19} {
		b.WriteString(respN(i, hex16[i%16]))
	}
	return b.String()
}

var hex16 = []string{"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"}

func TestDetectPaletteFullResponse(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(fullResponses())
	got, ok := DetectPalette(in, &out, fastOpts())
	if !ok {
		t.Fatal("expected OSC support")
	}
	if got.Palette[1] != "#800000" || got.Palette[15] != "#ffffff" {
		t.Errorf("palette = %v", got.Palette)
	}
	if got.DefaultBackground != "#c0c0c0" { // respN(11, hex16[11%16] = "#c0c0c0")
		t.Errorf("DefaultBackground = %q", got.DefaultBackground)
	}
	if got.DefaultForeground != "#c0c0c0" { // respN(10, hex16[10%16] = "#c0c0c0")
		t.Errorf("DefaultForeground = %q", got.DefaultForeground)
	}
	// the queries must have been written, tmux-unwrapped by default
	q := out.String()
	if !strings.Contains(q, "\x1b]4;0;?\x07") || !strings.Contains(q, "\x1b]4;15;?\x07") {
		t.Errorf("palette queries missing: %q", q)
	}
	if !strings.Contains(q, "\x1b]11;?\x07") || !strings.Contains(q, "\x1b]19;?\x07") {
		t.Errorf("special queries missing: %q", q)
	}
}

func TestDetectPaletteRGBScaling(t *testing.T) {
	// the terminal answers the probe with a full 16-bit rgb: value:
	// rgb:1f/3c/5a → each component scaled: round(31/255*255)=31 etc.
	// (maxIn = 1 << (4*len) - 1 = 255 for 2-digit hex — identity at 8-bit)
	probe := resp4(0, "rgb:1f/3c/5a")
	full := probe
	for i := 0; i < 16; i++ {
		full += resp4(i, hex16[i])
	}
	for _, i := range []int{10, 11, 12, 13, 14, 15, 16, 17, 19} {
		full += respN(i, hex16[i%16])
	}
	got, ok := DetectPalette(strings.NewReader(full), &bytes.Buffer{}, fastOpts())
	if !ok {
		t.Fatal("expected OSC support")
	}
	if got.Palette[0] != "#1f3c5a" { // rgb:1f/3c/5a → #1f3c5a (8-bit identity)
		t.Errorf("scaled palette[0] = %q, want #1f3c5a", got.Palette[0])
	}
}

func TestDetectPaletteNoResponseUnsupported(t *testing.T) {
	// no probe answer within ProbeTimeout → unsupported (spec §3: no system
	// theme, active falls back to "opencode").
	in := func() io.Reader {
		pr, pw := io.Pipe()
		go func() { time.Sleep(120 * time.Millisecond); pw.Close() }()
		return pr
	}()
	got, ok := DetectPalette(in, &bytes.Buffer{}, fastOpts())
	if ok {
		t.Fatal("expected unsupported")
	}
	if got != (TerminalColors{}) {
		t.Errorf("colors = %+v, want zero", got)
	}
}

func TestDetectPalettePartialResponseIdleFallsBack(t *testing.T) {
	// the terminal answers only palette indices 0-2, then goes silent: the
	// idle timer ends the query and the partial result is returned (the
	// caller treats palette[0] present as usable; S0.7 decides system-theme
	// eligibility upstream-exactly: palette[0] must be present).
	probe := resp4(0, "#111111")
	partial := probe + resp4(0, "#111111") + resp4(1, "#222222") + resp4(2, "#333333")
	got, ok := DetectPalette(strings.NewReader(partial), &bytes.Buffer{}, fastOpts())
	if !ok {
		t.Fatal("expected OSC support (probe answered)")
	}
	if got.Palette[0] != "#111111" || got.Palette[1] != "#222222" || got.Palette[2] != "#333333" {
		t.Errorf("partial palette = %v", got.Palette)
	}
	if got.Palette[3] != "" {
		t.Errorf("unanswered index must stay empty, got %q", got.Palette[3])
	}
}

func TestDetectPaletteLegacyTmuxWrapping(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(fullResponses())
	opts := fastOpts()
	opts.LegacyTmux = true
	if _, ok := DetectPalette(in, &out, opts); !ok {
		t.Fatal("expected OSC support")
	}
	q := out.String()
	// legacy tmux: ESC ESC doubled, palette query wrapped in Ptmux container;
	// special queries NOT wrapped (upstream writeOsc second arg).
	if !strings.Contains(q, "\x1bPtmux;") {
		t.Errorf("legacy tmux wrapping missing: %q", q)
	}
	if strings.Contains(q, "\x1bPtmux;\x1b\x1b]11;?") {
		t.Errorf("special queries must not be tmux-wrapped: %q", q)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/tui/theme/ -run TestDetectPalette -v`
Expected: FAIL — `undefined: DetectPalette`.

- [ ] **Step 4: Write the minimal implementation**

Create `internal/tui/theme/palette.go`. Structure (port semantics from the
Strict-copy note; the reader demuxes both regexes in one loop):

```go
package theme

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	term "github.com/charmbracelet/x/term"
)

// Response shapes (verbatim from @opentui/core 0.4.5 terminal-palette.ts:
// OSC4_RESPONSE / OSC_SPECIAL_RESPONSE).
var (
	osc4Response     = regexp.MustCompile(`\x1b]4;(\d+);(?:(?:rgb:)([0-9a-fA-F]+)/([0-9a-fA-F]+)/([0-9a-fA-F]+)|#([0-9a-fA-F]{6}))(?:\x07|\x1b\\)`)
	oscSpecialResponse = regexp.MustCompile(`\x1b](\d+);(?:(?:rgb:)([0-9a-fA-F]+)/([0-9a-fA-F]+)/([0-9a-fA-F]+)|#([0-9a-fA-F]{6}))(?:\x07|\x1b\\)`)
)

// toHex8 is the port of toHex (terminal-palette.ts:10065): #hex6 verbatim
// (lowercased), or rgb: components scaled to 8 bits
// (round(val / (16^len-1) * 255)), else #000000.
func toHex8(r, g, b, hex6 string) string { ... }

// wrapForTmux is the port of wrapForTmux (terminal-palette.ts:10072):
// double every ESC and wrap in the Ptmux passthrough container.
func wrapForTmux(osc string) string {
	escaped := strings.ReplaceAll(osc, "\x1b", "\x1b\x1b")
	return "\x1bPtmux;" + escaped + "\x1b\\"
}

type PaletteOptions struct {
	ProbeTimeout time.Duration // default 100ms (spec-pinned; upstream detectOSCSupport is 300ms)
	IdleTimeout  time.Duration // default 100ms (spec-pinned; upstream OTUI_PALETTE_IDLE_TIMEOUT_MS is 300ms)
	HardTimeout  time.Duration // default 100ms (spec-pinned; upstream detect timeout is 5000ms)
	LegacyTmux   bool          // TMUX set && TMUX_PANE unset
}

func (o *PaletteOptions) fill() {
	if o.ProbeTimeout == 0 { o.ProbeTimeout = 100 * time.Millisecond }
	if o.IdleTimeout == 0 { o.IdleTimeout = 100 * time.Millisecond }
	if o.HardTimeout == 0 { o.HardTimeout = 100 * time.Millisecond }
}

// DetectPalette ports TerminalPalette.detect: (1) the OSC 4;0 support probe,
// (2) the 16 palette + 9 special-color queries with per-query idle timers,
// the hard timeout, and the 8192/4096 buffer cap. in/out are injected; the
// TTY preconditions are the caller's job (DetectStd).
func DetectPalette(in io.Reader, out io.Writer, opts PaletteOptions) (TerminalColors, bool) { ... }
```

Implementation requirements (reviewer checklist — the subagent writes the
bodies to these):
1. **Probe phase:** write `wrapForTmux("\x1b]4;0;?\x07")` when `LegacyTmux`,
   else bare; read chunks into a buffer; `osc4Response.MatchString(buffer)`
   → supported; `ProbeTimeout` fires → `(TerminalColors{}, false)`.
2. **Query phase:** write the palette queries
   (`strings.Join` of `"\x1b]4;"+i+";?\x07"` for 0-15, `wrapForTmux`-wrapped
   when `LegacyTmux`) FIRST, then the special queries (10-17,19 — or 10-12
   when `LegacyTmux`, NOT wrapped). One reader goroutine → `chan []byte`;
   main loop `select` over the channel, the palette idle timer, the special
   idle timer, and the hard timer. On each chunk: append to buffer (cap
   8192 → keep last 4096), run BOTH regexes with `FindAllStringSubmatch`,
   store palette indices 0-15 and special indices (10-17,19) via `toHex8`,
   reset the respective idle timer only when a response for that group was
   stored. Per-group done = all indices stored OR its idle timer fired.
   Overall done when both groups are done (or hard timeout / reader EOF).
3. **EOF:** reader close ends the loop with whatever was collected (the
   timers would have fired anyway).
4. **`DetectStd(ctx)`:** if `os.Stdin`/`os.Stdout` are not both char devices
   (`Stat().Mode()&os.ModeCharDevice`), try `os.OpenFile("/dev/tty",
   os.O_RDWR, 0)`; get the fd, `term.MakeRaw(fd)`, `defer term.Restore(fd,
   state)`; run `DetectPalette` with `ctx`-aware deadlines (cancel → close
   the input pipe early); return the result. LegacyTmux =
   `os.Getenv("TMUX") != "" && os.Getenv("TMUX_PANE") == ""`.

- [ ] **Step 5: Run to verify it passes, then gate**

Run: `go test ./internal/tui/theme/ -v` — Expected: PASS.
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green; gofmt prints nothing; `grep 'charmbracelet/x/term' go.mod`
shows the direct require.

- [ ] **Step 6: Commit + close the beads**

Append entry 122 to `docs/superpowers/DEVIATIONS.md` (after entry 121), same
commit (root principle 2):

```
122. OSC palette-detection timeouts 100ms/100ms/100ms vs upstream 300ms/300ms/5s (behavior/low, 2026-08-25): approved spec 2026-08-24-opencode-tui-parity-design.md §3 pins "~100 ms timeout; no response → no system theme" and risk 3 pins "hard 100 ms fallback"; upstream `@opentui/core` 0.4.5 `terminal-palette.ts` uses 300 ms probe / 300 ms idle / 5 s hard. The port keeps the upstream MECHANICS verbatim (probe OSC 4;0, 16+9 queries, tmux wrapping, rgb: scaling, 8192/4096 buffer cap) with the spec-pinned constants; worst-case startup block ~200 ms; partial palettes remain usable (only palette[0] gates the system theme, upstream theme.tsx:159). `theme.PaletteOptions` keeps all three timeouts overridable — restoring the upstream constants is a one-line change.
```

```sh
git add internal/tui/theme/ go.mod go.sum docs/superpowers/DEVIATIONS.md
git commit -m "feat: OSC 11/10/4 terminal palette detection"
bd close yolo-oae.1.5 --reason "OSC probe + 16/9 query port (hermetic tests incl. rgb: scaling, partial idle, legacy-tmux wrap) + x/term promotion" --json
bd close <dep-proposal-bead-id> --reason "approved + landed (direct require v0.2.2, zero new modules)" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.6: Custom theme discovery (config dir + .yolo walk, SIGUSR2 refresh) + tests (`yolo-oae.1.6`)

**Files:**
- Create: `internal/tui/theme/discover.go`
- Test: `internal/tui/theme/discover_test.go`

**Interfaces:**
- Consumes: Task S0.1 `ThemeJson`, `IsTheme` (the test asserts raw non-theme values via `IsTheme`; S0.7's caller filters + casts the discovered values to `ThemeJson`).
- Produces:
  - `theme.ThemeDirs(globalYoloDir, cwd string) []string` — ordered: `globalYoloDir` FIRST, then `<dir>/.yolo` for dir = cwd, parent(cwd), … up to and including the filesystem root (upstream order theme.tsx:38–44; no dedupe). The caller injects `globalYoloDir` — cmd/yolo computes `config.GlobalYoloDir()` (TUI purity, root principle 4: the theme package never imports `internal/config`).
  - `theme.Discover(dirs []string) (map[string]any, error)` — for each dir in order: scan `<dir>/themes/` for `*.json` entries using `os.ReadDir` + suffix filter (NOT `filepath.Glob` — it skips dotfiles; upstream `Glob.scan` runs with `dot:true, symlink:true`: include dotfile names, accept symlink entries that resolve to regular files); name = base name minus `.json`; later dirs override earlier names (upstream object-assignment order, theme.tsx:52–61); return the RAW parsed values (`map[string]any`) — the `IsTheme` filter is the CALLER's job (S0.7, mirroring theme.tsx:137–140), NOT Discover's; an unparseable JSON file is a hard `error` (upstream `JSON.parse` throws → the whole discover fails; the caller's catch sets `active = "opencode"` — S0.7); a missing `<dir>/themes` is skipped without error (upstream `Glob.scan` yields nothing).
  - `theme.WatchThemeSignals(refresh func()) (stop func())` — `signal.Notify` for SIGUSR2; a goroutine forwards every signal to `refresh` (upstream's 250/1000 ms debounce lives in the engine — S0.7 — mirroring theme.tsx:235–244, NOT here); `stop()` stops the goroutine and restores the default SIGUSR2 disposition.

**Strict-copy note (binding):** upstream directory order (global config first, then the cwd → root walk) and later-wins override — the outer project dir beats the inner one, which beats the global dir (the theme.tsx:54–58 assignment order, verbatim); `JSON.parse` throw = whole-discover error, never a per-file skip; the `isTheme` filter is caller-side (theme.tsx:137–140); SIGUSR2 kept (spec §3); yolo's flat `~/.config/yolo/themes` (spec §3) mirrors upstream's flat `~/.config/opencode/themes` — and because yolo has no TUI plugin system, upstream's plugin theme registration (`addTheme`/`upsertTheme`, imported at theme.tsx:16–19) is NOT ported. The 250/1000 ms refresh debounce (theme.tsx:82, 235–244) is deliberately NOT in this task — S0.7's engine owns it, receiving raw signals from `WatchThemeSignals`.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/theme/discover_test.go`:

```go
package theme

import (
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func writeThemeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := filepath.Join(dir, "themes", name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func primaryOf(v any) string {
	obj, _ := v.(map[string]any)
	th, _ := obj["theme"].(map[string]any)
	p, _ := th["primary"].(string)
	return p
}

// TestThemeDirs: the exact upstream order (theme.tsx:38-44) — the injected
// global dir first, then <dir>/.yolo for cwd walking up to and including
// the filesystem root (the /.yolo entry); no dedupe. ThemeDirs is a pure
// string walk (no FS access), so a fixed absolute path is fully hermetic.
func TestThemeDirs(t *testing.T) {
	got := ThemeDirs("/home/u/.config/yolo", "/home/u/proj/pkg")
	want := []string{
		"/home/u/.config/yolo",
		"/home/u/proj/pkg/.yolo",
		"/home/u/proj/.yolo",
		"/home/u/.yolo",
		"/home/.yolo",
		"/.yolo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ThemeDirs = %v, want %v", got, want)
	}
	if got := ThemeDirs("/g", "/"); !reflect.DeepEqual(got, []string{"/g", "/.yolo"}) {
		t.Fatalf("ThemeDirs at the filesystem root = %v, want [/g /.yolo]", got)
	}
}

// TestDiscover: names = stems; later dirs override earlier (upstream
// object-assignment order — the outer project dir beats the inner one,
// which beats the global dir); dotfile names included (upstream dot:true);
// symlinked theme files followed (upstream symlink:true); non-theme JSON
// returned raw (the IsTheme filter is the caller's job — theme.tsx:137-140);
// non-.json files ignored; missing themes dirs skipped without error.
func TestDiscover(t *testing.T) {
	global := t.TempDir()
	base := t.TempDir()
	mid := filepath.Join(base, "mid")
	cwd := filepath.Join(mid, "src")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	writeThemeFile(t, global, "shared.json", `{"theme":{"primary":"#111111"}}`)
	writeThemeFile(t, global, "custom.json", `{"theme":{"primary":"#ffffff"}}`)
	writeThemeFile(t, filepath.Join(base, ".yolo"), "shared.json", `{"theme":{"primary":"#222222"}}`)
	writeThemeFile(t, filepath.Join(mid, ".yolo"), "shared.json", `{"theme":{"primary":"#333333"}}`)
	writeThemeFile(t, filepath.Join(mid, ".yolo"), ".hidden.json", `{"theme":{"primary":"#444444"}}`)
	writeThemeFile(t, filepath.Join(mid, ".yolo"), "notatoken.json", `{"defs":{}}`)
	writeThemeFile(t, filepath.Join(mid, ".yolo"), "notes.txt", "not json")
	link := filepath.Join(mid, ".yolo", "themes", "linked.json")
	if err := os.Symlink(filepath.Join(global, "themes", "custom.json"), link); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(ThemeDirs(global, cwd))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("entries = %v, want 5 (shared, custom, .hidden, linked, notatoken)", got)
	}
	if p := primaryOf(got["shared"]); p != "#222222" {
		t.Errorf("shared = %q, want #222222 (outer project dir overrides inner + global)", p)
	}
	if p := primaryOf(got["custom"]); p != "#ffffff" {
		t.Errorf("custom = %q, want #ffffff (global dir)", p)
	}
	if p := primaryOf(got[".hidden"]); p != "#444444" {
		t.Errorf(".hidden = %q, want #444444 (dotfile name included)", p)
	}
	if p := primaryOf(got["linked"]); p != "#ffffff" {
		t.Errorf("linked = %q, want #ffffff (symlinked theme file followed)", p)
	}
	if IsTheme(got["notatoken"]) {
		t.Error(`notatoken (no "theme" key) must be returned raw — IsTheme filtering is caller-side`)
	}
	if _, ok := got["notes"]; ok {
		t.Error("non-.json entry must not be scanned")
	}
}

// TestDiscoverCorruptFileIsHardError: an unparseable .json fails the whole
// discover (upstream JSON.parse throws; the caller's catch sets active to
// "opencode" — S0.7). Never a per-file skip.
func TestDiscoverCorruptFileIsHardError(t *testing.T) {
	dir := t.TempDir()
	writeThemeFile(t, dir, "good.json", `{"theme":{"primary":"#ffffff"}}`)
	writeThemeFile(t, dir, "bad.json", `{not json`)
	if _, err := Discover([]string{dir}); err == nil {
		t.Fatal("corrupt .json must be a hard error")
	}
}

// TestDiscoverNoThemesDirs: a missing <dir>/themes is skipped (upstream
// Glob.scan yields nothing) — the common case on a clean machine.
func TestDiscoverNoThemesDirs(t *testing.T) {
	got, err := Discover([]string{t.TempDir(), t.TempDir()})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("entries = %v, want none", got)
	}
}

// TestWatchThemeSignals: SIGUSR2 forwards to refresh; after stop() no
// further forwarding. stop() restores the default SIGUSR2 disposition
// (signal.Reset — which would terminate the process), so the post-stop kill
// is guarded with signal.Ignore.
func TestWatchThemeSignals(t *testing.T) {
	var n atomic.Int32
	stop := WatchThemeSignals(func() { n.Add(1) })

	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR2); err != nil {
		t.Fatalf("kill: %v", err)
	}
	for i := 0; i < 200 && n.Load() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if got := n.Load(); got != 1 {
		t.Fatalf("refresh count = %d, want 1 (SIGUSR2 must forward to refresh)", got)
	}
	stop()
	signal.Ignore(syscall.SIGUSR2)
	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR2); err != nil {
		t.Fatalf("kill: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := n.Load(); got != 1 {
		t.Errorf("refresh count after stop = %d, want 1 (no forwarding after stop)", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/theme/ -run 'TestThemeDirs|TestDiscover|TestWatchThemeSignals' -v`
Expected: FAIL — build error: `undefined: ThemeDirs` (and `undefined: Discover`,
`undefined: WatchThemeSignals`) — the implementation is absent.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/tui/theme/discover.go`:

```go
package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// ThemeDirs is the port of the upstream discover directory list
// (theme.tsx:38-44): the injected global config dir FIRST, then <dir>/.yolo
// for every dir from cwd up to and including the filesystem root (upstream:
// <dir>/.opencode under the flat ~/.config/opencode root — yolo: the flat
// ~/.config/yolo, spec §3). No dedupe: Discover's later-dir-wins override
// follows exactly this order (upstream object-assignment order).
func ThemeDirs(globalYoloDir, cwd string) []string {
	dirs := []string{globalYoloDir}
	for current := cwd; ; current = filepath.Dir(current) {
		dirs = append(dirs, filepath.Join(current, ".yolo"))
		if filepath.Dir(current) == current {
			break
		}
	}
	return dirs
}

// Discover is the port of upstream discoverThemes (theme.tsx:52-61): for
// each dir in order, scan <dir>/themes/ for *.json entries — dotfile names
// included, symlink entries followed (upstream Glob.scan dot:true,
// symlink:true); name = base name minus ".json"; later dirs override
// earlier names. A missing themes dir is skipped (upstream Glob.scan yields
// nothing); an unreadable or unparseable file is a hard error (upstream
// JSON.parse throws → the whole discover fails; the caller's catch sets
// active to "opencode" — S0.7). Values are returned RAW: the IsTheme filter
// is the caller's job (theme.tsx:137-140, S0.7), not Discover's.
func Discover(dirs []string) (map[string]any, error) {
	result := map[string]any{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(dir, "themes"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("theme discovery %s: %w", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			full := filepath.Join(dir, "themes", name)
			info, err := os.Stat(full) // follows symlinks (upstream symlink:true)
			if err != nil {
				return nil, fmt.Errorf("theme discovery %s: %w", full, err)
			}
			if !info.Mode().IsRegular() {
				continue // a directory named *.json is not a file match
			}
			data, err := os.ReadFile(full)
			if err != nil {
				return nil, fmt.Errorf("theme discovery %s: %w", full, err)
			}
			var v any
			if err := json.Unmarshal(data, &v); err != nil {
				return nil, fmt.Errorf("theme discovery %s: %w", full, err)
			}
			result[strings.TrimSuffix(name, ".json")] = v
		}
	}
	return result, nil
}

// WatchThemeSignals is the port of upstream subscribeRefresh
// (theme.tsx:46-49): SIGUSR2 → refresh, kept per spec §3. The 250/1000 ms
// debounce (theme.tsx:82, 235-244) lives in the engine (S0.7), not here —
// every signal is forwarded. stop() stops the forwarding goroutine and
// restores the default SIGUSR2 disposition.
func WatchThemeSignals(refresh func()) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR2)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				refresh()
			case <-done:
				return
			}
		}
	}()
	return func() {
		close(done)
		signal.Stop(ch)
		signal.Reset(syscall.SIGUSR2)
	}
}
```

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/theme/ -v` — Expected: PASS (the 5 new discovery
tests + all S0.1–S0.5 tests).
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green; gofmt prints nothing.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/
git commit -m "feat: custom theme discovery (.yolo walk) + SIGUSR2 refresh"
bd close yolo-oae.1.6 --reason "ThemeDirs + Discover (global-first walk, later-wins, dotfile+symlink, corrupt=hard-error, raw values) + SIGUSR2 watcher; hermetic tests green" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.7: Selection chain (config > KV > default) + mode lock + KV file + `config.theme` wire change + app wiring (`yolo-oae.1.7`)

**Files:**
- Create: `internal/tui/theme/kv.go`
- Create: `internal/tui/theme/engine.go`
- Test: `internal/tui/theme/kv_test.go`
- Test: `internal/tui/theme/engine_test.go`
- Modify: `internal/protocol/config.go` — `Theme map[string]any` → `Theme string` (the ONE sanctioned wire change; DEVIATIONS.md entry 123)
- Modify: `internal/config/config_test.go` — re-baseline the `"theme"` fixture + assertion to the string shape (same commit, root principle 3)
- Modify: `cmd/yolo/main.go` — build + Resolve the engine, pass it into `tui.NewApp`, arm the SIGUSR2 watcher
- Modify: `internal/tui/app.go` — `engine`/`theme` fields, `ThemeRefreshMsg`, the 250/1000 ms debounce, `retheme()`
- Modify: `internal/tui/rec_test.go` — `newRecApp` passes a nil engine (the `NewApp` signature change)
- Modify: `internal/tui/app_test.go` — the six `NewApp` call sites (lines 36, 69, 113, 162, 222, 254) pass a nil engine
- Modify: `docs/superpowers/DEVIATIONS.md` — entry 123 (wire/low)

**Interfaces:**
- Consumes:
  - Task S0.1: `ThemeJson`, `AllThemes() (map[string]ThemeJson, error)`, `IsTheme(any) bool`, `DefaultName = "opencode"`.
  - Task S0.2: `ResolveTheme(ThemeJson, string) (Resolved, error)`.
  - Task S0.3: `Theme{R Resolved, Name string, Mode string}` (the construction shape + the S0.8–S0.10 consumption shape).
  - Task S0.4: `TerminalColors`, `GenerateSystem(TerminalColors, string) ThemeJson`, `TerminalMode(string) string`.
  - Task S0.5: `DetectStd(context.Context) (TerminalColors, bool)` — injected as the `Palette` func by cmd/yolo.
  - Task S0.6: `ThemeDirs(string, string) []string`, `Discover([]string) (map[string]any, error)`, `WatchThemeSignals(func()) (stop func())`.
  - Upstream (port source, read at execution time): `packages/tui/src/context/theme.tsx` (the State store, init produce-block 114–125, the config effect 127–130, syncCustomThemes 132–144, onMount 146–150, resolveSystemTheme 152–179, refreshSystemTheme 181–200, apply/pin/free 202–220, the THEME_MODE handler 222–226, the refresh debounce 235–246, the values memo 256–267, `set` 293–298) and `packages/tui/src/context/kv.tsx` (Flock + readJson load, promise-chained ordered writes, writeJsonAtomic, `??` get/set with nil-delete).
  - bubbletea v2: `tea.Tick(d, func(time.Time) tea.Msg) Cmd` and `(*Program).Send(Msg)` (app wiring only).
- Produces (binding for S0.8–S0.10):
  - `theme.KV` (thread-safe); `theme.OpenKV(path string) (*KV, error)` — MkdirAll the parent; missing file → empty store; corrupt JSON → `slog.Warn` + empty store, NO error (upstream catch → console.error → continue); the only returned error is an unwritable parent dir. `(k *KV) Get(key string, def any) any` — `??` semantics (missing or nil → def; JSON `false`/`""`/`0` preserved). `(k *KV) Set(key string, v any)` — a `nil` value DELETES the key (upstream `setStore(key, undefined)`); writes are ordered per-process (single writer goroutine + buffered channel, the promise-chain port) and atomic (temp file + `os.Rename`), cross-process locked with `syscall.Flock(LOCK_EX)` around each write (POSIX — Linux platform scope). `(k *KV) Close() error` — drain pending writes, stop the writer, idempotent.
  - `theme.EngineOptions{KVPath, GlobalYoloDir, CWD string; ConfigTheme string; Palette func(ctx context.Context) (TerminalColors, bool)}`; `theme.New(opts EngineOptions) (*Engine, error)` — state init EXACTLY per theme.tsx:114–125 (lock = pick(KV theme_mode_lock); mode = lock ?? "dark"; stale `theme_mode` cleared when no lock; active = ConfigTheme if non-empty, else KV "theme", else "opencode"). `(e *Engine) Resolve(ctx context.Context) error` — the startup sequence (system theme + custom discovery, ported sequentially); ALWAYS returns nil (both upstream catch paths are swallowed: log + fallback). `(e *Engine) ActiveTheme() (Theme, error)` — the values memo (theme.tsx:256–267). `(e *Engine) Active() string`; `(e *Engine) Mode() string`; `(e *Engine) Locked() bool`; `(e *Engine) Ready() bool`; `(e *Engine) AllThemes() map[string]ThemeJson` (builtins + customs + `"system"` when a system theme exists); `(e *Engine) Has(name string) bool`; `(e *Engine) Set(name string) bool`; `(e *Engine) Pin(mode string)`; `(e *Engine) Free()`; `(e *Engine) Apply(mode string)`; `(e *Engine) ThemeModeEvent(mode string)`; `(e *Engine) Reapply()`; `(e *Engine) RefreshCustoms(ctx context.Context) error` (error path: active = "opencode", customs emptied, error returned); `(e *Engine) Close() error` — flush the KV.
  - `tui.ThemeRefreshMsg` — exported `struct{}`; cmd/yolo sends it to the running program on every theme signal.
  - `tui.NewApp(c *client.Service, s store.State, startSessionID string, engine *theme.Engine) *App` — signature change (a nil engine runs without the theme engine; the zero `theme.Theme` paints nothing — the S0.8+ views degrade to unstyled, never panic).
  - `App.engine *theme.Engine` + `App.theme theme.Theme` — the S0.8–S0.10 consumption points (views read `a.theme`; components never see hex).
  - Wire: `protocol.Config.Theme string` (`json:"theme,omitempty"` — the ONE sanctioned wire change).
**S0 scoping rule (binding, behavior/low):** the terminal palette is probed EXACTLY ONCE, at startup (`Resolve`). All later re-resolution (`Apply`/`Free`/`Reapply`/`RefreshCustoms`) runs on the CACHED palette + fresh customs discovery — NO mid-session re-probe. Upstream re-probes through the opentui renderer's input pipeline (`refreshSystemTheme` clears the palette cache and re-issues the OSC queries, theme.tsx:181–200), which bubbletea cannot share mid-render — the tty belongs to the running program. Note upstream parity check S8 may revisit. Logged as DEVIATIONS.md entry 124 in Step 5.

**Upstream parity notes (binding):**
- **Live config:** the upstream `createEffect` (theme.tsx:127–130) watches `config.theme` live; the Go port takes a config snapshot at startup (`ConfigTheme` is fixed after `New`) — there is no TUI config hot-reload in S0.
- **`theme_mode` is write-only (verbatim):** upstream writes the KV `theme_mode` on `Apply` while locked (theme.tsx:203) but never reads it back — init reads only `theme_mode_lock` (theme.tsx:116–118). The port preserves this verbatim (writes it, clears it on the stale-unlock pass and on `Free`, never reads it).
- **`ThemeModeEvent` has no caller in S0:** no bubbletea source for opentui's `CliRenderEvents.THEME_MODE` (theme.tsx:222–226) exists in S0 — the method lands for API parity, no caller yet.
- **Debounce coalescing:** upstream `refresh()` clears prior timeouts on a re-signal (theme.tsx:236–244); bubbletea v2 has no tick cancellation, so a re-signal re-arms a second 250/1000 ms pair. Both legs are idempotent (they re-derive from the cached state) — the outcome is identical, so this is not a visible-behavior deviation (no DEVIATIONS entry).
- **KV load is synchronous:** the upstream load is async (a `ready` signal, kv.tsx:17–31); the Go port loads in `OpenKV` so `New` returns before the first `Get` — the init produce-block reads are race-free by construction.
- **KV concurrency:** one writer goroutine + a 1024-buffered flush-trigger channel is the promise-chain port (kv.tsx:57–61); the in-memory store is the source of truth (updated under the lock at Set time) and the writer marshals it under the same lock per flush; `syscall.Flock(LOCK_EX)` guards the cross-process atomic temp+rename write — POSIX, the project platform is Linux. The queue is never closed (a Set racing Close would panic on a closed channel): Close closes a separate `done` channel, the writer drains the remaining triggers and performs one final flush.
- **Import-cycle guard:** the 250/1000 ms `THEME_REFRESH_DELAYS` live in the APP (`themeRefreshDelays` in `app.go`) — the engine cannot import bubbletea (the app imports the engine, never the reverse).
- **Wire-change blast radius (verified by grep):** `rg -n '"theme"|\.Theme\b|Theme ' internal/ cmd/ --type go` plus a `"theme"` check over `internal/server/testdata/` hits exactly `internal/protocol/config.go:41` (the field), `internal/config/config_test.go:29` (fixture) and `:72` (assertion). No server handler reads `.Theme`; no golden/testdata encodes the old map shape — the re-baseline is `config_test.go` only, same commit (root principles 2+3), DEVIATIONS.md entry 123. `internal/config/config.go` needs NO code change: `cfgFromMap`/`mapFromCfg` are generic JSON marshal round-trips, and `json:"theme,omitempty"` on the string field already implements the string round-trip with empty omitted.

- [ ] **Step 1: Write the failing tests + re-baseline the wire fixture**

Create `internal/tui/theme/kv_test.go`:

```go
package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestKVGetSetAndNilDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.json")
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatalf("OpenKV: %v", err)
	}
	defer kv.Close()

	// `??` semantics: missing key → default.
	if got := kv.Get("absent", "dflt"); got != "dflt" {
		t.Errorf("Get(absent) = %v, want dflt", got)
	}
	kv.Set("theme", "kanagawa")
	if got := kv.Get("theme", "dflt"); got != "kanagawa" {
		t.Errorf("Get(theme) = %v, want kanagawa", got)
	}
	// Falsy JSON values are preserved (`??`, not `||`): false, "", 0.
	kv.Set("flag", false)
	if got := kv.Get("flag", "dflt"); got != false {
		t.Errorf("Get(flag) = %v, want false (falsy preserved)", got)
	}
	kv.Set("empty", "")
	if got := kv.Get("empty", "dflt"); got != "" {
		t.Errorf("Get(empty) = %v, want \"\" (falsy preserved)", got)
	}
	kv.Set("zero", 0)
	if got := kv.Get("zero", "dflt"); got != float64(0) {
		t.Errorf("Get(zero) = %v, want 0 (falsy preserved)", got)
	}
	// A nil value deletes the key (upstream setStore(key, undefined)).
	kv.Set("theme", nil)
	if got := kv.Get("theme", "dflt"); got != "dflt" {
		t.Errorf("Get(theme) after nil-set = %v, want dflt (nil deletes)", got)
	}
}

func TestKVMissingFileIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.json")
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatalf("OpenKV on missing file must not error: %v", err)
	}
	defer kv.Close()
	if got := kv.Get("theme", "opencode"); got != "opencode" {
		t.Errorf("Get = %v, want opencode (missing file = empty store)", got)
	}
}

func TestKVCorruptFileIsLoggedAndEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatalf("corrupt file must not error (upstream catch → continue): %v", err)
	}
	defer kv.Close()
	if got := kv.Get("theme", "opencode"); got != "opencode" {
		t.Errorf("Get = %v, want opencode (corrupt file = empty store)", got)
	}
	kv.Set("theme", "nord")
	if got := kv.Get("theme", "dflt"); got != "nord" {
		t.Errorf("Get = %v, want nord (store usable after corrupt load)", got)
	}
}

func TestKVRapidSetsFlushOrderedOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "nested", "kv.json")
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatalf("OpenKV (must MkdirAll the parent): %v", err)
	}
	for i := 0; i < 50; i++ {
		kv.Set(fmt.Sprintf("key%02d", i), i)
	}
	if err := kv.Close(); err != nil {
		t.Fatalf("Close (drain + flush): %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read KV file: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("KV file must be valid JSON: %v\n%s", err, data)
	}
	if len(m) != 50 {
		t.Fatalf("keys = %d, want 50", len(m))
	}
	for i := 0; i < 50; i++ {
		if got := m[fmt.Sprintf("key%02d", i)]; got != float64(i) {
			t.Errorf("key%02d = %v, want %d (ordered writes)", i, got, i)
		}
	}
}

func TestKVReloadPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.json")
	kv, err := OpenKV(path)
	if err != nil {
		t.Fatal(err)
	}
	kv.Set("theme", "kanagawa")
	if err := kv.Close(); err != nil {
		t.Fatal(err)
	}
	kv2, err := OpenKV(path)
	if err != nil {
		t.Fatal(err)
	}
	defer kv2.Close()
	if got := kv2.Get("theme", "dflt"); got != "kanagawa" {
		t.Errorf("reloaded Get = %v, want kanagawa", got)
	}
}
```

Create `internal/tui/theme/engine_test.go` (hermetic: `t.TempDir()` KV + injected palette funcs — no network, no real tty; `hex16` is Task S0.5's package-level test fixture):

```go
package theme

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testPalette builds a TerminalColors over S0.5's hex16 with the given
// default bg/fg ("" = unknown).
func testPalette(bg, fg string) TerminalColors {
	var p [16]string
	copy(p[:], hex16)
	return TerminalColors{Palette: p, DefaultForeground: fg, DefaultBackground: bg}
}

// paletteFunc is the injected palette probe (hermetic: no tty, no network).
func paletteFunc(c TerminalColors, ok bool) func(context.Context) (TerminalColors, bool) {
	return func(context.Context) (TerminalColors, bool) { return c, ok }
}

func seedKV(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readKV(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read KV file: %v", err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("KV file invalid: %v\n%s", err, data)
	}
	return m
}

func newTestEngine(t *testing.T, opts EngineOptions) *Engine {
	t.Helper()
	e, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func engineDir(t *testing.T) (dir, kvPath string) {
	t.Helper()
	dir = t.TempDir()
	return dir, filepath.Join(dir, "kv.json")
}

// TestEngineSelectionChain: active = ConfigTheme > KV "theme" > default
// "opencode" (theme.tsx:121-122, spec §3) across the full cfg × kv matrix.
func TestEngineSelectionChain(t *testing.T) {
	dark := paletteFunc(testPalette("#000000", "#ffffff"), true)
	cases := []struct {
		name string
		cfg  string
		kv   string // "" = key absent
		want string
	}{
		{"cfg empty, kv absent", "", "", "opencode"},
		{"cfg empty, kv valid", "", "kanagawa", "kanagawa"},
		{"cfg empty, kv unknown", "", "ghost", "ghost"},
		{"cfg valid, kv absent", "nord", "", "nord"},
		{"cfg valid, kv valid", "nord", "kanagawa", "nord"},
		{"cfg valid, kv unknown", "nord", "ghost", "nord"},
		{"cfg unknown, kv absent", "ghostcfg", "", "ghostcfg"},
		{"cfg unknown, kv valid", "ghostcfg", "kanagawa", "ghostcfg"},
		{"cfg unknown, kv unknown", "ghostcfg", "ghostkv", "ghostcfg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, kvPath := engineDir(t)
			if c.kv != "" {
				seedKV(t, kvPath, `{"theme":"`+c.kv+`"}`)
			}
			e := newTestEngine(t, EngineOptions{
				KVPath: kvPath, GlobalYoloDir: dir, CWD: dir,
				ConfigTheme: c.cfg, Palette: dark,
			})
			if err := e.Resolve(context.Background()); err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got := e.Active(); got != c.want {
				t.Errorf("Active() = %q, want %q", got, c.want)
			}
		})
	}

	// An unknown active (kv "ghost") resolves to the default theme via the
	// upstream values-memo fallback (theme.tsx:256-267).
	dir, kvPath := engineDir(t)
	seedKV(t, kvPath, `{"theme":"ghost"}`)
	e := newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := e.Active(); got != "ghost" {
		t.Fatalf("Active() = %q, want ghost", got)
	}
	th, err := e.ActiveTheme()
	if err != nil {
		t.Fatalf("ActiveTheme: %v", err)
	}
	if th.Name != "opencode" {
		t.Errorf("ActiveTheme().Name = %q, want opencode (memo fallback)", th.Name)
	}
}

// TestEngineModeChain: mode = lock > terminal luminance > "dark"
// (theme.tsx:117, 165; S0 scoping rule: the luminance half applies in
// Resolve, after the palette probe).
func TestEngineModeChain(t *testing.T) {
	// (a) No lock + light-luminance bg → "light".
	dir, kvPath := engineDir(t)
	e := newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir,
		Palette: paletteFunc(testPalette("#ffffff", "#000000"), true),
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := e.Mode(); got != "light" {
		t.Errorf("Mode() = %q, want light (terminal luminance)", got)
	}
	if e.Locked() {
		t.Error("Locked() must be false")
	}

	// (b) Lock "dark" + light bg → "dark" (the lock wins).
	dir, kvPath = engineDir(t)
	seedKV(t, kvPath, `{"theme_mode_lock":"dark"}`)
	e = newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir,
		Palette: paletteFunc(testPalette("#ffffff", "#000000"), true),
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := e.Mode(); got != "dark" {
		t.Errorf("Mode() = %q, want dark (lock wins over luminance)", got)
	}
	if !e.Locked() {
		t.Error("Locked() must be true")
	}

	// (c) Unsupported palette → fallback "dark".
	dir, kvPath = engineDir(t)
	e = newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir,
		Palette: paletteFunc(TerminalColors{}, false),
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := e.Mode(); got != "dark" {
		t.Errorf("Mode() = %q, want dark (no palette)", got)
	}
}

// TestEngineStaleThemeModeClearedWhenUnlocked: the one-shot theme_mode is
// cleared at init when unlocked (theme.tsx:118) — read back from the KV
// file; retained while locked.
func TestEngineStaleThemeModeClearedWhenUnlocked(t *testing.T) {
	dark := paletteFunc(testPalette("#000000", "#ffffff"), true)

	dir, kvPath := engineDir(t)
	seedKV(t, kvPath, `{"theme_mode":"light"}`)
	e := newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := e.Mode(); got != "dark" {
		t.Errorf("Mode() = %q, want dark (stale theme_mode is not a mode source)", got)
	}
	if e.Locked() {
		t.Error("Locked() must be false")
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if m := readKV(t, kvPath); m["theme_mode"] != nil {
		t.Errorf("theme_mode = %v, want cleared (unlocked)", m["theme_mode"])
	}

	dir, kvPath = engineDir(t)
	seedKV(t, kvPath, `{"theme_mode_lock":"dark","theme_mode":"light"}`)
	e = newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
	})
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if m := readKV(t, kvPath); m["theme_mode"] != "light" {
		t.Errorf("theme_mode = %v, want light (retained while locked)", m["theme_mode"])
	}
}

// TestEngineSystemTheme: the "system" key exists when the palette probe is
// ok + palette[0] present, and config "system" selects it; palette[0]
// empty → no "system" + active "system" falls back to "opencode" (the
// upstream catch path, theme.tsx:159-163, 174-178).
func TestEngineSystemTheme(t *testing.T) {
	dir, kvPath := engineDir(t)
	e := newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir,
		ConfigTheme: "system",
		Palette:     paletteFunc(testPalette("#000000", "#ffffff"), true),
	})
	if e.Ready() {
		t.Fatal("Ready() must be false before Resolve")
	}
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !e.Ready() {
		t.Error("Ready() must be true after Resolve")
	}
	if !e.Has("system") {
		t.Error("system theme must be present (palette ok, palette[0] set)")
	}
	if got := e.Active(); got != "system" {
		t.Errorf("Active() = %q, want system (config selects it)", got)
	}
	th, err := e.ActiveTheme()
	if err != nil {
		t.Fatalf("ActiveTheme: %v", err)
	}
	if th.Name != "system" {
		t.Errorf("theme Name = %q, want system", th.Name)
	}

	dir, kvPath = engineDir(t)
	e = newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir,
		ConfigTheme: "system",
		Palette:     paletteFunc(TerminalColors{}, true), // supported, empty palette
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if e.Has("system") {
		t.Error("system theme must be absent (palette[0] empty)")
	}
	if got := e.Active(); got != DefaultName {
		t.Errorf("Active() = %q, want %s (upstream catch path)", got, DefaultName)
	}
	th, err = e.ActiveTheme()
	if err != nil {
		t.Fatalf("ActiveTheme: %v", err)
	}
	if th.Name != DefaultName {
		t.Errorf("theme Name = %q, want %s", th.Name, DefaultName)
	}
}

// TestEngineSet: unknown name → false, KV untouched; known name → active +
// persisted (theme.tsx:293-298).
func TestEngineSet(t *testing.T) {
	dark := paletteFunc(testPalette("#000000", "#ffffff"), true)
	dir, kvPath := engineDir(t)
	e := newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if e.Set("ghost") {
		t.Error("Set(unknown) must return false")
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if m := readKV(t, kvPath); m["theme"] != nil {
		t.Errorf("KV theme = %v, want untouched after Set(unknown)", m["theme"])
	}

	e = newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
	})
	if !e.Set("nord") {
		t.Fatal("Set(known) must return true")
	}
	if got := e.Active(); got != "nord" {
		t.Errorf("Active() = %q, want nord", got)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if m := readKV(t, kvPath); m["theme"] != "nord" {
		t.Errorf("KV theme = %v, want nord", m["theme"])
	}
}

// TestEnginePinFreeApplyAndModeEvents: Pin writes theme_mode_lock; Apply
// while locked writes theme_mode; ThemeModeEvent while locked is a no-op;
// Free clears the lock + both KV keys and re-resolves the mode from the
// cached terminal luminance (theme.tsx:202-226).
func TestEnginePinFreeApplyAndModeEvents(t *testing.T) {
	dark := paletteFunc(testPalette("#000000", "#ffffff"), true)

	t.Run("pin-locked-apply-and-theme-mode-event", func(t *testing.T) {
		dir, kvPath := engineDir(t)
		e := newTestEngine(t, EngineOptions{
			KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
		})
		if err := e.Resolve(context.Background()); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		e.Pin("dark")
		if !e.Locked() {
			t.Fatal("Locked() must be true after Pin")
		}
		if got := e.Mode(); got != "dark" {
			t.Errorf("Mode() = %q, want dark (Pin applies)", got)
		}
		e.ThemeModeEvent("light")
		if got := e.Mode(); got != "dark" {
			t.Errorf("Mode() = %q, want dark (ThemeModeEvent ignored while locked)", got)
		}
		e.Apply("light")
		if got := e.Mode(); got != "light" {
			t.Errorf("Mode() = %q, want light (Apply switches mode even while locked)", got)
		}
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
		m := readKV(t, kvPath)
		if m["theme_mode_lock"] != "dark" {
			t.Errorf("KV theme_mode_lock = %v, want dark (Pin)", m["theme_mode_lock"])
		}
		if m["theme_mode"] != "light" {
			t.Errorf("KV theme_mode = %v, want light (Apply while locked)", m["theme_mode"])
		}
	})

	t.Run("free-clears-lock-and-keys", func(t *testing.T) {
		dir, kvPath := engineDir(t)
		seedKV(t, kvPath, `{"theme_mode_lock":"light","theme_mode":"light"}`)
		e := newTestEngine(t, EngineOptions{
			KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
		})
		if !e.Locked() {
			t.Fatal("Locked() must be true (seeded lock)")
		}
		// Resolve first: Free re-resolves the mode from the CACHED
		// terminal luminance (S0 scoping rule) — without a probe there
		// is no luminance and the old (locked) mode would stand.
		if err := e.Resolve(context.Background()); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got := e.Mode(); got != "light" {
			t.Fatalf("Mode() = %q, want light (the lock wins over the dark-bg luminance)", got)
		}
		e.Free()
		if e.Locked() {
			t.Error("Locked() must be false after Free")
		}
		if got := e.Mode(); got != "dark" {
			t.Errorf("Mode() = %q, want dark (Free re-resolves: dark bg luminance)", got)
		}
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
		m := readKV(t, kvPath)
		if _, ok := m["theme_mode_lock"]; ok {
			t.Errorf("KV theme_mode_lock = %v, want cleared", m["theme_mode_lock"])
		}
		if _, ok := m["theme_mode"]; ok {
			t.Errorf("KV theme_mode = %v, want cleared", m["theme_mode"])
		}
	})

	t.Run("unlocked-apply-and-theme-mode-event", func(t *testing.T) {
		dir, kvPath := engineDir(t)
		e := newTestEngine(t, EngineOptions{
			KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
		})
		if err := e.Resolve(context.Background()); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		e.Apply("light")
		if got := e.Mode(); got != "light" {
			t.Errorf("Mode() = %q, want light (Apply unlocked)", got)
		}
		e.ThemeModeEvent("dark")
		if got := e.Mode(); got != "dark" {
			t.Errorf("Mode() = %q, want dark (ThemeModeEvent unlocked)", got)
		}
	})
}

// TestEngineRefreshCustoms: the 1000 ms refresh leg — a runtime-added
// custom appears; a corrupt file is the error path: active "opencode",
// customs emptied, error returned (the theme.tsx:132-144 catch).
func TestEngineRefreshCustoms(t *testing.T) {
	dark := paletteFunc(testPalette("#000000", "#ffffff"), true)
	dir, kvPath := engineDir(t)
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTheme := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(themesDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTheme("good.json", `{"theme":{"primary":"#111111"}}`)

	e := newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir, Palette: dark,
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !e.Has("good") {
		t.Fatal("custom 'good' must be discovered at Resolve")
	}
	if !e.Set("good") {
		t.Fatal("Set(good) must succeed")
	}

	writeTheme("late.json", `{"theme":{"primary":"#222222"}}`)
	if err := e.RefreshCustoms(context.Background()); err != nil {
		t.Fatalf("RefreshCustoms: %v", err)
	}
	if !e.Has("late") {
		t.Error("custom 'late' must appear after RefreshCustoms")
	}

	writeTheme("bad.json", `{not json`)
	if err := e.RefreshCustoms(context.Background()); err == nil {
		t.Fatal("RefreshCustoms must return the discover error")
	}
	if got := e.Active(); got != DefaultName {
		t.Errorf("Active() = %q, want %s (upstream catch)", got, DefaultName)
	}
	if e.Has("good") {
		t.Error("customs must be empty after the discover error")
	}
}

// TestEngineReapply: the 250 ms refresh leg — regenerates the system
// theme from the cached palette at the current mode (no re-probe, S0
// scoping rule).
func TestEngineReapply(t *testing.T) {
	dir, kvPath := engineDir(t)
	e := newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir,
		ConfigTheme: "system",
		Palette:     paletteFunc(testPalette("#ffffff", "#000000"), true),
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := e.Mode(); got != "light" {
		t.Fatalf("Mode() = %q, want light (luminance)", got)
	}
	e.Pin("dark")
	if got := e.Mode(); got != "dark" {
		t.Fatalf("Mode() = %q, want dark (Pin)", got)
	}
	e.Reapply()
	th, err := e.ActiveTheme()
	if err != nil {
		t.Fatalf("ActiveTheme: %v", err)
	}
	if th.Name != "system" {
		t.Errorf("theme Name = %q, want system", th.Name)
	}
	if th.Mode != "dark" {
		t.Errorf("theme Mode = %q, want dark (regenerated at the current mode)", th.Mode)
	}
}
```

Re-baseline the wire fixture in `internal/config/config_test.go` (this is the failing half of the wire change — the fixture becomes the string shape while the struct field is still the old map):

- Line 29 — the yolo.jsonc fixture: replace the old-shape `"theme":{"dark":true}` value with the string shape `"theme":"opencode"` (the fixture object becomes `{"instructions":["/docs/A.md"], "theme":"opencode"}`).
- Lines 72–74 — replace the map assertion `if cfg.Theme == nil { t.Fatal("theme lost") }` with:

```go
	if cfg.Theme != "opencode" {
		t.Fatalf("theme = %q, want opencode (string shape, deviation 123)", cfg.Theme)
	}
```
- [ ] **Step 2: Run the tests — confirm the new tests FAIL**

```
cd /home/kido/network/projects/yolo && go test ./internal/tui/theme/ ./internal/config/
```

Expected: `internal/tui/theme` fails to BUILD — `undefined: OpenKV`, `undefined: EngineOptions`, `undefined: New` (kv_test.go / engine_test.go reference the absent implementation); `internal/config` fails with a type error — `cfg.Theme` is compared against the string literal `"opencode"` while the field is still `map[string]any` (the re-baselined fixture pins the wire change). This is the failing-test state (root principle 5: the test defines the contract).

- [ ] **Step 3: Minimal implementation (KV file + Engine + wire change + app wiring)**

Create `internal/tui/theme/kv.go` (port of upstream `packages/tui/src/context/kv.tsx`; the single-writer channel is the promise-chain port, the `Flock` is the `flockfile` port, the atomic temp+rename is `writeJsonAtomic`):

```go
package theme

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// KV is the TUI key-value store (port of context/kv.tsx): a flat
// JSON map[string]any persisted at path, read `??`-style, written
// ordered + atomic + cross-process locked. The engine is the only
// consumer in S0.
//
// Concurrency model: the in-memory store is the source of truth
// (updated under mu at Set time); the queue only carries flush
// triggers, so a single writer goroutine applies them in order
// (the upstream promise-chain port). The queue is NEVER closed
// (a Set racing Close would panic on a closed channel); Close
// instead closes done, the writer drains the remaining triggers,
// performs one final flush, and exits.
type KV struct {
	path string

	mu       sync.Mutex
	closed   bool
	store    map[string]any
	queue    chan struct{}
	done     chan struct{} // Close closes it; the writer selects on it
	finished chan struct{}
	once     sync.Once
}

// OpenKV opens (creating, with parent dirs) the KV file at path. A
// missing file yields an empty store; a corrupt file is logged and
// yields an empty store (upstream catch → console.error → continue);
// the only error is an unwritable parent dir.
func OpenKV(path string) (*KV, error) {
	kv := &KV{
		path:     path,
		store:    map[string]any{},
		queue:    make(chan struct{}, 1024),
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("theme: kv: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("theme: kv: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &kv.store); err != nil {
			slog.Warn("theme: kv: corrupt file, starting empty", "path", path, "error", err)
			kv.store = map[string]any{}
		}
	}
	go kv.run()
	return kv, nil
}

// Get returns store[key], or def when the key is absent or its value
// is nil — `??` semantics: JSON falsy values (false, "", 0) are
// preserved.
func (k *KV) Get(key string, def any) any {
	k.mu.Lock()
	defer k.mu.Unlock()
	if v, ok := k.store[key]; ok && v != nil {
		return v
	}
	return def
}

// Set stores val under key (a nil val deletes the key) and requests a
// flush. The store is updated under the lock; a flush trigger is then
// offered to the single writer (the upstream promise-chain port), which
// serializes + flock-locks + atomically rewrites the file. Offer is
// non-blocking: if 1024 triggers are already queued the writer is
// guaranteed to flush on its next tick, so dropping a trigger loses no
// state (the store already holds it). Set after Close is a no-op.
func (k *KV) Set(key string, val any) {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return
	}
	if val == nil {
		delete(k.store, key)
	} else {
		k.store[key] = val
	}
	select {
	case k.queue <- struct{}{}:
	default:
	}
	k.mu.Unlock()
}

// Close marks the store closed, stops the writer, and is idempotent.
// Because the queue is never closed, an in-flight Set cannot panic on a
// closed channel: the writer exits via done, and its final drain+flush
// captures every Set that offered before Close returned.
func (k *KV) Close() error {
	k.once.Do(func() {
		k.mu.Lock()
		k.closed = true
		k.mu.Unlock()
		close(k.done)
		<-k.finished
	})
	return nil
}

// run is the single writer goroutine (the upstream promise-chain
// port): each queued trigger flushes the whole store, ordered. On
// done it drains any remaining triggers and performs a final flush so
// nothing offered before Close is lost.
func (k *KV) run() {
	defer close(k.finished)
	flush := func() {
		k.mu.Lock()
		defer k.mu.Unlock()
		k.writeLocked()
	}
	for {
		select {
		case <-k.queue:
			flush()
		case <-k.done:
			for {
				select {
				case <-k.queue:
				default:
					flush()
					return
				}
			}
		}
	}
}

// writeLocked marshals + atomically rewrites the store; the caller
// holds k.mu.
func (k *KV) writeLocked() {
	data, err := json.Marshal(k.store)
	if err != nil {
		slog.Warn("theme: kv: marshal failed, write dropped", "error", err)
		return
	}
	if err := k.writeAtomic(data); err != nil {
		slog.Warn("theme: kv: write failed", "path", k.path, "error", err)
	}
}

// writeAtomic writes data to path via temp-file + rename, holding an
// exclusive flock on the target file for the whole write (upstream
// flockfile, kv.tsx:96-111; POSIX — Linux platform scope).
func (k *KV) writeAtomic(data []byte) error {
	dir := filepath.Dir(k.path)
	tmp, err := os.CreateTemp(dir, ".kv-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	lockF, err := os.OpenFile(k.path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer lockF.Close()
	if err := syscall.Flock(int(lockF.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(lockF.Fd()), syscall.LOCK_UN) }()
	if err := os.Rename(tmpName, k.path); err != nil {
		return err
	}
	return nil
}
```

Create `internal/tui/theme/engine.go` (port of the upstream `context/theme.tsx` store — init produce-block 114–125, onMount 146–150, resolveSystemTheme 152–179, apply/pin/free 202–220, THEME_MODE 222–226, the values memo 256–267, `set` 293–298):

```go
package theme

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
)

// EngineOptions is the Engine's startup config (the port of the
// upstream createSignal defaults + the createEffect config read + the
// onMount discovery + the palette probe, flattened for Go: the palette
// is injected, the config is a startup snapshot).
type EngineOptions struct {
	// KVPath is the KV file (e.g. <data>/tui/kv.json).
	KVPath string
	// GlobalYoloDir is the global yolo config root (~/.config/yolo);
	// Discover scans its themes/ subdir (S0.6).
	GlobalYoloDir string
	// CWD is the project root (the .yolo/themes walk, S0.6).
	CWD string
	// ConfigTheme is the config `theme` string (the top of the chain;
	// empty = unset). Startup snapshot only — the upstream live
	// config effect has no TUI hot-reload counterpart in S0.
	ConfigTheme string
	// Palette probes the terminal palette (S0.5's theme.DetectStd in
	// prod; tests inject a fixed TerminalColors). Called EXACTLY ONCE
	// — in Resolve (S0 scoping rule).
	Palette func(ctx context.Context) (TerminalColors, bool)
}

// Engine is the theme selection store (port of context/theme.tsx):
// active = config > KV "theme" > default "opencode";
// mode = KV lock > terminal luminance > "dark".
type Engine struct {
	opts EngineOptions
	kv   *KV

	mu sync.Mutex

	// customs is the raw custom-theme value map (Discover output,
	// S0.6).
	customs map[string]any
	// system holds the generated system theme (absent = no usable
	// palette).
	system   ThemeJson
	hasSys   bool
	mode     string
	lock     string
	active   string
	ready    bool
	colors   TerminalColors
	hasColor bool
}

// New initializes the Engine exactly per the upstream init produce
// block (theme.tsx:114-125): lock = pick(theme_mode_lock);
// mode = lock ?? "dark" (the upstream `pick(renderer.themeMode) ??
// props.mode` half has no init-time counterpart — the palette is
// probed in Resolve, S0 scoping rule); the one-shot theme_mode is
// cleared when unlocked (only when it holds a valid mode,
// theme.tsx:118); active = config > KV "theme" > "opencode". The KV
// is loaded synchronously so the first Get is race-free.
func New(opts EngineOptions) (*Engine, error) {
	kv, err := OpenKV(opts.KVPath)
	if err != nil {
		return nil, err
	}
	e := &Engine{opts: opts, kv: kv}
	lock, _ := e.kv.Get("theme_mode_lock", nil).(string)
	if pickMode(lock) != "" {
		e.lock = lock
		e.mode = lock
	} else {
		e.mode = "dark"
		if v, ok := e.kv.Get("theme_mode", nil).(string); ok && pickMode(v) != "" {
			e.kv.Set("theme_mode", nil) // stale one-shot (upstream :118)
		}
	}
	active := opts.ConfigTheme
	if active == "" {
		if v, ok := e.kv.Get("theme", nil).(string); ok && v != "" {
			active = v
		}
	}
	if active == "" {
		active = DefaultName
	}
	e.active = active
	return e, nil
}

// pickMode returns mode when it is a valid theme mode ("dark" or
// "light"), else "" (the upstream typeof-guarded pick, theme.tsx:116,
// 164, 202, 223).
func pickMode(v string) string {
	if v == "dark" || v == "light" {
		return v
	}
	return ""
}

// Resolve runs the startup sequence (the upstream onMount:
// resolveSystemTheme + syncCustomThemes, ported sequentially; the
// upstream Promise.allSettled keeps both independent — the Go port
// is sequential, same observable state): system theme first — the
// palette is probed exactly here, exactly once (S0 scoping rule) —
// then custom discovery. It ALWAYS returns nil: both upstream catch
// paths are swallowed (log + fallback).
func (e *Engine) Resolve(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.colors, e.hasColor = e.opts.Palette(ctx)
	e.hasSys = false
	if e.hasColor {
		if e.colors.Palette[0] != "" {
			// Mode re-resolution (theme.tsx:164-166):
			// next = lock ?? terminalMode(colors) ?? mode.
			if e.lock == "" {
				if m := TerminalMode(e.colors.DefaultBackground); m != "" {
					e.mode = m
				}
			}
			e.system = GenerateSystem(e.colors, e.mode)
			e.hasSys = true
		} else if e.active == "system" {
			// The empty-palette[0] path (theme.tsx:157-161): no
			// system theme; an active "system" falls back to the
			// default.
			e.active = DefaultName
		}
	} else if e.active == "system" {
		// The catch path (theme.tsx:174-178): palette probe failed.
		e.active = DefaultName
	}

	customs, err := Discover(ThemeDirs(e.opts.GlobalYoloDir, e.opts.CWD))
	if err != nil {
		slog.Warn("theme: discovery failed at startup", "error", err)
		customs = map[string]any{}
	}
	e.customs = customs

	e.ready = true
	return nil
}

// themesFromRaw filters the raw value map to theme objects (upstream
// Object.entries(...).filter isTheme — theme.tsx:137-140) and
// round-trips each to a ThemeJson.
func themesFromRaw(raw map[string]any) map[string]ThemeJson {
	out := map[string]ThemeJson{}
	for name, v := range raw {
		if !IsTheme(v) {
			continue
		}
		b, err := json.Marshal(v)
		if err != nil {
			continue
		}
		var tj ThemeJson
		if err := json.Unmarshal(b, &tj); err != nil {
			continue
		}
		out[name] = tj
	}
	return out
}

// themesMap is the builtins + customs + (optional) "system" map (the
// priority defaults < custom < system: later entries win the values
// memo's map lookup, theme.tsx:256-267). The upstream
// systemThemeSignature/systemThemeMode skip (theme.tsx:167-171) is
// not ported: the Go port regenerates on every refresh leg — the
// regeneration is idempotent and the legs are at most every second.
func (e *Engine) themesMap() map[string]ThemeJson {
	out := map[string]ThemeJson{}
	base, err := AllThemes()
	if err != nil {
		slog.Warn("theme: builtins failed", "error", err)
	}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range themesFromRaw(e.customs) {
		out[k] = v
	}
	if e.hasSys {
		out["system"] = e.system
	}
	return out
}

// resolveThemeJson resolves one ThemeJson at the current mode and
// tags it with the selected name + mode (the Theme shape S0.8–S0.10
// consume).
func (e *Engine) resolveThemeJson(name string, tj ThemeJson) (Theme, error) {
	r, err := ResolveTheme(tj, e.mode)
	if err != nil {
		return Theme{}, err
	}
	return Theme{R: r, Name: name, Mode: e.mode}, nil
}

// ActiveTheme is the values memo (theme.tsx:256-267):
// themes[active] ?? themes[KV "theme"] ?? themes[opencode].
func (e *Engine) ActiveTheme() (Theme, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	themes := e.themesMap()
	if tj, ok := themes[e.active]; ok {
		return e.resolveThemeJson(e.active, tj)
	}
	if saved, ok := e.kv.Get("theme", nil).(string); ok {
		if tj, ok := themes[saved]; ok {
			return e.resolveThemeJson(saved, tj)
		}
	}
	tj, ok := themes[DefaultName]
	if !ok {
		base, err := AllThemes()
		if err != nil {
			return Theme{}, err
		}
		tj = base[DefaultName]
	}
	return e.resolveThemeJson(DefaultName, tj)
}

// Active is the active theme name.
func (e *Engine) Active() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active
}

// Mode is the current mode ("dark" or "light").
func (e *Engine) Mode() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mode
}

// Locked reports whether a mode lock is active.
func (e *Engine) Locked() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lock != ""
}

// Ready reports whether Resolve has completed.
func (e *Engine) Ready() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ready
}

// AllThemes is the name → ThemeJson map (builtins + customs +
// "system" when present) — the upstream `all` export
// (theme.tsx:285).
func (e *Engine) AllThemes() map[string]ThemeJson {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.themesMap()
}

// Has reports whether name is a known theme (builtin, custom, or
// "system") — the upstream hasTheme (theme.tsx:290-292).
func (e *Engine) Has(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.themesMap()[name]
	return ok
}

// Set switches the active theme and persists it to the KV "theme"
// (upstream `set`, theme.tsx:293-298). Unknown names return false
// and leave the KV untouched.
func (e *Engine) Set(name string) bool {
	e.mu.Lock()
	if _, ok := e.themesMap()[name]; !ok {
		e.mu.Unlock()
		return false
	}
	e.active = name
	e.mu.Unlock()
	e.kv.Set("theme", name)
	return true
}

// Pin sets the mode lock and applies the mode (upstream pin,
// theme.tsx:213-217).
func (e *Engine) Pin(mode string) {
	e.mu.Lock()
	e.lock = mode
	e.mu.Unlock()
	e.kv.Set("theme_mode_lock", mode)
	e.Apply(mode)
}

// Free clears the lock + the one-shot KV keys and re-resolves the
// mode (upstream free, theme.tsx:219: refresh at
// renderer.themeMode ?? store.mode — the Go port uses the CACHED
// terminal luminance, S0 scoping rule: no re-probe).
func (e *Engine) Free() {
	e.mu.Lock()
	e.lock = ""
	e.mu.Unlock()
	e.kv.Set("theme_mode_lock", nil)
	e.kv.Set("theme_mode", nil)
	e.mu.Lock()
	defer e.mu.Unlock()
	if m := TerminalMode(e.colors.DefaultBackground); m != "" {
		e.mode = m
	}
	if e.hasSys {
		e.system = GenerateSystem(e.colors, e.mode)
	}
}

// Apply switches the mode and persists it to the KV "theme_mode"
// ONLY while locked (upstream apply, theme.tsx:202-211: the KV write
// precedes the early-return, the refresh follows the switch). When a
// system theme exists it is regenerated at the new mode from the
// CACHED palette (no re-probe — S0 scoping rule).
func (e *Engine) Apply(mode string) {
	e.mu.Lock()
	locked := e.lock != ""
	if e.mode != mode {
		e.mode = mode
		if e.hasSys {
			e.system = GenerateSystem(e.colors, mode)
		}
	}
	e.mu.Unlock()
	if locked {
		e.kv.Set("theme_mode", mode)
	}
}

// ThemeModeEvent is the port of the THEME_MODE handler
// (theme.tsx:222-226): ignored while locked, else Apply. No S0
// caller (bubbletea has no opentui CliRenderEvents source yet).
func (e *Engine) ThemeModeEvent(mode string) {
	e.mu.Lock()
	locked := e.lock != ""
	e.mu.Unlock()
	if locked {
		return
	}
	e.Apply(mode)
}

// Reapply is the 250 ms refresh leg (upstream refreshSystemTheme
// theme.tsx:181-200, minus the palette cache clear + re-probe — S0
// scoping rule): regenerates the system theme at the current mode
// from the cached palette.
func (e *Engine) Reapply() {
	e.mu.Lock()
	if e.hasSys {
		e.system = GenerateSystem(e.colors, e.mode)
	}
	e.mu.Unlock()
}

// RefreshCustoms is the 1000 ms refresh leg (upstream
// syncCustomThemes, theme.tsx:132-144). On error it returns the error
// and takes the upstream catch path: active "opencode", customs
// emptied (the upstream catch touches only `active`; the Go port also
// empties customs so a later successful refresh re-discovers from
// scratch — the custom set is derived state, never persisted).
func (e *Engine) RefreshCustoms(ctx context.Context) error {
	customs, err := Discover(ThemeDirs(e.opts.GlobalYoloDir, e.opts.CWD))
	e.mu.Lock()
	if err != nil {
		e.customs = map[string]any{}
		e.active = DefaultName
		e.mu.Unlock()
		return err
	}
	e.customs = customs
	e.mu.Unlock()
	return nil
}

// Close flushes the KV (pending writes drain + writer stops).
func (e *Engine) Close() error {
	return e.kv.Close()
}
```

Modify `internal/protocol/config.go` — the ONE sanctioned wire change (the upstream `theme` config field is a legacy object toggle `{"dark": true}`; the selection chain spec §3 needs the theme NAME string — DEVIATIONS.md entry 123). Replace the struct field (line 41):

```go
	Theme        string                    `json:"theme,omitempty"`
```

(no other change in the file — `ParsePerms` is untouched; `gofmt` re-aligns the struct block)

Re-baseline `internal/config/config_test.go` (same commit — root principle 3):

- Line 29 — the yolo.jsonc fixture: replace `"theme":{"dark":true}` with the string shape; the fixture line becomes:

```go
	write(t, filepath.Join(global, "yolo.jsonc"), `// comment
{"instructions":["/docs/A.md"], "theme":"opencode"}`)
```

- Lines 72–74 — replace the map assertion with:

```go
	if cfg.Theme != "opencode" {
		t.Fatalf("theme = %q, want opencode (string shape, deviation 123)", cfg.Theme)
	}
```

Modify `internal/tui/app.go`:

1. Imports — add `"time"` (stdlib block) and `"github.com/kido5217/yolo/internal/tui/theme"` (module block, after the `store` import):

```go
import (
	"context"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)
```

2. `App` struct — the full struct after the change (the engine + resolved theme land between `lastErr` and `spinIdx`; the comment breaks the gofmt alignment run, so the two field blocks align independently):

```go
type App struct {
	*client.Service
	store        store.State
	route        route
	curSessionID string
	home         homeModel
	sess         sessionModel
	prompt       promptModel
	dlg          dialogStack
	toasts       []toast
	toastSeq     int
	toastCmds    []tea.Cmd
	lastErr      string
	// theme engine (S0.7): nil = unthemed run (the zero Theme paints
	// nothing — S0.8+ views read a.theme, never hex)
	engine  *theme.Engine
	theme   theme.Theme
	spinIdx int // footer spinner frame
	// tea plumbing
	size      tea.WindowSizeMsg
	eventCh   chan protocol.Event
	resyncCh  chan struct{} // SSE drop pings from the client
	resyncing bool          // a transient SSE drop's re-hydrate is in flight
	stop      context.CancelFunc
	emitSink  func(cmds ...tea.Cmd) // test seam, set from _test.go only
}
```

3. `NewApp` — signature gains the engine; the initial theme is derived before the first render:

```go
// NewApp builds the root model. A non-empty startSessionID starts on that
// session (resume); empty starts at home. A nil engine runs without the
// theme engine (the zero Theme paints nothing). The prompt is always
// focused with a static (non-blinking) cursor.
func NewApp(c *client.Service, s store.State, startSessionID string, engine *theme.Engine) *App {
	ctx, cancel := context.WithCancel(context.Background())
	eventCh, resyncCh := c.Events(ctx)
	a := &App{
		Service:  c,
		store:    s,
		route:    routeHome,
		home:     homeModel{now: nowMillis},
		sess:     newSessionModel(80, 21),
		size:     tea.WindowSizeMsg{Width: 80, Height: 24},
		eventCh:  eventCh,
		resyncCh: resyncCh,
		stop:     cancel,
		engine:   engine,
	}
	in := textinput.New()
	// textinput's View is prompt(2) + width + cursor(1): size the value
	// area so the whole line fits the 80-column default terminal.
	in.SetWidth(77)
	st := in.Styles()
	st.Cursor.Blink = false
	in.SetStyles(st)
	in.Focus()
	a.prompt.input = in
	if startSessionID != "" {
		a.route = routeSession
		a.curSessionID = startSessionID
	}
	a.retheme()
	return a
}
```

4. New message types + debounce (place after the `EventMsg` declaration):

```go
// ThemeRefreshMsg re-arms the theme refresh debounce (the port of
// upstream themes.subscribeRefresh → refresh, theme.tsx:235-244);
// cmd/yolo sends it to the running program on every theme signal
// (SIGUSR2 via theme.WatchThemeSignals, S0.6).
type ThemeRefreshMsg struct{}

type themeReapplyMsg struct{} // 250 ms leg: regenerate the system theme
type themeCustomsMsg struct{}  // 1000 ms leg: system theme + customs re-discovery

// themeRefreshDelays mirrors upstream THEME_REFRESH_DELAYS
// (theme.tsx:82): the 250 ms leg re-generates the system theme; the
// 1000 ms leg (the last) also re-discovers customs.
var themeRefreshDelays = [2]time.Duration{250 * time.Millisecond, time.Second}
```

5. `updateMsg` — add the three cases (before the closing `return nil` of the switch):

```go
	case ThemeRefreshMsg:
		if a.engine == nil {
			return nil
		}
		return a.themeRefresh()
	case themeReapplyMsg:
		if a.engine != nil {
			a.engine.Reapply()
			a.retheme()
		}
		return nil
	case themeCustomsMsg:
		if a.engine != nil {
			// Upstream leg order (theme.tsx:239-243): refreshSystemTheme
			// FIRST, then syncCustomThemes on the last delay.
			a.engine.Reapply()
			_ = a.engine.RefreshCustoms(context.Background())
			a.retheme()
		}
		return nil
```

6. The debounce + re-derivation (place after `updateMsg`):

```go
// themeRefresh arms the two refresh legs (upstream refresh,
// theme.tsx:235-244). A re-signal re-arms a second pair — bubbletea
// v2 has no tick cancellation; the legs are idempotent (they
// re-derive from the engine's cached state), so the outcome is
// unchanged.
func (a *App) themeRefresh() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(themeRefreshDelays))
	for i, d := range themeRefreshDelays {
		// Go ≥ 1.22: i and d are per-iteration (module requires 1.25).
		cmds = append(cmds, tea.Tick(d, func(time.Time) tea.Msg {
			if i == len(themeRefreshDelays)-1 {
				return themeCustomsMsg{}
			}
			return themeReapplyMsg{}
		}))
	}
	return tea.Batch(cmds...)
}

// retheme refreshes a.theme from the engine (the port of the upstream
// values() memo read, theme.tsx:256-267). With no engine, a.theme
// stays the zero Theme.
func (a *App) retheme() {
	if a.engine == nil {
		return
	}
	if th, err := a.engine.ActiveTheme(); err == nil {
		a.theme = th
	}
}
```

Modify `internal/tui/rec_test.go` — `newRecApp` passes the nil engine (the `NewApp` signature change):

```go
func newRecApp(c *client.Service, s store.State, startSessionID string) *recApp {
	ra := &recApp{App: NewApp(c, s, startSessionID, nil)}
	ra.emitSink = func(cmds ...tea.Cmd) { ra.Cmds = append(ra.Cmds, cmds...) }
	return ra
}
```

Modify `internal/tui/app_test.go` — the six `NewApp` call sites (lines 36, 69, 113, 162, 222, 254) gain the nil engine: `tui.NewApp(c, store.State{}, <sessionID>, nil)` (every existing call passes `""` or `ses.ID` unchanged).

Modify `cmd/yolo/main.go` — the import block gains `"github.com/kido5217/yolo/internal/tui/theme"` (after the `store` import), and `tuiCmd` builds + resolves the engine, passes it to `NewApp`, and arms the SIGUSR2 watcher. Replace the block from `app := tui.NewApp(...)` to the `program.Run()` line:

```go
	// Theme engine (S0.7): the config > KV > default selection chain
	// over the TUI-local KV file. The config is loaded via the same
	// profile-pinned loader buildDeps used (buildDeps consumes its
	// config internally and does not return it).
	globalDir, err := config.GlobalYoloDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return 1
	}
	loader := config.Loader{Profile: deps.Dirs.Profile}
	cfg, err := loader.LoadAt(filepath.Join(globalDir, deps.Dirs.Profile), wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return 1
	}
	engine, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(deps.Dirs.Data, "tui", "kv.json"),
		GlobalYoloDir: globalDir,
		CWD:           wd,
		ConfigTheme:   cfg.Theme,
		Palette:       theme.DetectStd,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "yolo: %v\n", err)
		drain(deps, srv)
		return 1
	}
	defer engine.Close()
	if err := engine.Resolve(context.Background()); err != nil {
		deps.Log.Error("theme resolve failed", "error", err)
	}

	app := tui.NewApp(cl, store.State{}, sessionID, engine)
	deps.Log.Info("tui start", "workdir", wd)
	program := tea.NewProgram(app)
	// The theme watcher (S0.6) sends ThemeRefreshMsg into the running
	// program; armed just before Run (a SIGUSR2 in the arm→Run gap
	// reaches the program at its first flush — the first refresh leg
	// runs one tick late at worst).
	stopTheme := theme.WatchThemeSignals(func() {
		program.Send(tui.ThemeRefreshMsg{})
	})
	defer stopTheme()
	_, runErr := program.Run()
```

(the following `runErr` handling + `app.Close()` + `drain` are unchanged; `main.go` already imports `config`, `filepath`, `context`)

Append entries 123 and 124 to `docs/superpowers/DEVIATIONS.md` (after entry 122):

```
123. config.theme wire field type change (wire/low, 2026-08-25): spec §3's selection chain (config > KV > default, upstream theme.tsx:121-122) requires the config `theme` to be a theme NAME string; the ported wire had `protocol.Config.Theme map[string]any` — a verbatim mirror of the upstream opencode config, whose `theme` field is a legacy object toggle (e.g. `{"dark": true}`), never read by the upstream TUI's selection chain. Change: `Theme string` (`json:"theme,omitempty"`), the ONE sanctioned wire deviation for S0 (plan 2026-08-24-opencode-tui-parity, Task S0.7). Re-baselined in the same commit (root principle 3): `internal/config/config_test.go` (fixture `"theme":"opencode"` + string assertion). Blast radius (grep-verified before the change): no server handler reads `.Theme`; no `internal/server/testdata/` golden encodes the old map shape; `internal/config` code is a generic JSON round-trip and needs no change. Spec §10 "no new endpoints" unaffected (field type only). Root principles 2+3: explicit user-sanctioned deviation, logged here.

```
124. Theme palette probed exactly once at startup (behavior/low, 2026-08-25): upstream re-probes the terminal palette on refresh (theme.tsx:181-200 clears the palette cache and re-issues the OSC queries through the opentui renderer's input pipeline); yolo's bubbletea program owns the tty while running, so a mid-session raw-mode re-probe is not possible without pausing the program. The engine (`theme.Engine`) probes once in `Resolve` and re-resolves on the CACHED palette + fresh customs discovery for all later events (Apply/Free/Reapply/RefreshCustoms/SIGUSR2). Upstream parity check (S8) may revisit.
```
```

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/theme/ -v` — Expected: PASS (the 5 new KV tests +
the 8 new engine tests + all S0.1–S0.6 tests).
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green — including the re-baselined `internal/config` suite, the
`internal/tui` teatest suites (nil-engine `NewApp` call sites), and
`TestImportsDirection` (app.go's new `internal/tui/theme` import is
`internal/tui/*` — pure-client-legal). gofmt prints nothing.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/kv.go internal/tui/theme/engine.go internal/tui/theme/kv_test.go internal/tui/theme/engine_test.go internal/protocol/config.go internal/config/config_test.go internal/tui/app.go internal/tui/rec_test.go internal/tui/app_test.go cmd/yolo/main.go docs/superpowers/DEVIATIONS.md
git commit -m "feat: theme selection chain (config > KV > default) + TUI KV"
bd close yolo-oae.1.7 --reason "KV file (ordered/atomic/flock, ??-get, nil-delete) + Engine (config>KV>default chain, mode lock, single-probe system theme, 250/1000ms refresh) + config.theme wire change (deviation 123) + single-probe scoping (deviation 124) + app wiring; hermetic tests green" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.8: Shell restyle — logo + borders (+ teatest SGR goldens) (`yolo-oae.1.8`)

**Files:**
- Create: `internal/tui/logo.go` — the 8 upstream logo lines (sha256-pinned) + `renderLogo`/`logoLine` (port of `logo.tsx`)
- Create: `internal/tui/logo_test.go` — `TestLogoBlockPinned` (sha256, root principle 3) + `TestRenderLogoZeroThemeIsPlain`
- Create: `internal/tui/home_theme_test.go` — `TestHomeLogoThemeSGR` (teatest SGR goldens, real engine)
- Modify: `internal/tui/home.go` — `render(s, w, th theme.Theme)`; the logo replaces the title + top divider; the bottom divider → `th.BorderSubtle()`
- Modify: `internal/tui/view.go` — the home call site passes `a.theme`
- Modify: `internal/tui/style.go` — ownership comment for the surviving statics (home no longer consumes `title`/`divider`)
- Modify: `internal/tui/home_test.go` — `TestHomeRenderLockedLayout` re-baseline (new layout + the theme arg)
- Modify: `internal/tui/overflow_test.go` — `TestHomeRenderWraps` renders at `logoWidth+1` (the logo is a fixed 39-col block)
- Modify: `internal/tui/app_test.go` — the two `"Yolo"` wait tokens → `"New session"` (lines 41, 259)
- Modify: `internal/tui/tui_suite_test.go` — the four `"Yolo"` wait/capture tokens → `"New session"` (lines 62, 175, 247, 292)
- Modify: `internal/tui/AGENTS.md` — DOX pass: `logo` in the concern-file list + the `yolo-ukc` wrap carve-out

**Interfaces:**
- Consumes:
  - Task S0.3: `theme.Theme`; `th.Color(name string) (Rgba, bool)` (the raw-token hook — the logo needs the Rgba for `Tint`, not a lipgloss.Style); `th.BorderSubtle() lipgloss.Style`. Zero-Theme reads are safe (nil-map read → no style, no panic).
  - Task S0.2: `Rgba` (`Hex()` = `#rrggbbaa` — the lipgloss color takes `[:7]`).
  - Task S0.4: `Tint(base, overlay Rgba, alpha float64) Rgba`.
  - Task S0.7: `App.theme theme.Theme`; `NewApp(c, s, startSessionID, engine *theme.Engine)`; `theme.New(EngineOptions)`, `(*Engine).Resolve(ctx)`, `(*Engine).Active() string`, `(*Engine).Close()`.
  - Upstream (port source, read at execution time): `packages/tui/src/component/logo.tsx` (`renderLine` 9–47, `Logo` 49–60) + `packages/tui/src/logo.ts` (the 8 lines, marks `"_^~,"`).
  - lipgloss v2: `lipgloss.NewStyle()`, `.Foreground(lipgloss.Color)`, `.Background`, `.Bold`, `.Render`.
- Produces (for S0.9/S0.10):
  - `renderLogo(th theme.Theme) string` — the 4-line block: left lines `textMuted` non-bold, right lines `text` bold, one unstyled gap column; zero/missing tokens → the plain translated glyphs (never a panic).
  - `logoWidth = 39` — the fixed block width (19 + 1 + 19); the logo never wraps or shrinks.
  - `homeModel.render(s *store.State, w int, th theme.Theme) string` — signature change (the theme arg; the home frame = logo + rows + `th.BorderSubtle()` divider + dim help line).

**Upstream parity notes (binding):**
- **Strict copy:** the 8 logo lines are a verbatim port of `logo.ts` (left 4 + right 4, each 19 columns), sha256-pinned in the same commit (root principle 3: the pin records the current intended content, not an upstream lock).
- **Per-cell port** (`renderLine`, logo.tsx:9–47): the marks are ALWAYS translated — `'_'` → `" "`, `'^'` → `"▀"`, `'~'` → `"▀"`, `','` → `"▄"`; the paint: `'_'`/`'^'` cells get fg + bg(shadow), `'~'`/`','` cells get fg(shadow) only, all others fg only; `shadow = Tint(background, fg, 0.25)` (logo.tsx:10). Left lines fg = `textMuted` non-bold, right lines fg = `text` bold (logo.tsx:49–60). Consecutive same-class cells emit as ONE styled run.
- **No wrap/shrink:** the block is fixed 39 columns; the 28-wide divider is deliberately narrower (the upstream look — upstream home has no divider under the logo at all; the divider is yolo's). Upstream centers the logo (`alignItems="center"`); yolo home is left-aligned with no centering machinery — the logo renders from column 0 and terminals under 39 columns clip the block in the alt-screen frame.
- **Zero-Theme degradation** (the S0.7 nil-engine contract): every `Color` read misses → the plain translated glyphs, no SGR; the divider renders plain `─`×28. No `retheme()`/view guard is needed.
- **style.go ownership after this task:** home no longer consumes `title`/`divider`; they SURVIVE for the non-home surfaces that use them — `view.go:101` (session title), `dialog.go:90-92,471,735` (`dividerLineRendered`, the quit/help/model/agents dialogs), `session.go:109` (session divider) — restyled by S0.10 (session chrome) / S3 (dialog restyles). Remaining static ownership: `dim`/footer → S0.9; `cursor`, `errRed`, `okGreen`, `toolRow` → S0.10 (the home cursor row keeps the static `cursor` until then).
- **No DEVIATIONS entry:** the look is plan-prescribed; the centering difference lives in yolo's own home layout (not a port of upstream home — upstream home has no session list), documented in `internal/tui/AGENTS.md`.

- [ ] **Step 1: Write the failing tests (+ re-baseline the old-layout tests)**

Create `internal/tui/logo_test.go`:

```go
package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// wantLogoBlockSHA256 pins the 8 logo lines in logo.go (root principle 3:
// the pin records the current intended content; an intentional change
// re-baselines the pin in the same commit). Canonical form: logoLeft[0..3]
// then logoRight[0..3], each line followed by "\n".
const wantLogoBlockSHA256 = "3132b81006fa6290b45fef72e39cd451c7d06c90aa333109e9a88eac2c79e2ee"

func logoBlockText() string {
	var b strings.Builder
	for _, l := range logoLeft {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	for _, l := range logoRight {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestLogoBlockPinned(t *testing.T) {
	sum := sha256.Sum256([]byte(logoBlockText()))
	if got := hex.EncodeToString(sum[:]); got != wantLogoBlockSHA256 {
		t.Fatalf("logo block sha256 = %s, want %s — re-baseline the pin in the same commit", got, wantLogoBlockSHA256)
	}
}

// logoPlainLines are the 4 combined (left + gap + right) plain lines —
// the zero-Theme render; TestHomeRenderLockedLayout composes the layout
// over them.
func logoPlainLines() []string {
	var zero theme.Theme
	return strings.Split(renderLogo(zero), "\n")
}

// TestRenderLogoZeroThemeIsPlain pins the mark translation with no theme
// (nil-engine runs, S0.7): the plain translated glyphs, no SGR, never a
// panic.
func TestRenderLogoZeroThemeIsPlain(t *testing.T) {
	var zero theme.Theme
	got := renderLogo(zero)
	want := strings.Join([]string{
		"                   " + " " + "             ▄     ",
		"█▀▀█ █▀▀█ █▀▀█ █▀▀▄" + " " + "█▀▀▀ █▀▀█ █▀▀█ █▀▀█",
		"█  █ █  █ █▀▀▀ █  █" + " " + "█   █  █ █  █ █▀▀▀",
		"▀▀▀▀ █▀▀▀ ▀▀▀▀ ▀▀▀▀" + " " + "▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀",
	}, "\n")
	if got != want {
		t.Fatalf("zero-theme logo = %q, want the plain translated glyphs:\n%q", got, want)
	}
}
```

Create `internal/tui/home_theme_test.go`:

```go
package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// logoSGRTokens are the logo + divider SGR color parameters under the
// pinned TTY_FORCE=1 + TERM=xterm-256color env (ANSI256 profile: the
// 24-bit hex tokens quantize onto the xterm-256 gray ramp 232–255).
// Derived from the opencode dark-mode tokens (the S0.2 goldens):
//
//	textMuted      #808080 -> 244  (128 = (244-232)*10+8, exact)
//	text           #eeeeee -> 255  (238 = (255-232)*10+8, exact)
//	Tint(#0a0a0a, #808080, .25) = #282828 -> 235 (40: |38-40|=2 < |48-40|=8)
//	Tint(#0a0a0a, #eeeeee, .25) = #434343 -> 238 (67: |68-67|=1)
//	borderSubtle   #3c3c3c  -> 237  (60: |58-60|=2 < |68-60|=8)
//
// Substring assertions (no escape/terminator boundaries): the renderer's
// pen-diff merges the changed params into ONE CSI whose inner param order
// is not pinned (the redSGR precedent).
var logoSGRTokens = []string{
	"38;5;244", // left lines fg (textMuted)
	"38;5;255", // right lines fg (text)
	"48;5;235", // hollow marks bg (the shadow, left)
	"48;5;238", // hollow marks bg (the shadow, right)
	"38;5;237", // bottom divider (borderSubtle)
}

// logoBoldRe: the right block is bold (logo.tsx:49–60) — the fg-255 CSI
// must carry the bold attribute; the param order within the CSI is not
// pinned, so match both orders.
var logoBoldRe = regexp.MustCompile(`\x1b\[(?:1;38;5;255|38;5;255;1)m`)

// TestHomeLogoThemeSGR is the teatest SGR golden: boot the app with a
// REAL theme engine (the S0.7 wiring — the same theme the app uses), let
// it render home, and pin the logo + divider SGR color parameters.
func TestHomeLogoThemeSGR(t *testing.T) {
	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}
	if got := e.Active(); got != "opencode" {
		t.Fatalf("active theme = %s, want opencode (no config, no KV)", got)
	}

	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		// The fake terminal is not a TTY, so lipgloss strips every style.
		// Pin the env that derives ANSI256 from TERM alone (suite
		// convention).
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	// ONE merged condition (consecutive WaitFors drain each other): the
	// logo plain text (left line 2, the stable box-drawing marker),
	// every logo/divider SGR token, and the right block's bold flag.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		if !strings.Contains(stripANSI(string(b)), logoLeft[1]) {
			return false
		}
		for _, tok := range logoSGRTokens {
			if !bytes.Contains(b, []byte(tok)) {
				return false
			}
		}
		return logoBoldRe.Match(b)
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

Modify `internal/tui/home_test.go` — `TestHomeRenderLockedLayout` re-baselines to the new layout (the 4 logo lines replace the `"Yolo"` title + top divider; the render call gains the theme arg — the nil-engine test app passes its zero `a.theme`, so the frame is the plain logo + plain divider):

```go
	div := strings.Repeat("─", 28)
	want := strings.Join(append(logoPlainLines(),
		"  ▸ New session",
		"  T1 · kido/q · 2m",
		"  T2 · opencode/gpt-5-nano · 3h",
		"  old · kido/q · 4d",
		div,
		"↑/↓ move · enter open · n new · /help",
	), "\n")
	got := stripANSI(a.home.render(&a.store, 80, a.theme))
```

Modify `internal/tui/overflow_test.go` — `TestHomeRenderWraps` renders at `logoWidth+1` (the logo is a fixed 39-col glyph block that never wraps or shrinks — rendering at 30 would fail `fitsWidth` by construction; the long session title still exercises the wrap at the content width):

```go
	// w >= logoWidth: the logo is a fixed 39-column glyph block that
	// never wraps or shrinks (the upstream look; clipped on <39-column
	// terminals). Render at logoWidth+1 so the fitsWidth contract holds
	// while the long session title still exercises the wrap.
	got := stripANSI(a.home.render(&a.store, logoWidth+1, a.theme))
	fitsWidth(t, got, logoWidth+1)
```

Modify `internal/tui/app_test.go` (package `tui_test`) — the two `"Yolo"` wait tokens (lines 40–43 in `TestHomeRendersListAndNewSession`, lines 258–261 in `TestPromptSlashNewWithoutSession`) become the home marker that survives the restyle:

```go
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("New session"))
	}, teatest.WithDuration(5*time.Second))
```

Modify `internal/tui/tui_suite_test.go` — the four `"Yolo"` tokens become `"New session"`: lines 62 and 175 and 292 `hasLine("Yolo")` → `hasLine("New session")` (the home screen no longer prints the literal title — the logo replaces it; `"New session"` is the stable home marker, present in one bold run), and line 247 `capture("Yolo")` → `capture("New session")` (its frame seeds the dialog-ordering seq; the ordering assertion only inspects the Model/Agents/Help/quit tokens).

- [ ] **Step 2: Confirm the tests FAIL**

Run: `go test ./internal/tui/ -run 'TestLogoBlockPinned|TestRenderLogoZeroThemeIsPlain|TestHomeLogoThemeSGR|TestHomeRenderLockedLayout|TestHomeRenderWraps' -v`
Expected: FAIL — a build error, NOT an assertion failure: `undefined: logoLeft`, `undefined: logoRight`, `undefined: renderLogo` (logo.go does not exist yet) plus `too many arguments in call to a.home.render` (3 given, 2 wanted) in `home_test.go`/`overflow_test.go` — i.e. `FAIL	github.com/kido5217/yolo/internal/tui [build failed]`.

- [ ] **Step 3: Minimal implementation**

Create `internal/tui/logo.go`:

```go
package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// The 8 logo lines — the strict copy of upstream
// packages/tui/src/logo.ts (each line 19 columns). sha256-pinned in
// logo_test.go (root principle 3).
var (
	logoLeft = []string{
		"                   ",
		"█▀▀█ █▀▀█ █▀▀█ █▀▀▄",
		"█__█ █__█ █^^^ █__█",
		"▀▀▀▀ █▀▀▀ ▀▀▀▀ ▀~~▀",
	}
	logoRight = []string{
		"             ▄     ",
		"█▀▀▀ █▀▀█ █▀▀█ █▀▀█",
		"█___ █__█ █__█ █^^^",
		"▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀",
	}
)

// logoWidth is the fixed block width (left 19 + gap 1 + right 19). The
// logo never wraps or shrinks — on a <39-column terminal the
// alt-screen frame clips it (the upstream look).
const logoWidth = 39

// Mark classes (upstream marks "_^~,"). The glyph is always translated;
// the paint follows the class.
const (
	markPlain = iota // every unmarked rune: fg only
	markHollow       // '_','^' -> " "/"▀": fg + bg(shadow)
	markShadow       // '~',',' -> "▀"/"▄": fg(shadow)
)

// renderLogo renders the 4-line upstream logo block (logo.tsx:49–60):
// the left lines in textMuted (non-bold), the right lines in text
// (bold), one unstyled gap column between them. A zero Theme (nil-engine
// runs, S0.7) degrades to the plain translated glyphs — never a panic.
func renderLogo(th theme.Theme) string {
	fgLeft, leftOK := th.Color("textMuted")
	fgRight, rightOK := th.Color("text")
	bg, bgOK := th.Color("background")
	var b strings.Builder
	for i := range logoLeft {
		b.WriteString(logoLine(logoLeft[i], fgLeft, leftOK, bg, bgOK, false))
		b.WriteByte(' ')
		b.WriteString(logoLine(logoRight[i], fgRight, rightOK, bg, bgOK, true))
		if i+1 < len(logoLeft) {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// logoLine renders one 19-column line (the port of renderLine,
// logo.tsx:9–47). The mark glyphs are always translated ('_' -> " ",
// '^' -> "▀", '~' -> "▀", ',' -> "▄"); the paint follows the mark class
// with shadow = Tint(background, fg, 0.25) (logo.tsx:10). Consecutive
// same-class cells emit as one styled run; a missing token (zero Theme)
// renders the plain glyphs.
func logoLine(line string, fg theme.Rgba, fgOK bool, bg theme.Rgba, bgOK bool, bold bool) string {
	shadow := theme.Tint(bg, fg, 0.25)
	var (
		fgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(fg.Hex()[:7]))
		hollowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(fg.Hex()[:7])).Background(lipgloss.Color(shadow.Hex()[:7]))
		shadowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(shadow.Hex()[:7]))
	)
	if bold {
		fgStyle = fgStyle.Bold(true)
		hollowStyle = hollowStyle.Bold(true)
		shadowStyle = shadowStyle.Bold(true)
	}
	var b, run strings.Builder
	class := -1
	flush := func() {
		if run.Len() == 0 {
			return
		}
		glyphs := run.String()
		run.Reset()
		if !fgOK {
			b.WriteString(glyphs) // zero Theme: the plain translated glyphs
			return
		}
		var st lipgloss.Style
		switch {
		case class == markHollow && bgOK:
			st = hollowStyle
		case class == markShadow:
			st = shadowStyle
		default:
			st = fgStyle
		}
		b.WriteString(st.Render(glyphs))
	}
	for _, r := range line {
		var c int
		switch r {
		case '_':
			c, r = markHollow, ' '
		case '^':
			c, r = markHollow, '▀'
		case '~':
			c, r = markShadow, '▀'
		case ',':
			c, r = markShadow, '▄'
		default:
			c = markPlain
		}
		if c != class {
			flush()
			class = c
		}
		run.WriteRune(r)
	}
	flush()
	return b.String()
}
```

Modify `internal/tui/home.go` — the import block gains `"github.com/kido5217/yolo/internal/tui/theme"` (after the `store` import), and `render` becomes (the logo replaces the title + top divider; the bottom divider moves from the static `divider` (ANSI 240) to `th.BorderSubtle()`):

```go
// render produces the locked home layout for the store: the 4-line
// upstream logo (S0.8 — replaces the old title + top divider), the
// session rows word-wrapped at the terminal width (the cursor stays one
// stop per session — continuation lines align under the content), the
// theme borderSubtle divider and the dim help line.
func (h *homeModel) render(s *store.State, w int, th theme.Theme) string {
	h.clampCursor(s)
	rows := h.visible(s)
	var b strings.Builder
	b.WriteString(renderLogo(th))
	b.WriteByte('\n')
	b.WriteString(h.renderRow(0, "New session", w))
	b.WriteByte('\n')
	for i, se := range rows {
		b.WriteString(h.renderRow(i+1, lineContent(se, h.now()), w))
		b.WriteByte('\n')
	}
	b.WriteString(th.BorderSubtle().Render(dividerLine()))
	b.WriteByte('\n')
	b.WriteString(dimWrapped(helpText, w))
	return b.String()
}
```

Modify `internal/tui/view.go` — the home call site (line 35) passes the app theme:

```go
		b.WriteString(a.home.render(&a.store, w, a.theme))
```

Modify `internal/tui/style.go` — the var block comment gains the ownership note (the statics are UNCHANGED — only the comment moves):

```go
// Static styles. S0.8 moved the home surface to the theme accessors
// (a.theme) — home no longer consumes title/divider. Ownership of the
// remaining statics: dim (footer/help line) → S0.9; cursor, errRed,
// okGreen, toolRow → S0.10; title/divider serve the non-home surfaces
// (session chrome view.go + session.go, the dialogs dialog.go) →
// S0.10/S3.
var (
	title   = lipgloss.NewStyle().Bold(true)
	divider = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursor  = lipgloss.NewStyle().Bold(true)
	dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errRed  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	okGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	toolRow = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)
```

DOX pass — modify `internal/tui/AGENTS.md`: (1) the Ownership concern-file list gains `logo` (after `keys`): `app, hydrate, dialog, keys, logo, commands, view, footer, home, permission, prompt, session, style, toast, wrap`; (2) the `yolo-ukc` wrap-contract bullet gains the carve-out sentence: "The home logo (S0.8) is the one exception: a fixed 39-column glyph block that never wraps or shrinks (the upstream look) — terminals under 39 columns clip it in the alt-screen frame."

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/ -v` — Expected: PASS (new: `TestLogoBlockPinned`, `TestRenderLogoZeroThemeIsPlain`, `TestHomeLogoThemeSGR`; re-baselined: `TestHomeRenderLockedLayout`, `TestHomeRenderWraps`; re-tokenized teatest suites: `TestTUIFullTurn`, `TestTUIDialogs`, `TestTUILongReplyWraps`, `TestHomeRendersListAndNewSession`, `TestPromptSlashNewWithoutSession` — all green on the `"New session"` home marker).
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green — including `TestImportsDirection` (logo.go's `internal/tui/theme` import is `internal/tui/*` — pure-client-legal). gofmt prints nothing.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/logo.go internal/tui/logo_test.go internal/tui/home_theme_test.go internal/tui/home.go internal/tui/view.go internal/tui/style.go internal/tui/home_test.go internal/tui/overflow_test.go internal/tui/app_test.go internal/tui/tui_suite_test.go internal/tui/AGENTS.md
git commit -m "feat: shell restyle - upstream logo + border tokens"
bd close yolo-oae.1.8 --reason "upstream logo (strict copy, sha256 pin) + home borderSubtle divider + teatest SGR goldens (ANSI256 244/255/235/238/237 + bold); zero-Theme degrades to the plain logo" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.9: Shell restyle — home list + footer theme tokens (+ teatest SGR goldens) (`yolo-oae.1.9`)

**Files:**
- Modify: `internal/tui/home.go` — `lineParts` (replaces `lineContent`), the segment-preserving wrap (`rowLead`/`rowLine`/`rowLines`/`joinRowLine`, replacing `homeRow`), `renderRow` restyled with the selection tokens, the help line → `dimWrapped(th, …)`, imports gain lipgloss + theme
- Modify: `internal/tui/footer.go` — `footerView` main + status segments → `a.theme.TextMuted()` (the conn segment keeps the statics)
- Modify: `internal/tui/style.go` — `dim` REMOVED; ownership comment updated
- Modify: `internal/tui/view.go` — session help line → `a.theme.TextMuted()`; `a.sess.sync` + `a.prompt.menuView` gain the theme arg
- Modify: `internal/tui/dialog.go` — `helpDialog(th)` per-frame render (replaces the package-init `helpDialogRendered`), `dialogStack.view(th)`, model/agent dialog views gain `th` (dim → textMuted), `dimWrapped(th, …)`, `providerStatus(th, …)`, `modelCell(th, …)`
- Modify: `internal/tui/permission.go` — overlay rows → `a.theme.TextMuted()`
- Modify: `internal/tui/session.go` — `sync`/`renderMessages`/`renderAssistant` gain `th` (reasoning lines → textMuted)
- Modify: `internal/tui/prompt.go` — `menuView(cmds, w, th)` (no-match/rows → textMuted)
- Modify: `internal/tui/home_theme_test.go` — `TestHomeFooterThemeSGR` (teatest SGR goldens, real engine + seeded session), `TestHomeRowLines`, `TestHomeRenderRowZeroTheme`
- Modify: `internal/tui/session_test.go` + `session_bench_test.go` — the `renderMessages` call sites gain the theme arg (same commit); `overflow_test.go` — `menuView`/`dlg.agent().view`/`dlg.model().view` call sites; `agent_test.go` + `model_test.go` — the `view(&a.store, 80)` call sites
- Modify: `internal/tui/AGENTS.md` — DOX pass: the `yolo-ukc` bullet gains the segment-wrap + selected-row-background note

**Interfaces:**
- Consumes:
  - Task S0.3: `theme.Theme`; `th.Text()`, `th.TextMuted()`, `th.Primary() lipgloss.Style`; `th.Color(name string) (Rgba, bool)`; `th.SelectedForeground(bg ...Rgba) Rgba`; `Rgba.Hex()` (`#rrggbbaa` — lipgloss takes `[:7]`). Zero-Theme reads are safe (absent token → empty style, never a panic).
  - Task S0.7: `App.theme theme.Theme`; the zero-Theme nil-engine contract.
  - Task S0.8: `renderLogo(th)`, `logoWidth`, `homeModel.render(s, w, th)` (the home frame shape), the static `cursor` (bold, no fg — S0.10's; S0.8 ownership note: the home cursor row keeps it), the teatest SGR convention (TTY_FORCE=1 + TERM=xterm-256color, one merged `WaitFor`, substring tokens + param-order-agnostic regex).
  - Upstream (port source, read at execution time): `packages/tui/src/ui/dialog-select.tsx` (the session-list row renderer: the active-row box `backgroundColor={active() ? … : (option.bg ?? theme.primary)}` 667–678, `Option` 732–791 — title `fg=selectedForeground(theme)` + BOLD when active 746–749,766–770, the description span + footer textMuted 780–788, non-active title `theme.text`) and `packages/tui/src/feature-plugins/home/footer.tsx` (1–81: directory `textMuted`, the "N MCP" count `text` + dot error/success/textMuted, `/status` + version `textMuted`).
  - `x/ansi` v0.11.8 (go.sum-pinned) `Convert256` (`$(go env GOMODCACHE)/github.com/charmbracelet/x/ansi@v0.11.8/color.go:185`) — the SGR derivations below.
- Produces (binding for S0.10/S7):
  - `lineParts(s protocol.Session, now int64) (title, meta string)` — meta = the ` · provider/model · relTime` tail; `title+meta` is byte-identical to the old `lineContent` output.
  - `rowLines(prefix, title, meta string, w int) []rowLine` + `type rowLine struct{ cur, title, meta string }` + `rowLead(prefix string) (lead, body string)` + `joinRowLine([]wTag) rowLine` — the word-wrap (same contract as `wrapLine`: word boundaries, over-long tokens hard-split at the width, single-space rejoin) re-derived over TAGGED words so every visual line splits into its styled runs; a row that fits is one verbatim line (internal spacing preserved).
  - `homeModel.renderRow(line int, title, meta string, w int, th theme.Theme) string` — the restyled row (replaces `renderRow(line, content, w)` + `homeRow`).
  - `dimWrapped(th theme.Theme, s string, w int) string` — signature change (the theme arg first).
  - Theme-arg shape (pinned): `th theme.Theme` VALUE as the LAST parameter of the renderers — `renderMessages(st, expanded, w, th)`, `renderAssistant(m, expanded, w, th)`, `sessionModel.sync(st, w, h, th)`, `modelDlg.view(st, w, th)`, `agentDlg.view(st, w, th)`, `modelDlg.modelCell(th, st, p, models, j)`, `providerStatus(th, auth)`, `promptModel.menuView(cmds, w, th)`, `dialogStack.view(th)`, `helpDialog(th)`. Existing call sites + tests update in the same commit.
  - `style.go`: the `dim` static is GONE; every former `dim` consumer reads the theme `textMuted` accessor. Remaining statics keep their S0.10/S3 ownership: `title`, `divider` (session chrome + dialogs), `cursor` (home cursor row, model/agent dialog rows, slash-menu selection), `errRed`/`okGreen` (footer conn segment, `!` error line, provider dots), `toolRow` (transcript tool rows).

**Upstream parity notes (binding):**
- **Selected row** (port of the dialog-select active row, dialog-select.tsx:635–707 + 746–788): the row background is `option.bg ?? theme.primary` → yolo pins **`th.Color("primary")`** (yolo rows carry no per-row bg override; upstream's `option.bg = theme.error` is the session-list double-press delete-confirm state — no yolo home analog in S0.9). The row text is **`th.SelectedForeground()`** (the S0.3 port of `selectedForeground`, called WITHOUT a bg arg exactly as upstream's `Option` does — the opaque-background fallback returns the `background` token, the transparent-background fallback contrasts against `theme.primary` internally). The title run is **BOLD** (upstream bolds the active title); the metadata tail is **NOT bold** (upstream's dimmed description/footer runs).
- **Token mapping (pinned):** non-selected row title → `th.Text()`; the ` · provider/model · relTime` tail → `th.TextMuted()` (upstream dims per-row metadata — the Option description span + footer, dialog-select.tsx:780–788); the `▸` cursor indicator (yolo's home convention — upstream has no home cursor; its `●` marker is the session-list current-session only) → selected row: fg = `th.SelectedForeground()` + bold; zero Theme: the static `cursor` bold, content plain (the S0.8 note — S0.10 takes the static over).
- **Continuation-line background (pinned):** the background applies to **EVERY rendered line** of the selected row (first line + every word-wrapped continuation), covering **that line's rendered content only** — the plain lead/indent columns and the empty tail columns beyond the content carry NO background. Justification: the upstream selected row is one full-width box (`wrapMode="none"` — it never wraps), so yolo's word-wrapped rows (the yolo-ukc contract) have no full-width box to port; painting the bar per rendered line over its content keeps the highlight on the text without inventing a full-width bar over empty columns upstream never renders.
- **Footer** (port of the home footer.tsx token mapping): the home footer is a dim metadata line (directory/version `textMuted`; the one `text` element, "N MCP", has no yolo home analog in S0.9) → the footer main segments (model · agent · ↑in ↓out · $cost, BOTH routes) + the status segment render in **`a.theme.TextMuted()`**; the conn segment (`● live`/`○ off`/`◌ reconnecting`) keeps the static `okGreen`/`errRed` (S0.10 ownership) — the S7.4 session-footer detail restyle refines the session route afterwards.
- **dim removal blast radius (grep-verified):** `dim` is consumed by the home help line, the session chrome (help line, reasoning lines), the dialogs (help block, model/agent rows + hint lines + `loading…`), the permission overlay, the slash menu and the footer — S0.9 moves ALL of them to `th.TextMuted()` (ANSI 245 → 244, a one-ramp-step dim shift under ANSI256); the per-surface restyles (S0.10 session chrome, S3 dialogs, S5 prompt) build on the themed token, not the static. The help block moves from the package-init `helpDialogRendered` const to a per-frame `helpDialog(th)` (theme-dependent — the block is short, the per-frame cost is negligible).
- **Zero-Theme degradation** (the S0.7 nil-engine contract): every accessor returns an empty style → every surface renders plain; the cursor row keeps the static `cursor` bold on the `▸` run with plain content — never a panic, no SGR color from a missing token.

- [ ] **Step 1: Write the failing tests (+ re-baseline the theme-arg call sites)**

Append to `internal/tui/home_theme_test.go` (imports unchanged — S0.8's already cover bytes/context/filepath/regexp/strings/time/tea/teatest/testutil/client/store/theme):

```go
// homeSGRTokens are the S0.9 home-row/footer SGR color parameters under the
// pinned TTY_FORCE=1 + TERM=xterm-256color env (ANSI256 profile: the 24-bit
// hex tokens quantize through x/ansi v0.11.8 Convert256 —
// charmbracelet/x/ansi color.go: to6Cube (v<48→0, v<115→1, else (v-35)/40 →
// 0x00/0x5f/0x87/0xaf/0xd7/0xff), an exact cube hit returns early, else the
// grey index (avg-3)/10 with avg>238 → 23 (avg = (r+g+b)/3) and a
// DistanceHSLuv cube-vs-grey tie-break). Derived from the opencode
// dark-mode tokens:
//
//	text (non-selected row title)       #eeeeee (238):
//	    grey 23 exact (238 = 8+10*23) -> 255
//	textMuted (row meta tail, help line, footer)  #808080 (128):
//	    grey 12 exact (128 = 8+10*12) -> 244
//	SelectedForeground (selected row text + the ▸ indicator) — the port
//	    returns the opaque `background` token, #0a0a0a (10):
//	    cube 16 (0,0,0) vs grey 232 (8): the closer achromatic is 8 -> 232
//	primary (selected row background)   #fab283 (250,178,131):
//	    cube 16+36*5+6*3+2 = 216 (255,175,135) vs grey 250 (188, avg 186):
//	    the peach cube beats the achromatic grey in HSLuv -> 216
//
// 255/244 also appear in the S0.8 logo block (right/left lines) — the row
// and footer markers ("T1", "↑0 ↓0") in the WaitFor condition pin their
// contribution here.
//
// Substring assertions (no escape/terminator boundaries): the renderer's
// pen-diff merges the changed params into ONE CSI whose inner param order
// is not pinned (the redSGR / logoBoldRe precedent).
var homeSGRTokens = []string{
	"38;5;255", // non-selected row title (text)
	"38;5;244", // row meta tail, help line, footer (textMuted)
	"38;5;232", // selected row text + ▸ (SelectedForeground = background)
	"48;5;216", // selected row background (primary)
}

// selectedRowSGRRe matches the ▸ cell's merged bold+fg+bg CSI (all six
// param permutations — the order is not pinned). The ▸ run re-emits all
// three params in one transition: the two plain lead columns before it
// reset the pen to the default state, so the ▸ cell sets bold, foreground
// and background together.
var selectedRowSGRRe = regexp.MustCompile(`\x1b\[(?:1;38;5;232;48;5;216|1;48;5;216;38;5;232|38;5;232;1;48;5;216|38;5;232;48;5;216;1|48;5;216;1;38;5;232|48;5;216;38;5;232;1)m`)

// TestHomeFooterThemeSGR is the teatest SGR golden for the S0.9 shell
// restyle: boot the app with a REAL theme engine (the S0.7 wiring) over a
// real server seeded with one session, let it render home, and pin the
// home-row + footer SGR color parameters.
func TestHomeFooterThemeSGR(t *testing.T) {
	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}
	if got := e.Active(); got != "opencode" {
		t.Fatalf("active theme = %s, want opencode (no config, no KV)", got)
	}

	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, err := c.CreateSession(ctx, "T1"); err != nil {
		t.Fatalf("seed session T1: %v", err)
	}
	cancel()

	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		// The fake terminal is not a TTY, so lipgloss strips every style.
		// Pin the env that derives ANSI256 from TERM alone (suite
		// convention).
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	// ONE merged condition (consecutive WaitFors drain each other): the
	// home markers (the selected "New session" row, the seeded "T1" row,
	// the home footer), every SGR token and the ▸ cell's merged bold+fg+bg
	// CSI.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		if !strings.Contains(s, "New session") || !strings.Contains(s, "T1") ||
			!strings.Contains(s, "↑0 ↓0") {
			return false
		}
		for _, tok := range homeSGRTokens {
			if !bytes.Contains(b, []byte(tok)) {
				return false
			}
		}
		return selectedRowSGRRe.Match(b)
	}, teatest.WithDuration(5*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestHomeRowLines pins the segment-preserving wrap (rowLines): a row that
// fits keeps its verbatim spacing; a wrapped row re-derives the title/meta
// split per visual line — a mid-line boundary leaves the trailing join
// space on the title run, a line-boundary boundary drops it (wrapLine drops
// leading spaces on continuation lines).
func TestHomeRowLines(t *testing.T) {
	got := rowLines("  ▸ ", "New session", "", 80)
	if len(got) != 1 || got[0].cur != "▸ " || got[0].title != "New session" || got[0].meta != "" {
		t.Fatalf("fast path = %+v, want one verbatim line", got)
	}
	got = rowLines("  ", "T1", " · kido/q · 2m", 80)
	if len(got) != 1 || got[0].title != "T1 " || got[0].meta != "· kido/q · 2m" {
		t.Fatalf("join split = %+v, want the join space on the title run", got)
	}
	// effW = 18: six 3-word lines, then "word word · 2m" (the mid-line
	// boundary leaves the join space on the title run).
	long := strings.Repeat("word ", 20)
	got = rowLines("  ", long, " · 2m", 20)
	if len(got) != 7 {
		t.Fatalf("lines = %d, want 7: %+v", len(got), got)
	}
	for i := 0; i < 6; i++ {
		if got[i].cur != "" || got[i].title != "word word word" || got[i].meta != "" {
			t.Fatalf("line %d = %+v", i, got[i])
		}
	}
	if got[6].cur != "" || got[6].title != "word word " || got[6].meta != "· 2m" {
		t.Fatalf("last line = %+v", got[6])
	}
	// An over-long token hard-splits at the width (the wrapLine contract).
	got = rowLines("  ", strings.Repeat("x", 30), "", 20)
	if len(got) != 2 || got[0].title != strings.Repeat("x", 18) || got[1].title != strings.Repeat("x", 12) {
		t.Fatalf("hard split = %+v, want 18 + 12", got)
	}
}

// TestHomeRenderRowZeroTheme pins the nil-engine degradation (the S0.7
// zero-Theme rule): plain text — the cursor row keeps the static cursor
// bold on the "▸" run with plain content, and a wrapped continuation line
// indents 2 (4 for the cursor row).
func TestHomeRenderRowZeroTheme(t *testing.T) {
	var zero theme.Theme
	h := homeModel{cursor: 0}
	if got := stripANSI(h.renderRow(0, "New session", "", 80, zero)); got != "  ▸ New session" {
		t.Fatalf("cursor row = %q, want %q", got, "  ▸ New session")
	}
	if got := stripANSI(h.renderRow(1, "T1", " · kido/q · 2m", 80, zero)); got != "  T1 · kido/q · 2m" {
		t.Fatalf("row = %q, want %q", got, "  T1 · kido/q · 2m")
	}
	long := strings.Repeat("word ", 20)
	want := "  word word word\n" +
		strings.Repeat("  word word word\n", 5) +
		"  word word · 2m"
	if got := stripANSI(h.renderRow(1, long, " · 2m", 20, zero)); got != want {
		t.Fatalf("wrapped row = %q, want %q", got, want)
	}
}
```

Re-baseline the call sites (same commit; the zero-theme render stays byte-identical after `stripANSI`, so no `want` strings change):
- `internal/tui/session_test.go` — the import block gains `"github.com/kido5217/yolo/internal/tui/theme"`; lines 197, 236, 351: `renderMessages(&s, tt.expanded, 80)` → `renderMessages(&s, tt.expanded, 80, theme.Theme{})`, `renderMessages(&tt.s, nil, 80)` → `renderMessages(&tt.s, nil, 80, theme.Theme{})`, `renderMessages(&s, nil, w)` → `renderMessages(&s, nil, w, theme.Theme{})`.
- `internal/tui/session_bench_test.go` — the import block gains the theme import; line 116: `renderMessages(st, exp, 80)` → `renderMessages(st, exp, 80, theme.Theme{})`.
- `internal/tui/overflow_test.go` — line 53: `a.prompt.menuView(cmds, 20)` → `a.prompt.menuView(cmds, 20, a.theme)`; line 75: `a.dlg.agent().view(&a.store, 20)` → `a.dlg.agent().view(&a.store, 20, a.theme)`; line 89: `a.dlg.model().view(&a.store, 40)` → `a.dlg.model().view(&a.store, 40, a.theme)`.
- `internal/tui/agent_test.go` line 41: `a.dlg.agent().view(&a.store, 80)` → `a.dlg.agent().view(&a.store, 80, a.theme)`.
- `internal/tui/model_test.go` line 92: `a.dlg.model().view(&a.store, 80)` → `a.dlg.model().view(&a.store, 80, a.theme)`.

(`TestHomeRenderLockedLayout`, `TestFooterRender`, `TestMenuViewWraps` and the teatest suites need NO change: the zero-theme plain text of every surface is unchanged, and the S0.8 `"New session"` home marker survives.)

- [ ] **Step 2: Confirm the tests FAIL**

Run: `go test ./internal/tui/ -run 'TestHomeRowLines|TestHomeRenderRowZeroTheme|TestHomeFooterThemeSGR' -v`
Expected: FAIL — a build error, NOT an assertion failure: `undefined: rowLines` (home.go does not carry the segment wrap yet) plus `too many arguments in call to h.renderRow` / `renderMessages` / `a.prompt.menuView` / `a.dlg.agent().view` / `a.dlg.model().view` across `home_theme_test.go`, `session_test.go`, `session_bench_test.go`, `overflow_test.go`, `agent_test.go`, `model_test.go` — i.e. `FAIL	github.com/kido5217/yolo/internal/tui [build failed]`.

- [ ] **Step 3: Minimal implementation**

Modify `internal/tui/home.go` — the import block gains `"charm.land/lipgloss/v2"` (after `tea`) and `"github.com/kido5217/yolo/internal/tui/theme"` (after the `store` import). Replace `lineContent` with `lineParts`, replace `renderRow`/`homeRow` with the restyled pair, and update `render` (the S0.8 shape, only the row calls + the help line change):

```go
// render produces the locked home layout for the store: the 4-line upstream
// logo (S0.8), the session rows word-wrapped at the terminal width (the
// cursor stays one stop per session — continuation lines align under the
// content), the theme borderSubtle divider and the dimmed help line.
func (h *homeModel) render(s *store.State, w int, th theme.Theme) string {
	h.clampCursor(s)
	rows := h.visible(s)
	var b strings.Builder
	b.WriteString(renderLogo(th))
	b.WriteByte('\n')
	b.WriteString(h.renderRow(0, "New session", "", w, th))
	b.WriteByte('\n')
	for i, se := range rows {
		title, meta := lineParts(se, h.now())
		b.WriteString(h.renderRow(i+1, title, meta, w, th))
		b.WriteByte('\n')
	}
	b.WriteString(th.BorderSubtle().Render(dividerLine()))
	b.WriteByte('\n')
	b.WriteString(dimWrapped(th, helpText, w))
	return b.String()
}

// lineParts splits a session row into its title and the metadata tail
// (" · provider/model · relTime", dimmed like upstream's per-row footer);
// title+meta is byte-identical to the old lineContent output.
func lineParts(s protocol.Session, now int64) (title, meta string) {
	title = s.Title
	if s.Model != nil {
		meta += " \u00B7 " + s.Model.ProviderID + "/" + s.Model.ID
	}
	return title, meta + " \u00B7 " + relTime(s.Time.Updated, now)
}

// rowLead splits the row prefix into its leading-space lead (rendered
// plain) and the styled body ("  ▸ " is two plain spaces + the ▸ run).
func rowLead(prefix string) (lead, body string) {
	body = strings.TrimLeft(prefix, " \t")
	return prefix[:len(prefix)-len(body)], body
}

// rowLine is one visual line of a wrapped home row, split into its styled
// runs: cur the "▸" run (cursor rows, first line only), title the title run
// (its trailing join space when the line continues into the meta), meta the
// " · provider/model · relTime" tail.
type rowLine struct {
	cur   string
	title string
	meta  string
}

// rowLines wraps the plain home row (prefix + title + meta) at w with the
// same word-wrap contract as wrapLine (word boundaries, over-long tokens
// hard-split at the width, single-space rejoin) and re-derives the
// title/meta split per visual line. A row that fits is returned verbatim as
// one line (internal spacing preserved).
func rowLines(prefix, title, meta string, w int) []rowLine {
	lead, body := rowLead(prefix)
	plain := prefix + title + meta
	if w < 1 || plain == "" || runeWidth(plain) <= w {
		return []rowLine{{cur: body, title: title, meta: meta}}
	}
	type wTag struct {
		word string
		seg  int // 0 prefix, 1 title, 2 meta
	}
	var words []wTag
	add := func(s string, seg int) {
		for _, f := range strings.Fields(s) {
			words = append(words, wTag{f, seg})
		}
	}
	add(body, 0)
	add(title, 1)
	add(meta, 2)
	effW := w - runeWidth(lead)
	if effW < 1 {
		effW = 1
	}
	var (
		lines []rowLine
		cur   []wTag
		curW  int
	)
	flush := func() {
		if len(cur) == 0 {
			return
		}
		lines = append(lines, joinRowLine(cur))
		cur, curW = cur[:0], 0
	}
	for _, wd := range words {
		fw := runeWidth(wd.word)
		if fw > effW {
			flush()
			for rest := wd.word; len(rest) > 0 {
				chunk, r := cutWidth(rest, effW)
				lines = append(lines, joinRowLine([]wTag{{chunk, wd.seg}}))
				rest = r
			}
			continue
		}
		switch {
		case len(cur) == 0:
			cur, curW = append(cur, wd), fw
		case curW+1+fw <= effW:
			cur, curW = append(cur, wd), curW+1+fw
		default:
			flush()
			cur, curW = append(cur, wd), fw
		}
	}
	flush()
	return lines
}

// joinRowLine joins the tagged words of one visual line into its styled
// runs; a join space belongs to the PRECEDING word's run (a run's trailing
// space is where the next run starts on the same line; a line-boundary
// boundary drops it, as wrapLine drops leading spaces on continuation
// lines).
func joinRowLine(ws []wTag) rowLine {
	var l rowLine
	for i, wd := range ws {
		var p *string
		switch wd.seg {
		case 0:
			p = &l.cur
		case 1:
			p = &l.title
		default:
			p = &l.meta
		}
		*p += wd.word
		if i < len(ws)-1 {
			*p += " "
		}
	}
	return l
}

// renderRow renders one home row (line 0 is the "New session" row). The
// cursor row is the SELECTED row (upstream dialog-select active row): every
// rendered line is painted with the selection background (theme primary —
// upstream `option.bg ?? theme.primary`) and the text in
// SelectedForeground; the "▸" cursor run and the title run are bold
// (upstream bolds the active title), the metadata tail is not (upstream's
// dimmed description/footer runs). The background covers each rendered
// line's content only — no background on the plain indent or the empty
// tail beyond the content. Other rows: the title in the theme text token,
// the metadata tail in textMuted, no background. A zero Theme (nil-engine
// runs, S0.7) degrades: the cursor row keeps the static cursor bold on the
// "▸" run with plain content, every other row plain — never a panic.
func (h *homeModel) renderRow(line int, title, meta string, w int, th theme.Theme) string {
	cursor := line == h.cursor
	prefix := "  "
	if cursor {
		prefix = "  \u25B8 "
	}
	lead, _ := rowLead(prefix)
	lines := rowLines(prefix, title, meta, w)
	ind := 2
	if cursor {
		ind = 4
	}
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			n := ind
			if ww := runeWidth(l.cur) + runeWidth(l.title) + runeWidth(l.meta); ww+n > w {
				n = w - ww
				if n < 0 {
					n = 0
				}
			}
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(" ", n))
		} else {
			b.WriteString(lead)
		}
		writeRowLine(&b, l, cursor, th)
	}
	return b.String()
}

// writeRowLine renders one visual line's styled runs (see renderRow).
func writeRowLine(b *strings.Builder, l rowLine, cursor bool, th theme.Theme) {
	if !cursor {
		if l.title != "" {
			b.WriteString(th.Text().Render(l.title))
		}
		if l.meta != "" {
			b.WriteString(th.TextMuted().Render(l.meta))
		}
		return
	}
	if bg, ok := th.Color("primary"); !ok {
		// zero Theme: the static cursor bold (S0.10's) + plain content
		if l.cur != "" {
			b.WriteString(cursor.Render(l.cur))
		}
		b.WriteString(l.title + l.meta)
		return
	}
	sel := th.SelectedForeground()
	fg := lipgloss.Color(sel.Hex()[:7])
	bgStyle := lipgloss.Color(bg.Hex()[:7])
	head := lipgloss.NewStyle().Foreground(fg).Background(bgStyle).Bold(true)
	tail := lipgloss.NewStyle().Foreground(fg).Background(bgStyle)
	if l.cur != "" || l.title != "" {
		b.WriteString(head.Render(l.cur + l.title))
	}
	if l.meta != "" {
		b.WriteString(tail.Render(l.meta))
	}
}
```

(`lineContent` and `homeRow` are DELETED — no other references.)

Modify `internal/tui/footer.go` — the import block gains the theme import; `footerView` keeps its content assembly verbatim, but the join becomes (the main segments + status in textMuted, the conn segment unchanged on its statics):

```go
	muted := a.theme.TextMuted()
	main := muted.Render(strings.Join([]string{
		model,
		agent,
		"↑" + strconv.FormatInt(tokens.Input, 10) + " ↓" + strconv.FormatInt(tokens.Output, 10),
		fmt.Sprintf("$%.4f", cost),
	}, " · "))
	parts := []string{main}
	switch {
	case a.resyncing:
		parts = append(parts, errRed.Render("◌ reconnecting"))
	case a.store.Live:
		parts = append(parts, okGreen.Render("● live"))
	default:
		parts = append(parts, errRed.Render("○ off"))
	}
	if seg := a.statusSeg(); seg != "" {
		parts = append(parts, muted.Render(seg))
	}
	return strings.Join(parts, " · ")
```

(replacing the old `segs` slice + the single `strings.Join(segs, " · ")`; `statusSeg` is unchanged.)

Modify `internal/tui/style.go` — `dim` removed, ownership comment updated:

```go
// Static styles. S0.8 moved the home surface to the theme accessors
// (a.theme) — home no longer consumes title/divider. S0.9 removed dim —
// every former consumer reads the theme's textMuted accessor. Ownership of
// the remaining statics: cursor (home cursor row, model/agent dialog rows,
// slash-menu selection), errRed/okGreen (footer conn segment, the `!`
// error line, provider dots), toolRow (transcript tool rows) → S0.10;
// title/divider serve the non-home surfaces (session chrome view.go +
// session.go, the dialogs dialog.go) → S0.10/S3.
var (
	title   = lipgloss.NewStyle().Bold(true)
	divider = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursor  = lipgloss.NewStyle().Bold(true)
	errRed  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	okGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	toolRow = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)
```

Modify `internal/tui/view.go` — three call sites (the home call site already passes `a.theme` since S0.8):

```go
	menu := a.prompt.menuView(a.store.Commands, w, a.theme)
```
```go
	a.sess.sync(&a.store, w, h, a.theme)
```
```go
	for _, l := range help {
		b.WriteString("\n" + a.theme.TextMuted().Render(l))
	}
```

Modify `internal/tui/dialog.go` — the import block gains the theme import; the `helpDialogRendered` const is DELETED and replaced by a per-frame render (the "Static frame parts" comment updates: only `dividerLineRendered` + `quitDialogRendered` are precomputed now):

```go
// helpDialog renders the locked help block in the theme's textMuted token
// (S0.9: the static dim was removed — the block is short, the per-frame
// render is negligible).
func helpDialog(th theme.Theme) string {
	muted := th.TextMuted()
	return title.Render("Help") +
		"\n" + muted.Render("  | Key | Action |") +
		"\n" + muted.Render("  |---|---|") +
		"\n" + muted.Render("  | enter | send prompt |") +
		"\n" + muted.Render("  | esc | abort turn (busy) / close dialog |") +
		"\n" + muted.Render("  | ctrl+c | quit (confirm) |") +
		"\n" + muted.Render("  | ctrl+p | model dialog |") +
		"\n" + muted.Render("  | ctrl+a | agent dialog |") +
		"\n" + muted.Render("  | / | command menu |") +
		"\n" + muted.Render("  | pgup/pgdn | viewport scroll |") +
		"\n" + muted.Render("  | 1/2/3 | permission reply |") +
		"\n" + muted.Render("  | alt+e / alt+t | expand tool part / toggle reasoning |") +
		"\n" + muted.Render("  pgup/pgdn scroll \u00B7 \\+enter newline")
}

func (d dialogStack) view(th theme.Theme) string {
	top, ok := d.top()
	if !ok {
		return ""
	}
	switch top.kind {
	case dlgQuit:
		return quitDialogRendered
	case dlgHelp:
		return helpDialog(th)
	}
	return ""
}
```

`dlgView` passes the theme (`d.model.view(&a.store, w, a.theme)`, `d.agent.view(&a.store, w, a.theme)`, `return a.dlg.view(a.theme)`). `modelDlg.view` and `agentDlg.view` gain the `th theme.Theme` param with the dim replacements: `dim.Render("  loading…")` → `th.TextMuted().Render("  loading…")`, `rowSty := dim` / `sty := dim` / `rows = append(rows, dim.Render(l))` → `th.TextMuted()`, `providerStatus(p.Auth)` → `providerStatus(th, p.Auth)`, `m.modelCell(st, curProv, models, j)` → `m.modelCell(th, st, curProv, models, j)` (both call sites), `dimWrapped(s, w)` → `dimWrapped(th, s, w)` (the four hint lines). And:

```go
// dimWrapped word-wraps a plain line at w and renders each visual line in
// the theme's textMuted token (the static dim was removed in S0.9).
func dimWrapped(th theme.Theme, s string, w int) string {
	muted := th.TextMuted()
	var b strings.Builder
	for i, l := range strings.Split(wrapLine(s, w), "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(muted.Render(l))
	}
	return b.String()
}

func (m *modelDlg) modelCell(th theme.Theme, st *store.State, p protocol.Provider, models []protocol.Model, j int) (string, lipgloss.Style) {
	// …unchanged cell assembly…
	if j == m.selModel {
		return cell, cursor
	}
	return cell, th.TextMuted()
}

func providerStatus(th theme.Theme, auth *protocol.ProviderAuth) (string, lipgloss.Style) {
	switch {
	case auth != nil && auth.Status == "loaded":
		return "● loaded", okGreen
	case auth != nil && auth.RequiresKey && auth.Status == "missing":
		return "○ missing", errRed
	default:
		return "· not-required", th.TextMuted()
	}
}
```

Modify `internal/tui/permission.go` — `permissionView` computes `muted := a.theme.TextMuted()` once and the four `{dim, …}` row literals become `{muted, …}` (the `{title, …}` row is unchanged).

Modify `internal/tui/session.go` — the import block gains the theme import; `sync(st *store.State, w, h int, th theme.Theme)` (body: `renderMessages(st, sm.expanded, w, th)`), `renderMessages(st *store.State, expanded map[string]bool, w int, th theme.Theme)` (body: `renderAssistant(m, expanded, w, th)`), `renderAssistant(m protocol.MessageWithParts, expanded map[string]bool, w int, th theme.Theme)` — the three `writeStyled(dim, …)` reasoning calls become `writeStyled(th.TextMuted(), …)`.

Modify `internal/tui/prompt.go` — the import block gains the theme import; `menuView(cmds []protocol.Command, w int, th theme.Theme)`: `dim.Render("  no match")` → `th.TextMuted().Render("  no match")`, `sty := dim` → `sty := th.TextMuted()` (the `cursor` selection style is unchanged).

DOX pass — modify `internal/tui/AGENTS.md`: the `yolo-ukc` wrap-contract bullet (Local Contracts) gains: "Home session rows wrap as tagged segments (`rowLines` re-derives the title/` · meta` split per visual line, S0.9); the selected row's background paints every rendered line's content only — no background on the plain indent or the empty tail beyond the content."

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/ -v` — Expected: PASS (new: `TestHomeRowLines`, `TestHomeRenderRowZeroTheme`, `TestHomeFooterThemeSGR`; re-baselined call sites: the session/overflow/agent/model suites green on the zero-theme arg; unchanged: `TestHomeRenderLockedLayout`, `TestFooterRender`, the S0.8 logo SGR golden and the re-tokenized teatest suites — the zero-theme plain text of every surface is byte-identical).
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green — including `TestImportsDirection` (home.go's new `charm.land/lipgloss/v2` import is an allowlisted charm dep and `internal/tui/theme` is `internal/tui/*` — pure-client-legal). gofmt prints nothing.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/home.go internal/tui/footer.go internal/tui/style.go internal/tui/view.go internal/tui/dialog.go internal/tui/permission.go internal/tui/session.go internal/tui/prompt.go internal/tui/home_theme_test.go internal/tui/session_test.go internal/tui/session_bench_test.go internal/tui/overflow_test.go internal/tui/agent_test.go internal/tui/model_test.go internal/tui/AGENTS.md
git commit -m "feat: shell restyle - home list + footer theme tokens"
bd close yolo-oae.1.9 --reason "home rows restyled with the selection tokens (primary bg + selectedForeground, title bold, meta dimmed) + segment-preserving wrap; the dim static removed (all surfaces on textMuted); footer themed; teatest SGR goldens (232/216/255/244 + the ▸ merged bold+fg+bg CSI)" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

### Task S0.10: Shell restyle — session chrome theme tokens (+ teatest SGR goldens) (`yolo-oae.1.10`)

**Files:**
- Modify: `internal/tui/session.go` — `toolRowLine(p, th)` on the upstream tool-row tokens, the `!` message-error line → `th.Error()`
- Modify: `internal/tui/footer.go` — the conn segment → `a.theme.Success()`/`a.theme.Error()`
- Modify: `internal/tui/view.go` — the app-level `!` lastErr line → `a.theme.Error()`
- Modify: `internal/tui/toast.go` — `toastsView` → `a.theme.Error()`
- Modify: `internal/tui/app.go` — `retheme()` applies the prompt cursor color (import gains lipgloss)
- Modify: `internal/tui/home.go` — the `writeRowLine` zero-Theme fallback: static `cursor` → `cursorStyle(th)`
- Modify: `internal/tui/prompt.go` — the slash-menu selection: `cursor` → `cursorStyle(th)`
- Modify: `internal/tui/dialog.go` — the model/agent selection styles: `cursor` → `cursorStyle(th)`; `providerStatus` dots: `okGreen`/`errRed` → `th.Success()`/`th.Error()`
- Modify: `internal/tui/style.go` — `cursor`/`errRed`/`okGreen`/`toolRow` REMOVED; `cursorStyle(th)` added; ownership comment (ONLY `title` + `divider` survive)
- Create: `internal/tui/session_theme_test.go` — `TestSessionChromeThemeSGR` (teatest SGR goldens, real engine), `TestToolRowLineTheme` (the state→token chain, unit), `TestSessionChromeZeroThemeIsPlain` (zero Theme)
- Modify: `internal/tui/tui_suite_test.go` — `redSGR` REMOVED (+ its `bytes` import); the `TestTUIPermissionFlow/reject` subtest re-baselined (no SGR assertion — a zero-engine run renders plain)
- Modify: `internal/tui/AGENTS.md` — DOX pass: the app-shell-themed note (Local Contracts)

**Interfaces:**
- Consumes:
  - Task S0.3: `theme.Theme`; `th.Text()`, `th.TextMuted()`, `th.Error()`, `th.Success() lipgloss.Style`; `th.Color(name string) (Rgba, bool)`; `Rgba.Hex()` (`#rrggbbaa` — lipgloss takes `[:7]`). Zero-Theme reads are safe (absent token → empty style, never a panic).
  - Task S0.7: `App.theme theme.Theme`; `(*App).retheme()`; the zero-Theme nil-engine contract.
  - Task S0.9: the theme-arg renderers — `renderMessages(st, expanded, w, th)`, the themed footer main segments, `menuView(cmds, w, th)`, `modelDlg.view`/`agentDlg.view(st, w, th)`, `modelCell(th, …)`, `providerStatus(th, auth)`, `writeRowLine(b, l, cursor, th)`; the teatest SGR convention (TTY_FORCE=1 + TERM=xterm-256color, one merged `WaitFor`, substring tokens + param-order-agnostic regex).
  - Upstream (port source, read at execution time): `packages/tui/src/routes/session/index.tsx` — `InlineTool` fg memo 1882–1889 (the tool-row token mapping), `InlineToolRow` 1922–2000 (the icon/row/error runs), `BlockTool` error 2047–2049, the assistant error box 1534–1548; `packages/tui/src/routes/session/footer.tsx` 70–84 (the status dots: good = `theme.success`, bad = `theme.error`); `packages/tui/src/component/prompt/index.tsx` 252–254 (the cursor: `input.cursorColor = theme.text`); `packages/tui/src/ui/toast.tsx` 40–44 (upstream's general toast is `theme.text` — yolo's block is error-only).
  - bubbles v2.2.1: `textinput/styles.go:29-33,70-98` — the default `Cursor.Color` is `lipgloss.Color("7")`; `CursorStyle` carries {Color, Shape, Blink, BlinkSpeed} — NO bold field; `textinput.go:940-959` — the virtual cursor's style is `fg(Cursor.Color)`, the static (non-blinking) mode renders `Style.Inline(true).Reverse(true)` (cursor.go:238-240).
  - `x/ansi` v0.11.8 (go.sum-pinned) `Convert256` (`$(go env GOMODCACHE)/github.com/charmbracelet/x/ansi@v0.11.8/color.go:185`) — the SGR derivations below.
- Produces (binding for S1/S2–S3/S7):
  - `cursorStyle(th theme.Theme) lipgloss.Style` — bold + the theme text foreground; zero Theme → plain bold (never a panic).
  - `toolRowLine(p protocol.Part, th theme.Theme) (lipgloss.Style, string, bool)` — signature change (the theme arg LAST, the S0.9 convention); the row text is byte-identical to before.
  - The prompt cursor: `retheme()` sets `Styles().Cursor.Color` from the `text` token (absent token → no change, the default is kept); `Blink` stays false (V1 pin: the static non-blinking cursor, app.go).
  - `style.go` final shape: ONLY `title` + `divider` (+ the `dividerWidth` lock + `dividerLine`).

**Upstream parity notes (binding):**
- **Tool-row token mapping (pinned)** — upstream `InlineTool.fg()` (index.tsx:1882–1889) keys the row foreground on the row's state; yolo keys it on the part status (the same states — yolo's row always carries a title, so the upstream pending `~`-text state never occurs):
  - completed `✓ <tool> <title>` → **`th.TextMuted()`** — the settled row: `if (props.complete) return theme.textMuted` (index.tsx:1887).
  - running `▶ <tool> <title>` → **`th.Text()`** — the live row: `return theme.text` (index.tsx:1888).
  - error `✗ <tool> <error>` → **`th.Error()`** — the failed row: `if (failed()) return theme.error` (index.tsx:1885); the expanded error block shares the token (the `InlineToolRow` error run `fg={props.errorColor}`, index.tsx:1995; `BlockTool` error index.tsx:2048).
  - The upstream `permission()` branch (`theme.warning`, index.tsx:1884) has NO yolo S0.10 analog — yolo renders the ask as the overlay (permission.go) while the row keeps its running state; that state lands with the permission-dialog restyle (S2–S3). `th.Warning()` is deliberately not used here.
- **`!` error line + error states:** the transcript message-error line, the app-level lastErr line (view.go) and the toasts (toast.go) all render in **`th.Error()`** — upstream error text is always `fg=theme.error` (index.tsx:1995/2048; the assistant error box border `borderColor={theme.error}`, index.tsx:1544). The yolo toast block is error-ONLY (LOCKED red, toast.go) — the upstream general-purpose toast uses `theme.text` (toast.tsx:40–44), but for the error-only block `th.Error()` is the strict mapping (the LOCKED comment stands).
- **Footer conn segment (pinned):** connected `● live` → **`th.Success()`**; the error states `○ off` + `◌ reconnecting` → **`th.Error()`** — the upstream status-dot semantics, good = `theme.success` / bad = `theme.error` (footer.tsx:70,76,79). The provider dialog dots follow the same semantics: `● loaded` → `th.Success()`, `○ missing` → `th.Error()`.
- **Prompt cursor (pinned):** the cursor stays **BOLD** and takes the **`th.Text()`** foreground. Upstream: `input.cursorColor = theme.text` (prompt/index.tsx:253, also 1434–1441). Today yolo styles it in app.go's `NewApp` as `st.Cursor.Blink = false` over the bubbles default `Cursor.Color` = `lipgloss.Color("7")` — the MINIMAL change is in `retheme()`: set `st.Cursor.Color` from the `text` token. `CursorStyle` has no bold field (bubbles v2.2.1 textinput/styles.go:70–98), so the BOLD + text-fg pin lands on `cursorStyle(th)` = `th.Text().Bold(true)` for every selection surface that carried the static `cursor` (home cursor row, slash-menu selection, model/agent dialog rows).
- **style.go final shape (pinned):** after this task ONLY `title` (bold) + `divider` (ANSI 240) survive, with the ownership comment: they serve dialog.go (5 uses — the quit/help/model/agents titles + the `dividerLineRendered` const) + the view.go session title/divider surfaces (S2–S3 restyle) and the session.go transcript divider (S1). The upstream session route has NO title line and separates messages with margins, not dividers (index.tsx:1199, 1404) — there is no upstream token for those two; the session help line was already moved to `th.TextMuted()` by S0.9 and is untouched.
- **Zero-Theme degradation** (the S0.7 nil-engine contract): every accessor returns an empty style → tool rows, `!` lines, toasts and the conn segment render plain; `cursorStyle` is plain bold; the prompt cursor keeps its default color (no `text` token → no `SetStyles` call). Never a panic, no SGR from a missing token.

- [ ] **Step 1: Write the failing tests (+ re-baseline the static-SGR assertion)**

Create `internal/tui/session_theme_test.go`:

```go
package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// sessionChromeSGRTokens are the S0.10 session-chrome SGR color parameters
// under the pinned TTY_FORCE=1 + TERM=xterm-256color env (ANSI256 profile:
// the 24-bit hex tokens quantize through x/ansi v0.11.8 Convert256 —
// charmbracelet/x/ansi color.go:185: to6Cube (v<48→0, v<115→1, else
// (v-35)/40 → 0x00/0x5f/0x87/0xaf/0xd7/0xff), an exact cube hit returns
// early, else the grey index (avg-3)/10 with avg>238 → 23 (avg =
// (r+g+b)/3) and a DistanceHSLuv cube-vs-grey tie-break, the cube winning
// ties). Derived from the opencode dark-mode tokens (the S0.2 goldens):
//
//	text (running tool row, prompt cursor)  #eeeeee (238,238,238):
//	    grey 23 exact (238 = 8+10*23) -> 255
//	textMuted (completed tool row)         #808080 (128,128,128):
//	    grey 12 exact (128 = 8+10*12) -> 244
//	error (error tool row, `!` lines, `○ off`/`◌ reconnecting`)  #e06c75 (224,108,117):
//	    cube (215,95,135) -> 16+152 = 168 vs grey 14 (avg 149 -> 148) -> 246:
//	    the achromatic grey wins in HSLuv -> 246
//	success (footer `● live`, provider `● loaded`)  #7fd88f (127,216,143):
//	    cube (135,215,135) -> 16+98 = 114 vs grey 15 (avg 162 -> 158) -> 247:
//	    the green cube beats the achromatic grey in HSLuv -> 114
//
// 244/255 also appear in the S0.8 logo + S0.9 home/footer surfaces; the
// marker text below pins each chrome contribution.
//
// Substring assertions (no escape/terminator boundaries): the renderer's
// pen-diff merges the changed params into ONE CSI whose inner param order
// is not pinned (the redSGR / logoBoldRe precedent).
var sessionChromeSGRTokens = []string{
	"38;5;244", // completed tool row (textMuted)
	"38;5;246", // error tool row (error)
	"38;5;114", // footer `● live` (success)
	"38;5;255", // prompt cursor + running tool row (text)
}

// Position-anchored row/dot regexes: the CSI that OPENS the styled run must
// carry the token (param order within the CSI not pinned, resets merged in).
var (
	completedRowRe = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*38;5;244(?:;[0-9]+)*m\u2713 read`)
	errorRowRe     = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*38;5;246(?:;[0-9]+)*m\u2717 bash`)
	liveDotRe      = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*38;5;114(?:;[0-9]+)*m\u25CF live`)
	// The prompt cursor: the static virtual cursor's render is
	// fg(text)+reverse (bubbles cursor.View: Style.Inline(true).Reverse(true),
	// Style = fg(Cursor.Color) via SetStyles) — the merged CSI carries both
	// (order not pinned, other params may merge in).
	cursorRe = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*(?:7;38;5;255|38;5;255;7)(?:;[0-9]+)*m`)
)

// TestSessionChromeThemeSGR is the teatest SGR golden for the S0.10 shell
// restyle: boot the app with a REAL theme engine (the S0.7 wiring) over a
// real server whose scripted turn completes a read, rejects a bash ask and
// ends with text, and pin the session-chrome SGR color parameters.
func TestSessionChromeThemeSGR(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "tool", Name: "read", CallID: "call_1", Args: json.RawMessage(`{"filePath":"hello.txt"}`), Finish: "tool_calls"},
		}},
		fake.Turn{Parts: []llm.Part{
			{Kind: "tool", Name: "bash", CallID: "call_2", Args: json.RawMessage(`{"command":"echo hi"}`), Finish: "tool_calls"},
		}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "all done"}}},
	)
	cfg := &protocol.Config{Permission: map[string]any{"bash": "ask"}}
	ts := testutil.BootWithDriverConfig(t, drv, cfg)
	if err := os.WriteFile(filepath.Join(ts.Dir, "hello.txt"), []byte("world\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}
	if got := e.Active(); got != "opencode" {
		t.Fatalf("active theme = %s, want opencode (no config, no KV)", got)
	}

	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		// The fake terminal is not a TTY, so lipgloss strips every style.
		// Pin the env that derives ANSI256 from TERM alone (suite
		// convention).
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "read then bash")
	tm.Send(press(tea.KeyEnter))
	teatest.WaitFor(t, tm.Output(), hasPermDialogEcho, teatest.WithDuration(5*time.Second))
	// The park lands on the engine's goroutine after the render; sync on it
	// before replying (same guard as TestPermissionDialogKeyReply).
	waitPending(t, ts, 1)
	tm.Send(press('3')) // reject the bash ask -> the tool error part

	// ONE merged condition (consecutive WaitFors drain each other): the
	// session-chrome markers (the completed read row, the rejected bash
	// row, the final text, the help line, the live conn dot), every SGR
	// token, and the position-anchored row/dot/cursor regexes.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		if !strings.Contains(s, "\u2713 read") || !strings.Contains(s, "\u2717 bash") ||
			!strings.Contains(s, "all done") || !strings.Contains(s, "esc abort/back") ||
			!strings.Contains(s, "\u25CF live") {
			return false
		}
		for _, tok := range sessionChromeSGRTokens {
			if !bytes.Contains(b, []byte(tok)) {
				return false
			}
		}
		return completedRowRe.Match(b) && errorRowRe.Match(b) &&
			liveDotRe.Match(b) && cursorRe.Match(b)
	}, teatest.WithDuration(10*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestToolRowLineTheme pins the state->token chain at the lipgloss level
// (pure render, no renderer): the resolved opencode dark tokens emit their
// 24-bit hex as the foreground (the 38;5;N quantization is pinned by the
// teatest golden above).
func TestToolRowLineTheme(t *testing.T) {
	themes, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(themes["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}
	tests := []struct {
		name   string
		part   protocol.Part
		want   string
		fgWant string
	}{
		{
			name: "completed -> textMuted",
			part: protocol.Part{ID: "t1", Type: "tool", Tool: "read", CallID: "call_1",
				State: &protocol.ToolState{Status: "completed", Title: "f.go"}},
			want:   "\u2713 read f.go",
			fgWant: "#808080",
		},
		{
			name: "running -> text",
			part: protocol.Part{ID: "t2", Type: "tool", Tool: "bash", CallID: "call_2",
				State: &protocol.ToolState{Status: "running", Title: "ls -la"}},
			want:   "\u25B6 bash ls -la",
			fgWant: "#eeeeee",
		},
		{
			name: "error -> error",
			part: protocol.Part{ID: "t3", Type: "tool", Tool: "grep", CallID: "call_3",
				State: &protocol.ToolState{Status: "error", Title: "grep", Error: "no match"}},
			want:   "\u2717 grep no match",
			fgWant: "#e06c75",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, row, ok := toolRowLine(tt.part, th)
			if !ok || row != tt.want {
				t.Fatalf("toolRowLine = (%q, %v), want (%q, true)", row, ok, tt.want)
			}
			if got := string(st.GetForeground()); got != tt.fgWant {
				t.Errorf("fg = %q, want %q", got, tt.fgWant)
			}
		})
	}
}

// TestSessionChromeZeroThemeIsPlain pins the nil-engine degradation (the
// S0.7 zero-Theme rule): tool rows + the `!` error line + the footer conn
// segment render plain — no SGR from a missing token, never a panic.
func TestSessionChromeZeroThemeIsPlain(t *testing.T) {
	s := sessionFixture()
	s.Messages[1].Info.Error = &protocol.MessageError{Type: "unknown", Message: "something broke"}
	got := renderMessages(&s, nil, 80, theme.Theme{})
	if got != stripANSI(got) {
		t.Fatalf("zero-theme transcript carries SGR:\n%q", got)
	}
	want := "User: hello\n" +
		dividerLine() + "\n" +
		"\u25B8 think\n" +
		"\u2713 read src/main.go\n" +
		"\u25B6 bash ls -la\n" +
		"\u2717 grep pattern: no match\n" +
		"ok-text\n" +
		"! something broke"
	if got != want {
		t.Fatalf("transcript = %q, want %q", got, want)
	}

	a := newRecApp(client.New("http://127.0.0.1:9", ""), store.State{Live: true}, "")
	fv := a.footerView()
	if fv != stripANSI(fv) {
		t.Fatalf("zero-theme footer carries SGR:\n%q", fv)
	}
	if got := stripANSI(fv); got != "no model · default · ↑0 ↓0 · $0.0000 · ● live" {
		t.Fatalf("footer = %q", got)
	}
}
```

Re-baseline `internal/tui/tui_suite_test.go` (same commit): DELETE the `redSGR` helper (lines 140–148) and its `bytes` import (the only use); the `TestTUIPermissionFlow` "reject" subtest drops the SGR assertion (the harness uses `newRecApp` — a ZERO-engine run, which after this task renders the error row plain; the error token's SGR is pinned by `TestSessionChromeThemeSGR` under the real engine) and the test comment updates:

```go
	teatest.WaitFor(t, tm.Output(), hasLines("\u2717 bash", "permission rejected", "all done"), teatest.WithDuration(5*time.Second))
```

(`// TestTUIPermissionFlow: … reject renders the tool error part (theme error token — SGR pinned by TestSessionChromeThemeSGR under the real engine) with the engine's locked "permission rejected" text (the plan's "forbidden" word deviates; deviation 56).`)

- [ ] **Step 2: Confirm the tests FAIL**

Run: `go test ./internal/tui/ -run 'TestSessionChromeThemeSGR|TestToolRowLineTheme|TestSessionChromeZeroThemeIsPlain' -v`
Expected: FAIL — a build error, NOT an assertion failure: `too many arguments in call to toolRowLine` (2 given, 1 wanted) in `session_theme_test.go` — i.e. `FAIL	github.com/kido5217/yolo/internal/tui [build failed]`.

- [ ] **Step 3: Minimal implementation**

Modify `internal/tui/session.go` — `toolRowLine` gains the theme arg and moves off the statics (the row text is byte-identical):

```go
// toolRowLine renders the locked tool row: "✓ <tool> <title>" completed,
// "▶ <tool> <title>" running, "✗ <tool> <error>" error (first error line),
// in the upstream InlineTool fg tokens (index.tsx:1882-1889): completed ->
// textMuted, running -> text, error -> error. A zero Theme (nil-engine
// runs, S0.7) degrades to plain rows — never a panic. The caller applies
// the style per wrapped line (the row may wrap).
func toolRowLine(p protocol.Part, th theme.Theme) (lipgloss.Style, string, bool) {
	st := p.State
	status := "running"
	title := ""
	if st != nil {
		status = st.Status
		title = st.Title
	}
	if title == "" {
		title = toolTitleFallback(p)
	}
	switch status {
	case "completed":
		return th.TextMuted(), "\u2713 " + p.Tool + " " + title, true
	case "error":
		errText := ""
		if st != nil {
			errText = st.Error
		}
		if i := strings.IndexByte(errText, '\n'); i >= 0 {
			errText = errText[:i]
		}
		if errText == "" {
			errText = title
		}
		return th.Error(), "\u2717 " + p.Tool + " " + errText, true
	default:
		return th.Text(), "\u25B6 " + p.Tool + " " + title, true
	}
}
```

The `renderAssistant` call site (`sty, row, ok := toolRowLine(p, th)`) and the message-error line change:

```go
	if m.Info.Error != nil {
		writeStyled(th.Error(), "! "+m.Info.Error.Message)
	}
```

Modify `internal/tui/footer.go` — the conn segment of `footerView` (the S0.9 shape, only the conn switch changes):

```go
	parts := []string{main}
	switch {
	case a.resyncing:
		parts = append(parts, a.theme.Error().Render("◌ reconnecting"))
	case a.store.Live:
		parts = append(parts, a.theme.Success().Render("● live"))
	default:
		parts = append(parts, a.theme.Error().Render("○ off"))
	}
```

Modify `internal/tui/view.go` — the lastErr line (the app-level `!` line) renders in the theme error token:

```go
	if a.lastErr != "" {
		b.WriteByte('\n')
		for i, l := range strings.Split(wrapLine("! "+a.lastErr, w), "\n") {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(a.theme.Error().Render(l))
		}
	}
```

Modify `internal/tui/toast.go` — `toastsView` renders each wrapped toast line in the theme error token (the error-only block keeps its LOCKED red semantics via the token): `b.WriteString(errRed.Render(l))` → `b.WriteString(a.theme.Error().Render(l))`.

Modify `internal/tui/app.go` — the import block gains `"charm.land/lipgloss/v2"` (after `tea`), and `retheme` applies the prompt cursor (upstream prompt/index.tsx:253: `input.cursorColor = theme.text`; the cursor stays static — `Blink` untouched):

```go
// retheme refreshes a.theme from the engine (the port of the upstream
// values() memo read, theme.tsx:256-267) and applies the theme to the
// prompt cursor (upstream cursorColor = theme.text, prompt/index.tsx:253;
// CursorStyle carries a Color but no bold field — bubbles v2.2.1
// textinput/styles.go:70-98). With no engine (or no `text` token),
// a.theme stays the zero Theme and the cursor keeps its default.
func (a *App) retheme() {
	if a.engine == nil {
		return
	}
	if th, err := a.engine.ActiveTheme(); err == nil {
		a.theme = th
		if c, ok := th.Color("text"); ok && c.A != 0 {
			st := a.prompt.input.Styles()
			st.Cursor.Color = lipgloss.Color(c.Hex()[:7])
			a.prompt.input.SetStyles(st)
		}
	}
}
```

Modify `internal/tui/home.go` — the `writeRowLine` zero-Theme fallback takes the static over: `b.WriteString(cursor.Render(l.cur))` → `b.WriteString(cursorStyle(th).Render(l.cur))` (zero Theme: plain bold — byte-identical to the old static).

Modify `internal/tui/prompt.go` — `menuView`: `sty = cursor` → `sty = cursorStyle(th)`.

Modify `internal/tui/dialog.go` — the three selection styles: `rowSty = cursor` (model view) → `rowSty = cursorStyle(th)`, `sty = cursor` (agent view) → `sty = cursorStyle(th)`, and `modelCell`: `return cell, cursor` → `return cell, cursorStyle(th)`. And `providerStatus` (the S0.9 shape, the dots themed):

```go
func providerStatus(th theme.Theme, auth *protocol.ProviderAuth) (string, lipgloss.Style) {
	switch {
	case auth != nil && auth.Status == "loaded":
		return "● loaded", th.Success()
	case auth != nil && auth.RequiresKey && auth.Status == "missing":
		return "○ missing", th.Error()
	default:
		return "· not-required", th.TextMuted()
	}
}
```

Modify `internal/tui/style.go` — `cursor`/`errRed`/`okGreen`/`toolRow` REMOVED (no remaining references); the final shape:

```go
package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// dividerWidth is locked by the home layout: 28 box-drawing runes.
const dividerWidth = 28

// Static styles. S0.8 moved the home surface, S0.9 the footer, and S0.10
// the session chrome to the theme accessors (a.theme). The two remaining
// statics have no upstream session-chrome analog (the upstream session
// route has no title line and separates messages with margins, not
// dividers, index.tsx:1199,1404): title (bold) + divider (ANSI 240) serve
// dialog.go (5 uses — the quit/help/model/agents titles + the
// dividerLineRendered const) and the view.go session title/divider + the
// session.go transcript divider — S2–S3 (dialog restyles) / S1 (glamour
// message layout) retheme them.
var (
	title   = lipgloss.NewStyle().Bold(true)
	divider = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// cursorStyle is the theme cursor/selection style (S0.10 removed the
// static cursor): bold + the theme text foreground — a zero Theme
// (nil-engine runs, S0.7) degrades to plain bold, never a panic.
func cursorStyle(th theme.Theme) lipgloss.Style { return th.Text().Bold(true) }

func dividerLine() string { return strings.Repeat("─", dividerWidth) }
```

DOX pass — modify `internal/tui/AGENTS.md`: Local Contracts gains one bullet (after the bash-preview bullet): "App-shell themed (S0.8–S0.10): every shell surface reads the theme (the `th` render-arg or `a.theme`) — the only remaining statics are `title` + `divider` (the yolo-specific session title/divider + the dialog titles — S2–S3/S1 retheme them) and the `cursorStyle(th)` helper (bold + text fg). teatest SGR goldens pin TTY_FORCE=1 + TERM=xterm-256color (ANSI256 `38;5;N` via x/ansi v0.11.8 `Convert256` — deviation 125 at the S0 slice gate)."

- [ ] **Step 4: Run to verify it passes, then gate**

Run: `go test ./internal/tui/ -v` — Expected: PASS (new: `TestSessionChromeThemeSGR`, `TestToolRowLineTheme`, `TestSessionChromeZeroThemeIsPlain`; re-baselined: the `TestTUIPermissionFlow/reject` subtest — zero-engine run, no SGR assertion; unchanged: the `TestFooterRender` + `TestRenderMessages` tables (the zero-theme plain text is byte-identical), the S0.8/S0.9 SGR goldens and the re-tokenized teatest suites).
Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green — including `TestImportsDirection` (the new `internal/tui/theme` imports in session.go/toast.go/view.go/style.go are `internal/tui/*` — pure-client-legal; app.go's `charm.land/lipgloss/v2` is an allowlisted charm dep). gofmt prints nothing.

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/session.go internal/tui/footer.go internal/tui/view.go internal/tui/toast.go internal/tui/app.go internal/tui/home.go internal/tui/prompt.go internal/tui/dialog.go internal/tui/style.go internal/tui/session_theme_test.go internal/tui/tui_suite_test.go internal/tui/AGENTS.md
git commit -m "feat: shell restyle - session chrome theme tokens"
bd close yolo-oae.1.10 --reason "session chrome on theme tokens: tool rows (textMuted/text/error), ! line + toasts (error), footer conn (success/error), prompt cursor (text, upstream cursorColor=theme.text), cursorStyle helper; style.go down to title+divider; teatest SGR goldens (244/246/114/255)" --json
```

**STOP** — report gate, commit, `git status`; wait for go-ahead.

## S0 slice gate (slice bead `yolo-oae.1`)

NOT a task bead: runs once, after yolo-oae.1.1–yolo-oae.1.10 are ALL closed.
The slice bead closes only when every task bead is closed and the slice gate
is green (plan "Bead model").

- [ ] **Step 1: Final module gate at root**

Run at module root: `go vet ./... && go test ./...` then `gofmt -l .`
Expected: all green (including `TestImportsDirection` + every teatest SGR
golden of S0.8–S0.10); gofmt prints nothing.

- [ ] **Step 2: Verify the x/term promotion landed**

Run: `grep 'charmbracelet/x/term' go.mod`
Expected: `github.com/charmbracelet/x/term v0.2.2` WITHOUT the `// indirect`
comment (the S0.5 dep-proposal promotion — raw-mode tty for the OSC queries;
still zero NEW modules).

- [ ] **Step 3: Manual smoke (user-run, NOT CI)**

In a REAL terminal (a TTY — the teatest fakes pin ANSI256; a truecolor
terminal exercises the 24-bit SGR path):
- `just tui` or `go run ./cmd/yolo` — the logo renders (the S0.8 block), the
  home rows + footer render in the theme.
- Theme selection: set `theme: "<name>"` in the ACTIVE profile config
  (`~/.config/yolo/<id>/yolo.jsonc`) — restart — the shell re-renders in that
  theme (config > KV > default).
- Custom discovery + refresh: drop a custom theme under `~/.config/yolo/themes/`
  (or `.yolo/themes/` under the CWD), `kill -USR2 <pid>` — the 250/1000 ms
  refresh legs re-discover customs (deviation 124: the palette stays cached).
NOTE: this step is user-run and on-demand — it never runs in CI (tests never
hit the network / a real tty).

- [ ] **Step 4: Append DEVIATIONS.md entry 125 (render/low, 2026-08-25)**

Append after entry 124:

> 125. lipgloss color-profile quantization of the SGR encoding (render/low,
> 2026-08-25): yolo renders through lipgloss v2 + the bubbletea v2 renderer,
> which quantize hex tokens per the TERMINAL color profile — the teatest
> goldens pin TERM=xterm-256color, so the SGR codes are ANSI256 `38;5;N`
> derived by `x/ansi` v0.11.8 `Convert256` (to6Cube levels
> 0x00/0x5f/0x87/0xaf/0xd7/0xff with v<48→0, v<115→1, else (v-35)/40;
> exact-cube early return; else the grey index (avg-3)/10 with avg>238→23,
> grey=8+10*idx, avg=(r+g+b)/3; DistanceHSLuv cube-vs-grey tie-break, the
> cube winning ties); opentui always emits 24-bit SGR. The TOKEN hex chain
> is exact by construction (S0.2 golden matrix, bit-identical port) — only
> the SGR ENCODING follows the terminal profile: a truecolor terminal gets
> 24-bit SGR directly. The S8 pty-capture diff must run under a truecolor
> TERM to compare against the upstream captures (whose SGR is always
> 24-bit).

- [ ] **Step 5: PROGRESS.md — the one-line status pointer**

Replace the `**Status (…)**` line (S0 landed; the prior-release tail keeps):
`**Status (2026-08-25):** TUI parity S0 landed on `new_tui` — theme engine
(33 embedded upstream themes, the resolveTheme/generateSystem ports, OSC
11/10/4 detection, custom discovery + SIGUSR2, the config>KV>default
selection chain over the TUI KV) + the app-shell restyle (logo, borders,
home list, footer, session chrome — teatest SGR goldens under the pinned
ANSI256 env); deviations 122–125 logged. Next: the S1 detail pass (Slice
Detail Protocol, plan 2026-08-24-opencode-tui-parity) then S1 execution.
Prior: v0.4.3 (PR #20)…`

- [ ] **Step 6: Commit the checkpoint**

```sh
git add docs/superpowers/DEVIATIONS.md docs/superpowers/PROGRESS.md
git commit -m "docs: checkpoint — S0 done, next is S1 detail pass"
```

- [ ] **Step 7: Close the slice bead**

```sh
bd close yolo-oae.1 --reason "all 10 child beads closed, gate green, deviations 122-125 logged" --json
```

**Handoff:** report the gate result, the checkpoint commit, and `git status`. The
next step — the S1 detail pass (one subagent, `thinking=high`, per the Slice
Detail Protocol; its own bead, `docs: TUI parity plan — detail S1 tasks`) —
starts ONLY on explicit user go-ahead.
