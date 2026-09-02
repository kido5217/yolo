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

// KVPath is the KV file path (the S0.7 persistence surface) — the
// minimal seam for the KV-wiring assertions.
func (e *Engine) KVPath() string {
	return e.opts.KVPath
}

// FlushKV synchronously flushes the KV store to the file (a barrier
// before reading the file while the S0.7 writer goroutine is still
// in flight — the promise-chain writer keeps its own schedule).
func (e *Engine) FlushKV() {
	e.kv.Flush()
}

// Close flushes the KV (pending writes drain + writer stops).
func (e *Engine) Close() error {
	return e.kv.Close()
}
