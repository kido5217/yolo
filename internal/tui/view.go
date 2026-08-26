package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// View renders the active route, the dialog overlay and the last error line
// into a tea.View (bubbletea v2's Model interface returns tea.View, not
// string). The plain-string composition lives in a.view() for unit testing.
// AltScreen keeps the TUI in the alternate screen buffer (v2 expresses this
// on the View, not as a program option).
func (a *App) View() tea.View {
	v := tea.NewView(a.view())
	v.AltScreen = true
	return v
}

// view composes the on-screen string: the active route, the slash menu, the
// permission overlay above the prompt, the prompt line, toasts, the dialog
// overlay, the last error line and the status footer (both routes). Each
// overlay is rendered once per frame and passed pre-built to the route
// (the session route needs it both for line counting and composition).
func (a *App) view() string {
	w := a.termWidth()
	perm := a.permissionView(w)
	toasts := a.toastsView(w)
	dlg := a.dlgView(w)
	menu := a.prompt.menuView(a.store.Commands, w, a.theme)
	var b strings.Builder
	if a.route == routeSession {
		b.WriteString(a.viewSession(menu, perm, toasts, dlg))
	} else {
		b.WriteString(a.home.render(&a.store, w, a.theme))
	}
	if menu != "" {
		b.WriteString("\n" + menu)
	}
	if perm != "" {
		b.WriteString("\n" + perm)
	}
	b.WriteString("\n" + a.prompt.view())
	if toasts != "" {
		b.WriteString("\n" + toasts)
	}
	if dlg != "" {
		b.WriteString("\n" + dlg)
	}
	if a.lastErr != "" {
		b.WriteByte('\n')
		for i, l := range strings.Split(wrapLine("! "+a.lastErr, w), "\n") {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(errRed.Render(l))
		}
	}
	b.WriteString("\n" + a.footerView())
	return b.String()
}

// viewSession renders the session route: title, the transcript viewport and
// the locked help line. The viewport reserves a line for the prompt, one for
// the footer, the open slash menu and every below-viewport overlay (perm,
// toasts, dlg, lastErr), so the frame fits the terminal height — mandatory
// under the alt screen, whose frame (unlike the normal-screen frame, which
// grows with content) is the fixed terminal size. menu/perm/toasts/dlg are
// the pre-built overlay strings from view() (rendered once per frame).
func (a *App) viewSession(menu, perm, toasts, dlg string) string {
	w := a.size.Width
	if w < 1 {
		w = 80
	}
	overlays := 0
	for _, v := range []string{perm, toasts, dlg} {
		if v != "" {
			overlays += 1 + strings.Count(v, "\n")
		}
	}
	if a.lastErr != "" {
		overlays++
	}
	menuLines := 0
	if menu != "" {
		menuLines = 1 + strings.Count(menu, "\n")
	}
	// The help line may wrap on narrow terminals; the viewport height must
	// count its real line count so the frame stays within the terminal.
	help := strings.Split(wrapLine(sessionHelp, w), "\n")
	h := a.size.Height - 1 - 1 - len(help) - 1 - 1 - menuLines - overlays
	if h < 1 {
		h = 1
	}
	a.sess.sync(&a.store, w, h, a.theme)
	t := "session"
	if a.store.Current != nil {
		t = a.store.Current.Title
	}
	var b strings.Builder
	b.WriteString(title.Render(t) +
		"\n" + a.sess.vm.View() +
		"\n" + dividerLineRendered)
	for _, l := range help {
		b.WriteString("\n" + a.theme.TextMuted().Render(l))
	}
	return b.String()
}
