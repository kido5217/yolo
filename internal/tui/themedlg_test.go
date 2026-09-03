package tui

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// themeApp wires a REAL engine (the S0.7 KV + engine chain) into the app —
// the newRecApp helper hardcodes the nil engine, so this builds the recApp
// directly (the home_theme_test.go pattern).
func themeApp(t *testing.T) (*recApp, *theme.Engine) {
	t.Helper()
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
	ra := &recApp{App: NewApp(client.New("http://127.0.0.1:9", ""), store.State{}, "", e)}
	ra.emitSink = func(cmds ...tea.Cmd) { ra.Cmds = append(ra.Cmds, cmds...) }
	return ra, e
}

func TestThemeListDialogRender(t *testing.T) {
	a, e := themeApp(t)
	a.openThemeListDialog()
	// The 33 builtin rows need h/2-6 >= 33 visible rows: h=80 (the
	// 200(e) window precedent — h=24 shows 6 rows, the order walk
	// structurally unreachable).
	got := stripANSI(a.dlg.themes().view(80, 80))
	if !strings.Contains(got, "Themes") {
		t.Fatalf("title missing:\n%s", got)
	}
	// the options are the case-insensitively sorted AllThemes keys
	want := make([]string, 0, len(e.AllThemes()))
	for name := range e.AllThemes() {
		want = append(want, name)
	}
	sort.Slice(want, func(i, j int) bool {
		return strings.ToLower(want[i]) < strings.ToLower(want[j])
	})
	// line-based order walk: a name can be a substring of another theme's
	// name ("orng" in "lucent-orng"), so the row line (gutter-stripped)
	// is matched, not a bare substring index
	lines := strings.Split(got, "\n")
	last := -1
	for _, name := range want {
		i := -1
		for li, l := range lines {
			if strings.TrimSpace(strings.TrimLeft(l, " \u25CF")) == name {
				i = li
				break
			}
		}
		if i < 0 {
			t.Fatalf("theme %q missing:\n%s", name, got)
		}
		if i < last {
			t.Fatalf("themes not in sorted order at %q:\n%s", name, got)
		}
		last = i
	}
	// the current theme carries the marker
	if !strings.Contains(got, "●") {
		t.Fatalf("current-theme marker missing:\n%s", got)
	}
}

func TestThemeListDialogFlow(t *testing.T) {
	t.Run("move previews the theme live (Set + retheme), select confirms", func(t *testing.T) {
		a, e := themeApp(t)
		a.openThemeListDialog()
		names := themeOptions(e)
		if len(names) < 2 {
			t.Skip("need at least two themes")
		}
		other := ""
		for _, o := range names {
			if v, ok := o.value.(string); ok && v != e.Active() {
				other = v
				break
			}
		}
		if other == "" {
			t.Skip("no non-active theme")
		}
		// move to the other theme: the live preview fires (Set + retheme)
		for i := 0; i < len(names); i++ {
			a.handleKey(press(tea.KeyDown))
			if e.Active() == other {
				break
			}
		}
		if e.Active() != other {
			t.Fatalf("the live preview must Set the moved theme: active = %s, want %s", e.Active(), other)
		}
		a.handleKey(press(tea.KeyEnter))
		if !a.dlg.empty() {
			t.Fatal("select must close the dialog")
		}
		if e.Active() != other {
			t.Fatalf("select must persist the theme: active = %s", e.Active())
		}
	})

	t.Run("filter-empty restores the initial theme", func(t *testing.T) {
		a, e := themeApp(t)
		initial := e.Active()
		a.openThemeListDialog()
		// move off the initial, then clear the filter -> the initial comes back
		a.handleKey(press(tea.KeyDown))
		if e.Active() == initial {
			a.handleKey(press(tea.KeyDown))
		}
		if e.Active() == initial {
			t.Skip("could not move off the initial theme")
		}
		// select to confirm a different theme, then re-open + clear the filter
		a.handleKey(press(tea.KeyEnter))
		moved := e.Active()
		a.openThemeListDialog()
		// type then delete the filter text: the onFilter("") restores the
		// dialog's initial (the active-at-open = moved)
		a.handleKey(press('a'))
		a.handleKey(press(tea.KeyBackspace))
		if e.Active() != moved {
			t.Fatalf("filter-clear must restore the initial: active = %s, want %s", e.Active(), moved)
		}
	})

	t.Run("esc without a select restores the initial theme", func(t *testing.T) {
		a, e := themeApp(t)
		initial := e.Active()
		a.openThemeListDialog()
		a.handleKey(press(tea.KeyDown)) // preview a different theme
		if e.Active() == initial {
			a.handleKey(press(tea.KeyDown))
		}
		if e.Active() == initial {
			t.Skip("could not move off the initial theme")
		}
		a.handleKey(press(tea.KeyEscape))
		if e.Active() != initial {
			t.Fatalf("esc must restore the initial: active = %s, want %s", e.Active(), initial)
		}
	})

	t.Run("zero engine toasts and does not open", func(t *testing.T) {
		a := testApp()
		a.openThemeListDialog()
		if !a.dlg.empty() {
			t.Fatal("the zero engine must not open the dialog")
		}
		if len(a.toasts) != 1 || !strings.Contains(a.toasts[0].msg, "theme engine unavailable") {
			t.Fatalf("the engine-unavailable toast missing: %v", a.toasts)
		}
	})
}
