package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

func kvPathOf(e *theme.Engine) string { return e.KVPath() }

func TestThemeSwitchMode(t *testing.T) {
	a, e := themeApp(t)

	t.Run("switch pins the opposite mode (the setMode === pin quirk)", func(t *testing.T) {
		prev := e.Mode()
		next := "light"
		if prev == "light" {
			next = "dark"
		}
		a.themeSwitchMode()
		if e.Mode() != next || !e.Locked() {
			t.Fatalf("switch = mode %q locked %v, want %s + locked (the pin quirk)", e.Mode(), e.Locked(), next)
		}
		if got, want := switchModeTitle(e), "Switch to "+prev+" mode"; got != want {
			t.Fatalf("title = %q, want %q (the next mode)", got, want)
		}
	})

	t.Run("again -> pins back, still locked", func(t *testing.T) {
		prev := e.Mode()
		next := "light"
		if prev == "light" {
			next = "dark"
		}
		a.themeSwitchMode()
		if e.Mode() != next || !e.Locked() {
			t.Fatalf("second switch = mode %q locked %v, want %s + locked", e.Mode(), e.Locked(), next)
		}
	})
}

func TestThemeModeLock(t *testing.T) {
	a, e := themeApp(t)

	t.Run("unlocked: lock pins the current mode, then unlocks", func(t *testing.T) {
		if e.Locked() {
			t.Fatal("the fresh engine must be unlocked")
		}
		if got := modeLockTitle(e); got != "Lock theme mode" {
			t.Fatalf("title = %q, want %q", got, "Lock theme mode")
		}
		a.themeModeLock()
		if !e.Locked() || e.Mode() != "dark" {
			t.Fatalf("lock = locked %v mode %q, want locked+dark", e.Locked(), e.Mode())
		}
		if got := modeLockTitle(e); got != "Unlock theme mode" {
			t.Fatalf("title = %q, want %q", got, "Unlock theme mode")
		}
		a.themeModeLock()
		if e.Locked() {
			t.Fatal("the second press must unlock")
		}
	})
}

func TestThemeKVWiring(t *testing.T) {
	a, e := themeApp(t)

	t.Run("Set persists to the KV file and retheme follows it", func(t *testing.T) {
		names := themeOptions(e)
		target := ""
		for _, o := range names {
			if v, ok := o.value.(string); ok && v != e.Active() {
				target = v
				break
			}
		}
		if target == "" {
			t.Skip("no non-active theme")
		}
		a.openThemeListDialog()
		// move to the target (the S3.8 live preview: Set + retheme)
		for i := 0; i < len(names) && e.Active() != target; i++ {
			a.handleKey(press(tea.KeyDown))
		}
		if e.Active() != target {
			t.Fatalf("active = %s, want %s", e.Active(), target)
		}
		e.FlushKV() // barrier: the S0.7 writer is async — flush before the fresh engine reads the file
		// the KV file carries the persisted theme (the S0.7 writer has
		// flushed — the synchronous in-memory store + the writer goroutine;
		// the engine's Close flushes, but the KV read goes through the
		// in-memory store, so the file may lag: assert through the engine
		// re-read instead).
		// a fresh engine on the same KV file sees the persisted theme:
		dir := filepath.Dir(kvPathOf(e))
		e2, err := theme.New(theme.EngineOptions{
			KVPath:        filepath.Join(dir, "kv.json"),
			GlobalYoloDir: dir,
			CWD:           dir,
			Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
		})
		if err != nil {
			t.Fatalf("theme.New (second): %v", err)
		}
		defer func() { _ = e2.Close() }()
		if err := e2.Resolve(context.Background()); err != nil {
			t.Fatalf("theme.Resolve (second): %v", err)
		}
		if got := e2.Active(); got != target {
			t.Fatalf("the fresh engine sees active = %q, want %q (the KV persistence)", got, target)
		}
	})

	t.Run("Pin/Free persist the mode keys to the KV file", func(t *testing.T) {
		a.themeSwitchMode() // pins light
		if err := e.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		// the raw KV file carries theme_mode + theme_mode_lock (the writer
		// flushed on Close)
		data, err := os.ReadFile(kvPathOf(e))
		if err != nil {
			t.Fatalf("read KV: %v", err)
		}
		for _, tok := range []string{"theme_mode", "theme_mode_lock"} {
			if !strings.Contains(string(data), tok) {
				t.Fatalf("the KV file missing %q:\n%s", tok, data)
			}
		}
	})
}
