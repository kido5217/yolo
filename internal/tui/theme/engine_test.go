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
	copy(p[:], hex16[:])
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
// "yolo" (theme.tsx:121-122, spec §3) across the full cfg × kv matrix.
func TestEngineSelectionChain(t *testing.T) {
	dark := paletteFunc(testPalette("#000000", "#ffffff"), true)
	cases := []struct {
		name string
		cfg  string
		kv   string // "" = key absent
		want string
	}{
		{"cfg empty, kv absent", "", "", "yolo"},
		{"cfg empty, kv valid", "", "kanagawa", "kanagawa"},
		{"cfg empty, kv unknown", "", "ghost", "ghost"},
		{"cfg valid, kv absent", "nord", "", "nord"},
		{"cfg valid, kv valid", "nord", "kanagawa", "nord"},
		{"cfg valid, kv unknown", "nord", "ghost", "nord"},
		{"cfg unknown, kv absent", "ghostcfg", "", "ghostcfg"},
		{"cfg unknown, kv valid", "ghostcfg", "kanagawa", "ghostcfg"},
		{"cfg unknown, kv unknown", "ghostcfg", "ghostkv", "ghostcfg"},
		{"cfg legacy opencode, kv absent", "opencode", "", "opencode"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
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
	if th.Name != "yolo" {
		t.Errorf("ActiveTheme().Name = %q, want yolo (memo fallback)", th.Name)
	}

	// The legacy default name (pre-rebrand "opencode") is unknown after
	// the rename: the stored selection keeps it; the resolved theme
	// degrades to the default (the renamed asset, the same palette).
	dir, kvPath = engineDir(t)
	seedKV(t, kvPath, `{"theme":"opencode"}`)
	e = newTestEngine(t, EngineOptions{
		KVPath: kvPath, GlobalYoloDir: dir, CWD: dir,
		ConfigTheme: "opencode", Palette: dark,
	})
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := e.Active(); got != "opencode" {
		t.Fatalf("Active() = %q, want opencode (legacy stored selection)", got)
	}
	legacy, err := e.ActiveTheme()
	if err != nil {
		t.Fatalf("ActiveTheme: %v", err)
	}
	if legacy.Name != "yolo" {
		t.Fatalf("ActiveTheme().Name = %q, want yolo (legacy-name fallback)", legacy.Name)
	}
}

// TestEngineModeChain: mode = lock > terminal luminance > "dark"
// (theme.tsx:117, 165; S0 scoping rule: the luminance half applies in
// Resolve, after the palette probe).
func TestEngineModeChain(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
// empty → no "system" + active "system" falls back to "yolo" (the
// upstream catch path, theme.tsx:159-163, 174-178).
func TestEngineSystemTheme(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
// custom appears; a corrupt file is the error path: active "yolo",
// customs emptied, error returned (the theme.tsx:132-144 catch).
func TestEngineRefreshCustoms(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
