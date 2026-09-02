package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"github.com/sahilm/fuzzy"

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

// menuItems is the /-picker (S5.5 — the ported upstream /-autocomplete): the
// fuzzy-ranked merged commands. Nil when the menu is closed; an empty query
// (input == "/") returns the merged list in order (deviation 226); otherwise
// fuzzy.Find over the canonical + alias names, each score x2 for a prefix
// match, sorted desc, capped at maxPickerOptions, deduped by the canonical
// name (an alias match maps to the canonical command).
func (a *App) menuItems() []protocol.Command {
	if !a.prompt.slashActive() {
		return nil
	}
	merged := a.mergedCommands()
	cmds := make([]protocol.Command, 0, len(merged))
	for _, c := range merged {
		if len(c.Name) < 2 {
			continue // wire input: skip malformed (empty) command names
		}
		cmds = append(cmds, c)
	}
	if len(cmds) == 0 {
		return []protocol.Command{}
	}
	q := a.prompt.input.Value()[1:]
	if q == "" {
		return cmds // deviation 226: the empty query lists all merged, in order
	}
	names := make([]string, 0, len(cmds))
	byName := make(map[string]protocol.Command, len(cmds))
	for _, c := range cmds {
		byName[c.Name] = c
		names = append(names, c.Name)
		for _, alias := range commandAliases[c.Name] {
			byName[alias] = c
			names = append(names, alias)
		}
	}
	type scored struct {
		cmd   protocol.Command
		score int
	}
	seen := make(map[string]bool, len(cmds))
	var ranked []scored
	for _, m := range fuzzy.Find(q, names) {
		c := byName[m.Str]
		if seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		s := m.Score
		if strings.HasPrefix(m.Str[1:], q) {
			s *= 2
		}
		ranked = append(ranked, scored{cmd: c, score: s})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > maxPickerOptions {
		ranked = ranked[:maxPickerOptions]
	}
	out := make([]protocol.Command, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, r.cmd)
	}
	return out
}

// menuView renders the slash menu's items directly (S5.5: the filtering is
// App.menuItems); each item word-wraps at the terminal width (custom command
// descriptions can be long).
func (pm *promptModel) menuView(items []protocol.Command, w int, th theme.Theme) string {
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
