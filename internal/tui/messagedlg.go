// messagedlg.go — the full-message view (S7.3): the dlgMessage modal. The
// upstream routes/session/dialog-message.tsx @ v1.18.18 is the "Message
// Actions" dialog (Revert / Copy / Fork, the mouse-clicked user message) —
// the yolo redefinition (deviation 248): the full-message view over the
// LAST message snapshot (no mouse, no revert/fork endpoints, no clipboard
// contract), opened by the yolo-surface alt+m (the sessKeyMap Expand/Think
// precedent — deviation 211's scope; alt+m is unbound by the bubbles
// v2.2.1 textinput DefaultKeyMap). The content renders the message meta +
// every part (text / reasoning / tool output), each clamped at
// msgPartMaxLines with the "… (N more lines)" hint after the head (the
// headPreview idiom). No key case — the generic esc/ctrl+c modal close
// (handleDialogKey) dismisses.

package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// msgPartMaxLines is the per-part content clamp (the headPreview idiom —
// the 10-line bash output preview): the full-message view clamps every
// part's content block so the dialog fits the panel without scrolling.
const msgPartMaxLines = 12

// openMessageDialog pushes the full-message view: the LAST message
// snapshot (the yolo referent for the upstream clicked message) in the
// dlgLarge panel. A no-op with no message (the caller consumes the key).
func (a *App) openMessageDialog() {
	if len(a.store.Messages) == 0 {
		return
	}
	m := a.store.Messages[len(a.store.Messages)-1]
	a.pushModal(dialog{kind: dlgMessage, message: &m}, dlgLarge, nil)
}

// messageHeaderRow is the dialog header: the bold "Message" left, the
// muted "esc" right, space-between at the panel width (the statusHeaderRow
// idiom).
func (a *App) messageHeaderRow(w int, th theme.Theme) string {
	const t = "Message"
	pad := w - runeWidth(t) - runeWidth("esc")
	if pad < 0 {
		pad = 0
	}
	return title.Render(t) + strings.Repeat(" ", pad) + th.TextMuted().Render("esc")
}

// messageView renders the full-message dialog (the modal stack draws the
// panel chrome): the header row, the meta line (role · agent · created ·
// ↑in ↓out · $cost — the empty parts omitted), then one block per part:
// the muted header (Text / Reasoning / Tool: <name> — <title>) + the
// content word-wrapped at w, clamped at msgPartMaxLines with the hint
// after the head; a tool error renders in the Error fg.
func (a *App) messageView(m *protocol.MessageWithParts, w int, th theme.Theme) string {
	var b strings.Builder
	b.WriteString(a.messageHeaderRow(w, th))
	meta := []string{m.Info.Role}
	if m.Info.Agent != "" {
		meta = append(meta, m.Info.Agent)
	}
	meta = append(meta, time.UnixMilli(m.Info.Time.Created).Format("15:04:05"))
	if m.Info.Tokens != nil {
		meta = append(meta, "↑"+strconv.FormatInt(m.Info.Tokens.Input, 10)+" ↓"+strconv.FormatInt(m.Info.Tokens.Output, 10))
	}
	if m.Info.Cost > 0 {
		meta = append(meta, fmt.Sprintf("$%.2f", m.Info.Cost))
	}
	b.WriteString("\n" + th.TextMuted().Render(strings.Join(meta, " · ")))
	for _, p := range m.Parts {
		var header, content string
		switch p.Type {
		case "text":
			header, content = "Text", p.Text
		case "reasoning":
			header, content = "Reasoning", p.Text
		case "tool":
			header = "Tool: " + p.Tool
			if p.State != nil {
				if p.State.Title != "" {
					header += " — " + p.State.Title
				}
				content = p.State.Output
			}
		default:
			continue
		}
		b.WriteString("\n\n" + th.TextMuted().Render(header))
		if content != "" {
			// wrapLine is a single-line wrapper (strings.Fields field-splits
			// on \n): pre-split the content and wrap each line (the
			// writePlain idiom) before clamping.
			var rows []string
			for _, l := range strings.Split(content, "\n") {
				rows = append(rows, strings.Split(wrapLine(l, w), "\n")...)
			}
			if len(rows) > msgPartMaxLines {
				overflow := len(rows) - msgPartMaxLines
				rows = rows[:msgPartMaxLines]
				for _, r := range rows {
					b.WriteString("\n" + th.Text().Render(r))
				}
				b.WriteString("\n" + th.TextMuted().Render("… ("+strconv.Itoa(overflow)+" more lines)"))
			} else {
				for _, r := range rows {
					b.WriteString("\n" + th.Text().Render(r))
				}
			}
		}
		if p.Type == "tool" && p.State != nil && p.State.Error != "" {
			b.WriteString("\n" + th.Error().Render("error: "+p.State.Error))
		}
	}
	return b.String()
}
