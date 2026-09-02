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
	if d, ok := a.dlg.top(); ok && d.modal {
		return a.viewModal()
	}
	w := a.termWidth()
	perm := a.permissionView(w)
	toasts := a.toastsView(w)
	dlg := a.dlgView(w)
	menu := a.prompt.menuView(a.mergedCommands(), w, a.theme)
	wk := a.whichKeyView(w)
	var b strings.Builder
	if a.route == routeSession {
		b.WriteString(a.viewSession(menu, perm, toasts, dlg, wk))
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
	if wk != "" {
		b.WriteString("\n" + wk)
	}
	if a.lastErr != "" {
		b.WriteByte('\n')
		for i, l := range strings.Split(wrapLine("! "+a.lastErr, w), "\n") {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(a.theme.Error().Render(l))
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
func (a *App) viewSession(menu, perm, toasts, dlg, wk string) string {
	w := a.size.Width
	if w < 1 {
		w = 80
	}
	overlays := 0
	for _, v := range []string{perm, toasts, dlg, wk} {
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
	help := len(strings.Split(wrapLine(sessionHelp, w), "\n"))
	vh := a.size.Height - 1 - 1 - help - 1 - 1 - menuLines - overlays
	return a.sessionChrome(w, vh)
}

// modalChromeMin is the route chrome's minimum line count (the panel top
// never climbs above it): session = title + 1 viewport + divider + help,
// home = logo + new-session + divider + help.
func (a *App) modalChromeMin() int {
	switch a.route {
	case routeSession:
		return 1 + 1 + 1 + len(strings.Split(wrapLine(sessionHelp, a.termWidth()), "\n"))
	default:
		return 4 + 1 + 1 + len(strings.Split(wrapLine(helpText, a.termWidth()), "\n"))
	}
}

// sessionChrome renders the session route's chrome for a viewport of vh
// lines: title, transcript viewport, divider, the (possibly wrapped) help.
func (a *App) sessionChrome(w, vh int) string {
	if vh < 1 {
		vh = 1
	}
	a.sess.sync(&a.store, w, vh, a.theme, a.spinFrame())
	t := "session"
	if a.store.Current != nil {
		t = a.store.Current.Title
	}
	var b strings.Builder
	b.WriteString(title.Render(t) +
		"\n" + a.sess.vm.View() +
		"\n" + dividerLineRendered)
	for _, l := range strings.Split(wrapLine(sessionHelp, w), "\n") {
		b.WriteString("\n" + a.theme.TextMuted().Render(l))
	}
	return b.String()
}

// viewModal renders the modal frame (port of dialog.tsx): the route chrome
// clamped to the panel top, plain blank backdrop lines (deviation 166 —
// the upstream rgba(0,0,0,0.15) dim has no SGR equivalent), the centered
// panel (backgroundPanel fill, width min(size, w-2), top padding 1, top at
// max(h/4, chromeMin)) and the footer on the last line. Prompt, menu,
// toasts and lastErr are suppressed while a modal is open.
func (a *App) viewModal() string {
	w, h := a.size.Width, a.size.Height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	d, _ := a.dlg.top()
	panelW := int(d.size.width())
	if panelW > w-2 {
		panelW = w - 2
	}
	innerLines := strings.Split(a.modalInner(&d, panelW, h), "\n")
	panelTop := max(h/4, a.modalChromeMin())
	avail := h - panelTop - 1 // the footer line
	if avail < 1 {
		avail = 1
	}
	n := min(len(innerLines)+1, avail) // +1: the panel top-padding line
	var chrome string
	switch a.route {
	case routeSession:
		help := len(strings.Split(wrapLine(sessionHelp, w), "\n"))
		chrome = a.sessionChrome(w, panelTop-1-1-help)
	default:
		help := len(strings.Split(wrapLine(helpText, w), "\n"))
		chrome = a.home.renderClamped(&a.store, w, a.theme, panelTop-4-1-1-help)
	}
	chromeLines := strings.Split(chrome, "\n")
	for len(chromeLines) < panelTop {
		chromeLines = append(chromeLines, "")
	}
	if len(chromeLines) > panelTop {
		chromeLines = chromeLines[:panelTop]
	}
	bg := a.theme.BackgroundPanel().Width(panelW)
	panel := []string{bg.Render("")}
	for i := 0; i < n-1 && i < len(innerLines); i++ {
		panel = append(panel, bg.Render(innerLines[i]))
	}
	lead := strings.Repeat(" ", (w-panelW)/2)
	var b strings.Builder
	write := func(l string) {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
	}
	for _, l := range chromeLines {
		write(l)
	}
	for _, l := range panel {
		write(lead + l)
	}
	for i := panelTop + len(panel); i < h-1; i++ {
		write("")
	}
	write(a.footerView())
	return b.String()
}
