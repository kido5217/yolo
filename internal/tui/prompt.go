package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// promptModel is the always-focused single-line input with the slash command
// menu (T25). When the value starts with "/" the filtered command menu is
// open and arrows/enter/esc drive it; "\"+enter accumulates soft-entered
// lines into draft until the final enter sends them.
type promptModel struct {
	input textinput.Model
	sel   int
	draft strings.Builder
}

// busyToast is the locked message for a send attempted while the session is
// busy, whether by the store-side pre-check or a server 409 (client.ErrBusy).
const busyToast = "abort or wait (esc aborts)"

var promptEnter = key.NewBinding(key.WithKeys("enter"))

// slashActive reports whether the slash menu is open.
func (pm *promptModel) slashActive() bool {
	v := pm.input.Value()
	return v != "" && strings.HasPrefix(v, "/")
}

// commandAliases maps canonical command names to accepted aliases. Aliases
// are input forms only: the menu surfaces the canonical name.
var commandAliases = map[string][]string{"/quit": {"/exit"}}

// matchesAlias reports whether any of the command's accepted aliases starts
// with the typed prefix (the canonical-name check stays in the caller).
func matchesAlias(c protocol.Command, prefix string) bool {
	for _, alias := range commandAliases[c.Name] {
		if strings.HasPrefix(alias[1:], prefix) {
			return true
		}
	}
	return false
}

// menuItems filters the known commands by the typed "/prefix", matching both
// canonical names and their aliases. It returns nil when the menu is closed,
// else the filtered (possibly empty) list in server order.
func (pm *promptModel) menuItems(cmds []protocol.Command) []protocol.Command {
	if !pm.slashActive() {
		return nil
	}
	prefix := pm.input.Value()[1:]
	out := []protocol.Command{}
	for _, c := range cmds {
		if len(c.Name) < 2 {
			continue // wire input: skip malformed (empty) command names
		}
		if strings.HasPrefix(c.Name[1:], prefix) || matchesAlias(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// menuView renders the filtered slash menu; each item word-wraps at the
// terminal width (custom command descriptions can be long).
func (pm *promptModel) menuView(cmds []protocol.Command, w int, th theme.Theme) string {
	items := pm.menuItems(cmds)
	if items == nil {
		return ""
	}
	if len(items) == 0 {
		return th.TextMuted().Render("  no match")
	}
	muted := th.TextMuted()
	var b strings.Builder
	for i, c := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		sty := muted
		if i == pm.sel {
			sty = cursorStyle(th)
		}
		for j, l := range strings.Split(wrapLine("  "+c.Name+"  "+c.Description, w), "\n") {
			if j > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(sty.Render(l))
		}
	}
	return b.String()
}

// view renders the prompt line (the textinput carries the "> " prompt).
func (pm *promptModel) view() string { return pm.input.View() }

// mentionActive reports whether the input has an active @-trigger (S5.4).
func (pm *promptModel) mentionActive() bool {
	_, ok := mentionTriggerIndex(pm.input.Value())
	return ok
}

// acQuery is the @-query: the value after the active @-trigger ("" when
// there is none).
func (pm *promptModel) acQuery() string {
	idx, ok := mentionTriggerIndex(pm.input.Value())
	if !ok {
		return ""
	}
	return pm.input.Value()[idx+1:]
}

// acView renders the @-picker option rows, reusing the slash-menu rendering
// (muted + cursorStyle on the selected row, each row word-wrapped). Nil opts
// hide the picker; an empty list renders the "no match" line.
func (pm *promptModel) acView(opts []selectOption, w int, th theme.Theme) string {
	if opts == nil {
		return ""
	}
	if len(opts) == 0 {
		return th.TextMuted().Render("  no match")
	}
	muted := th.TextMuted()
	var b strings.Builder
	for i, o := range opts {
		if i > 0 {
			b.WriteByte('\n')
		}
		sty := muted
		if i == pm.sel {
			sty = cursorStyle(th)
		}
		p, _ := o.value.(string)
		for j, l := range strings.Split(wrapLine("  "+p, w), "\n") {
			if j > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(sty.Render(l))
		}
	}
	return b.String()
}

// moveMenuSel moves the selection by d with wraparound (n items).
func (pm *promptModel) moveMenuSel(n, d int) {
	if n == 0 {
		pm.sel = 0
		return
	}
	pm.sel = ((pm.sel+d)%n + n) % n
}

// maxHistoryEntries caps the prompt history (the ported
// MAX_HISTORY_ENTRIES); draftRetentionMin is the trimmed-draft length a
// prompt clear retains in the history (the ported DRAFT_RETENTION_MIN_CHARS).
const (
	maxHistoryEntries = 50
	draftRetentionMin = 20
)

// recallHistory walks the prompt history by dir (-1 up/older, +1 down/newer).
// The index runs 0 (present) … -len (oldest); at present the input restores
// the draft captured on the first up-press. The ported upstream move guard:
// a recall is aborted when the input was edited away from the recall text at
// the current index and is non-empty.
func (a *App) recallHistory(dir int) {
	if len(a.hist) == 0 {
		return
	}
	if a.histIdx == 0 && dir < 0 {
		a.histOrig = a.prompt.input.Value()
	}
	input := a.prompt.input.Value()
	if a.historyTextAt(a.histIdx) != input && input != "" {
		return
	}
	next := a.histIdx + dir
	if next < -len(a.hist) || next > 0 {
		return
	}
	a.histIdx = next
	if next == 0 {
		a.prompt.input.SetValue(a.histOrig)
		a.histText = ""
		return
	}
	text := a.hist[len(a.hist)+next]
	a.prompt.input.SetValue(text)
	a.histText = text
}

// historyTextAt is the recall text at index i: present (0) → the captured
// draft (histOrig), else hist[len(hist)+i] (-1 = newest … -len = oldest).
func (a *App) historyTextAt(i int) string {
	if i == 0 {
		return a.histOrig
	}
	return a.hist[len(a.hist)+i]
}

// appendHistory records a sent (or retained) prompt in the history (the
// ported upstream append): a duplicate of the newest resets the recall to
// present without re-adding, the cap keeps the LAST maxHistoryEntries, and
// the recall state resets.
func (a *App) appendHistory(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if n := len(a.hist); n > 0 && a.hist[n-1] == text {
		a.histIdx = 0
		a.histText = ""
		return
	}
	a.hist = append(a.hist, text)
	if len(a.hist) > maxHistoryEntries {
		a.hist = a.hist[len(a.hist)-maxHistoryEntries:]
	}
	a.histIdx = 0
	a.histText = ""
	a.saveHistory()
}

// clearPrompt clears the prompt input, retaining a trimmed draft of at least
// draftRetentionMin chars in the history (the ported clearPrompt retention).
func (a *App) clearPrompt() {
	if d := strings.TrimSpace(a.prompt.input.Value()); len(d) >= draftRetentionMin {
		a.appendHistory(d)
	}
	a.prompt.input.SetValue("")
	a.histIdx = 0
	a.histText = ""
}
