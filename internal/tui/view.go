package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kido5217/yolo/internal/protocol"
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

// view composes the on-screen string: the active route, the slash menu (passed
// as its items — each route anchors it per spec §6 S3: home above the box,
// session above the prompt), the permission overlay above the prompt, the
// prompt line, toasts, the dialog overlay, the last error line and the status
// footer (both routes). Each overlay is rendered once per frame and passed
// pre-built to the route (the session route counts its lines for the viewport
// height).
func (a *App) view() string {
	if d, ok := a.dlg.top(); ok && d.modal {
		return a.viewModal()
	}
	w := a.termWidth()
	perm := a.permissionView(w)
	toasts := a.toastsView(w)
	dlg := a.dlgView(w)
	items := a.menuItems()
	acMenu := ""
	if a.prompt.mentionActive() {
		acMenu = a.prompt.acView(a.mentionOptions(), w, a.theme)
	}
	wk := a.whichKeyView(w)
	// the home route owns the full frame (homeView — the overlays, the prompt
	// box, lastErr, the loading line and the footer are INSIDE the frame);
	// the session route keeps today's append-after-route composition exactly.
	if a.route != routeSession {
		return a.homeView(items, acMenu, perm, toasts, dlg, wk)
	}
	var b strings.Builder
	b.WriteString(a.viewSession(items, acMenu, perm, toasts, dlg, wk))
	if acMenu != "" {
		b.WriteString("\n" + acMenu)
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
	if line := a.loadingView(w); line != "" {
		b.WriteString("\n" + line)
	}
	b.WriteString("\n" + a.footerView())
	return b.String()
}

// viewSession renders the session route: title, the transcript viewport and
// the locked help line. The viewport reserves a line for the prompt, one for
// the footer, the open @-picker and every below-viewport overlay (perm,
// toasts, dlg, lastErr), so the frame fits the terminal height — mandatory
// under the alt screen, whose frame (unlike the normal-screen frame, which
// grows with content) is the fixed terminal size. The slash menu anchors above
// the prompt line (spec §6 S3): the viewport keeps its full height and the
// dropdown overlays the bottom transcript rows (painted over in
// sessionChrome), so it adds no frame rows. acMenu/perm/toasts/dlg/wk are the
// pre-built overlay strings from view() (rendered once per frame).
func (a *App) viewSession(items []protocol.Command, acMenu, perm, toasts, dlg, wk string) string {
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
	acMenuLines := 0
	if acMenu != "" {
		acMenuLines = 1 + strings.Count(acMenu, "\n")
	}
	// The help line may wrap on narrow terminals; the viewport height must
	// count its real line count so the frame stays within the terminal.
	help := len(strings.Split(wrapLine(sessionHelp, w), "\n"))
	vh := a.size.Height - 1 - 1 - help - 1 - 1 - acMenuLines - overlays
	// the slash dropdown overlays the viewport tail (space-above = the
	// viewport's row count — the tail it may paint over).
	overlay := a.prompt.slashRows(items, w, vh, a.theme)
	return a.sessionChrome(w, vh, overlay...)
}

// modalChromeMin is the route chrome's minimum line count (the panel top
// never climbs above it): session = title + 1 viewport + divider + help,
// home = the 0.8.0 start-screen chrome (logo 4 + box 5 + hint 1 = 10).
func (a *App) modalChromeMin() int {
	switch a.route {
	case routeSession:
		return 1 + 1 + 1 + len(strings.Split(wrapLine(sessionHelp, a.termWidth()), "\n"))
	default:
		return 4 + 5 + 1 // logo + box + hint
	}
}

// sessionChrome renders the session route's chrome for a viewport of vh
// lines: title, the transcript viewport (the todo sidebar — S7.2 — as the
// right sidebarWidth columns of the viewport lines, deviation 246), divider,
// the (possibly wrapped) help. overlay, when non-empty, is the slash dropdown
// (spec §6 S3): its rows paint over the bottom transcript rows (the viewport
// keeps its full height; the overlay rows are full-width and skip the sidebar).
func (a *App) sessionChrome(w, vh int, overlay ...string) string {
	if vh < 1 {
		vh = 1
	}
	var side []string
	leftW := w
	if a.sidebarVisible() && w > sidebarWidth {
		side = a.sidebarLines(vh)
		leftW = w - sidebarWidth
	}
	a.sess.sync(&a.store, leftW, vh, a.theme, a.spinFrame())
	t := "session"
	if a.store.Current != nil {
		t = a.store.Current.Title
	}
	var b strings.Builder
	b.WriteString(title.Render(t) + "\n")
	vmRows := strings.Split(a.sess.vm.View(), "\n")
	overlayN := len(overlay)
	if overlayN > len(vmRows) {
		overlayN = len(vmRows)
	}
	overlaidFrom := len(vmRows) - overlayN
	for i, row := range vmRows {
		if i > 0 {
			b.WriteString("\n")
		}
		if i >= overlaidFrom {
			// the slash dropdown overlay row (full width, no sidebar): the
			// dropdown paints over the bottom transcript rows.
			b.WriteString(overlay[i-overlaidFrom])
		} else {
			if side != nil && i < len(side) {
				row += side[i]
			}
			b.WriteString(row)
		}
	}
	b.WriteString("\n" + dividerLineRendered)
	for _, l := range strings.Split(wrapLine(sessionHelp, w), "\n") {
		b.WriteString("\n" + a.theme.TextMuted().Render(l))
	}
	return b.String()
}

// viewModal renders the modal frame (port of dialog.tsx): the route chrome
// clamped to the panel top, plain blank backdrop lines (deviation 166 —
// the upstream rgba(0,0,0,150/255) dim has no SGR equivalent), the centered
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
		// the 0.8.0 start-screen chrome: the top of the homeView frame
		// (the logo + box + hint region), clamped to panelTop rows below
		// (the frame's own footer is dropped by the clamp).
		chrome = a.homeView(nil, "", "", "", "", "")
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
	// the footer on the last line (the session route's status footer; the
	// home route's footer is inside the frame — the modal's last line is
	// blank).
	if a.route == routeSession {
		write(a.footerView())
	} else {
		write("")
	}
	return b.String()
}
