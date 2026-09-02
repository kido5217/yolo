// themedlg.go — the /themes picker (S3.8): the select over the engine's
// AllThemes (case-insensitive sort), the live-preview Set + retheme on
// move/filter/select (the upstream theme.set — persists immediately), and
// the initial-theme restore on filter-clear and on esc (the upstream
// onCleanup).

package tui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// themeDlg is the theme picker's payload: the select, the live theme
// accessor, the dialog's initial (active-at-open) theme name and the
// confirm flag (the upstream onCleanup gate: esc without a select
// restores the initial). th is a live accessor, not the at-open capture
// (the 197(c) convention): the upstream dialog renders inside the theme
// context, so a preview move re-themes the dialog's own rows — an at-open
// capture would keep the list in the stale palette while the preview
// switches.
type themeDlg struct {
	sel       *selectModel
	th        func() theme.Theme
	initial   string
	confirmed bool
}

// handleKey drives the select (the modal stack consumes esc/ctrl+c first).
func (d *themeDlg) handleKey(a *App, k tea.KeyPressMsg) []tea.Cmd {
	if d.sel == nil {
		return nil
	}
	return d.sel.handleKey(a, k)
}

// view renders the select (the modal stack draws the panel chrome).
func (d *themeDlg) view(w, h int) string {
	if d.sel == nil {
		return title.Render("Themes") + "\n" + d.th().TextMuted().Render("  loading…")
	}
	return d.sel.view(w, h, d.th())
}

// themeOptions is the select's option list: the engine's AllThemes keys
// (builtins + customs + "system") sorted case-insensitively (the upstream
// localeCompare port).
func themeOptions(e *theme.Engine) []selectOption {
	all := e.AllThemes()
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	opts := make([]selectOption, len(names))
	for i, name := range names {
		opts[i] = selectOption{title: name, value: name}
	}
	return opts
}

// openThemeListDialog pushes the /themes picker: the select over the
// engine's AllThemes (case-insensitive sort) with the current-theme
// marker, the live-preview Set + retheme (the upstream theme.set), the
// filter restore (empty → the initial + re-anchor; non-empty → the first
// match) and the initial-theme restore on esc (the upstream onCleanup).
func (a *App) openThemeListDialog() []tea.Cmd {
	if a.engine == nil {
		a.toast("theme engine unavailable")
		return nil
	}
	initial := a.engine.Active()
	opts := themeOptions(a.engine)
	d := &themeDlg{
		th:      func() theme.Theme { return a.theme },
		initial: initial,
	}
	sel := selectNew("Themes", "Search", opts,
		func(o selectOption) bool {
			v, _ := o.value.(string)
			return v == a.engine.Active()
		},
		func(app *App, o selectOption) {
			name, _ := o.value.(string)
			if app.engine.Set(name) {
				app.retheme()
			}
			d.confirmed = true
			app.closeTopModal()
		},
		func(o selectOption) {
			name, _ := o.value.(string)
			if a.engine.Set(name) {
				a.retheme()
			}
		})
	sel.skipFilter = true
	sel.onFilter = func(needle string) {
		if needle == "" {
			sel.options = opts
			if a.engine.Set(initial) {
				a.retheme()
			}
			for i, o := range opts {
				if v, _ := o.value.(string); v == initial {
					sel.sel = i
					return
				}
			}
			sel.sel = 0
			return
		}
		kept := make([]selectOption, 0, len(opts))
		for _, o := range opts {
			if strings.Contains(strings.ToLower(o.title), strings.ToLower(needle)) {
				kept = append(kept, o)
			}
		}
		sel.options = kept
		if len(kept) == 0 {
			return // no match: no Set
		}
		sel.sel = 0
		if v, ok := kept[0].value.(string); ok && a.engine.Set(v) {
			a.retheme()
		}
	}
	d.sel = sel
	a.pushModal(dialog{kind: dlgThemes, themes: d}, dlgMedium, func(*App) {
		if !d.confirmed {
			if a.engine.Set(d.initial) {
				a.retheme()
			}
		}
	})
	return nil
}
