// statusdlg.go — the /status dialog (S3.5): a static modal listing the
// provider + agent status. The upstream dialog-status.tsx header + per-section
// shape (the count header row, the status-colored bullet + the bold name + the
// muted detail, the "No X" fallback) port verbatim; the sections adapt — the
// upstream MCP/LSP/formatters/plugins have no yolo wire endpoints, so the
// content is Providers + Agents (deviation 193). No session section (the
// footer owns the session status).

package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// openStatusDialog pushes the status modal (S3.5): a static view (no payload).
func (a *App) openStatusDialog() []tea.Cmd {
	a.pushModal(dialog{kind: dlgStatus}, dlgMedium, nil)
	return nil
}

// statusHeaderRow is the dialog header: the bold "Status" left, the muted
// "esc" right, space-between at the panel width.
func (a *App) statusHeaderRow(w int, th theme.Theme) string {
	const t = "Status"
	pad := w - runeWidth(t) - runeWidth("esc")
	if pad < 0 {
		pad = 0
	}
	return title.Render(t) + strings.Repeat(" ", pad) + th.TextMuted().Render("esc")
}

// statusView renders the status dialog (the modal stack draws the panel
// chrome): the header row, then the Providers + Agents sections. Each section
// has a count header (text token) and a bullet row per item (the
// status-colored bullet + the bold name + the muted detail), or the "No X"
// fallback when empty.
func (a *App) statusView(w, h int, th theme.Theme) string {
	var b strings.Builder
	b.WriteString(a.statusHeaderRow(w, th))

	// Providers: the status-colored bullet via providerStatus (loaded→success,
	// missing→error, else→textMuted) + the bold name.
	provs := a.store.Providers
	if len(provs) == 0 {
		b.WriteString("\n" + th.Text().Render("No Providers"))
	} else {
		b.WriteString("\n" + th.Text().Render(strconv.Itoa(len(provs))+" Providers"))
		for _, p := range provs {
			label, st := providerStatus(th, p.Auth)
			b.WriteString("\n" + st.Render(label) + " " + title.Render(p.Name))
		}
	}

	// Agents: the success bullet + the bold name + the muted description
	// (wrapped at w-4; the continuation lines indent under the name).
	agents := a.store.Agents
	if len(agents) == 0 {
		b.WriteString("\n" + th.Text().Render("No Agents"))
	} else {
		b.WriteString("\n" + th.Text().Render(strconv.Itoa(len(agents))+" Agents"))
		muted := th.TextMuted()
		for _, ag := range agents {
			prefix := th.Success().Render("•") + " " + title.Render(ag.Name) + " "
			pw := runeWidth("•") + 1 + runeWidth(ag.Name) + 1
			if ag.Description == "" {
				b.WriteString("\n" + strings.TrimRight(prefix, " "))
				continue
			}
			descW := w - 4 - pw
			if descW < 1 {
				descW = 1
			}
			for i, l := range strings.Split(wrapLine(ag.Description, descW), "\n") {
				if i == 0 {
					b.WriteString("\n" + prefix + muted.Render(l))
				} else {
					b.WriteString("\n" + strings.Repeat(" ", pw) + muted.Render(l))
				}
			}
		}
	}
	return b.String()
}
