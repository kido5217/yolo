// themecmds.go — the theme mode commands (S3.9): the upstream `setMode`
// === pin quirk ported verbatim (the "switch mode" command pins the
// OPPOSITE mode — it both switches and locks), the theme.mode.lock
// toggle (locked() ? free() : pin(store.mode)), and the dynamic titles
// the S4 registry + the unit tests consume (yolo has no dynamic
// command titles pre-S4). No default keybinds (upstream "none" for
// both — deviation 196; the S4.1 registry carries the defaults + the
// remap). "KV wiring" = the end-to-end chain every command rides: the
// engine's Set/Pin/Free/Apply persist to the KV file immediately (the
// S0.7 writer goroutine) and retheme picks up the new state.

package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// themeSwitchMode is the port of the upstream `setMode` (the
// setMode === pin quirk, theme.tsx, findings §2 — verbatim): pin the
// OPPOSITE of the current mode. Pin applies the mode + persists
// theme_mode_lock + theme_mode (the KV wiring); retheme refreshes the
// styles from the new mode.
func (a *App) themeSwitchMode() []tea.Cmd {
	if a.engine == nil {
		a.toast("theme engine unavailable")
		return nil
	}
	mode := a.engine.Mode()
	next := "light"
	if mode == "light" {
		next = "dark"
	}
	a.engine.Pin(next)
	a.retheme()
	return nil
}

// themeModeLock is the port of the upstream theme.mode.lock
// (theme.tsx: locked() ? free() : lock(), lock = pin(store.mode) —
// verbatim): lock pins the CURRENT mode (persists theme_mode_lock +
// theme_mode); unlock frees (clears the lock + both KV keys and
// re-resolves the mode from the cached terminal luminance). retheme
// refreshes the styles from the new mode.
func (a *App) themeModeLock() []tea.Cmd {
	if a.engine == nil {
		a.toast("theme engine unavailable")
		return nil
	}
	if a.engine.Locked() {
		a.engine.Free()
	} else {
		a.engine.Pin(a.engine.Mode())
	}
	a.retheme()
	return nil
}

// switchModeTitle is the dynamic command title for themeSwitchMode —
// the upstream registers the command with
// mode() === "dark" ? "Switch to light mode" : "Switch to dark mode":
// the title always shows the NEXT mode (the one the next press pins).
func switchModeTitle(e *theme.Engine) string {
	if e.Mode() == "dark" {
		return "Switch to light mode"
	}
	return "Switch to dark mode"
}

// modeLockTitle is the dynamic command title for themeModeLock (the
// upstream: locked() ? "Unlock theme mode" : "Lock theme mode").
func modeLockTitle(e *theme.Engine) string {
	if e.Locked() {
		return "Unlock theme mode"
	}
	return "Lock theme mode"
}
