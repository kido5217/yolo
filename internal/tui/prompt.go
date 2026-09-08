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
	// slashDone suppresses the slash menu after a tab completion (the S6
	// close mechanism, spec §3.2): the completed input keeps the "/" prefix,
	// so the value-derived slashActive alone would leave the menu open; the
	// next input change re-arms it (the upstream onInput re-derivation,
	// autocomplete.tsx:676-695).
	slashDone bool
	// mode is the prompt input mode ("normal" | "shell" — the 0.8.0 box
	// state; the toggles that change it land in Task 10).
	mode string
	// placeholderIdx is shared across both placeholder pools (the upstream
	// single store.placeholder counter — each pool wraps by its own length).
	placeholderIdx int
}

// placeholder pools (upstream home.tsx:17-20 verbatim + the prefixes from
// prompt/index.tsx:1314-1321): the normal-mode and shell-mode placeholder
// strings.
var (
	placeholderNormal = []string{
		`Ask anything... "Fix a TODO in the codebase"`,
		`Ask anything... "What is the tech stack of this project?"`,
		`Ask anything... "Fix broken tests"`,
	}
	placeholderShell = []string{
		`Run a command... "ls -la"`,
		`Run a command... "git status"`,
		`Run a command... "pwd"`,
	}
)

// placeholderText is the active-mode placeholder: the shell pool in shell
// mode, else the normal pool, at placeholderIdx % len(pool) (the shared
// index wraps each pool by its own length).
func (pm *promptModel) placeholderText() string {
	pool := placeholderNormal
	if pm.mode == "shell" {
		pool = placeholderShell
	}
	return pool[pm.placeholderIdx%len(pool)]
}

// rollPlaceholder re-rolls the placeholder index over the active-mode pool
// (the repickTip idiom — the app's random seam; the active pool is the shell
// pool in shell mode, else the normal pool). Called from enterHome and the
// shell-mode toggle (Task 10).
func (a *App) rollPlaceholder() {
	pool := placeholderNormal
	if a.prompt.mode == "shell" {
		pool = placeholderShell
	}
	a.prompt.placeholderIdx = int(a.tipRand() * float64(len(pool)))
}

// enterShellMode is the `!` toggle-in (upstream prompt/index.tsx:830-837):
// re-roll the placeholder index over the shell pool and switch the mode; the
// input value is KEPT (a cursor-0 non-empty value becomes the command — the
// upstream referent). The mode is set BEFORE the re-roll so rollPlaceholder
// reads the shell pool.
func (a *App) enterShellMode() {
	a.prompt.mode = "shell"
	a.rollPlaceholder()
	a.applyPromptChrome()
}

// exitShellMode is the shell-mode exit (decision 3): back to normal with the
// NORMAL placeholder — NO re-roll (the index persists; the decision's
// re-rolls are home-entry + `!` only) — via the applyPromptChrome placeholder
// swap.
func (a *App) exitShellMode() {
	a.prompt.mode = "normal"
	a.applyPromptChrome()
}

// busyToast is the locked message for a send attempted while the session is
// busy, whether by the store-side pre-check or a server 409 (client.ErrBusy).
const busyToast = "abort or wait (esc aborts)"

var promptEnter = key.NewBinding(key.WithKeys("enter"))

// slashActive reports whether the slash menu is open: the value starts with
// "/" and the menu is not tab-completed (slashDone suppresses it until the
// next input change re-arms it — the S6 close mechanism, spec §3.2).
func (pm *promptModel) slashActive() bool {
	if pm.slashDone {
		return false
	}
	v := pm.input.Value()
	return v != "" && strings.HasPrefix(v, "/")
}

// commandAliases maps canonical command names to accepted aliases. Aliases
// are input forms only: the menu surfaces the canonical name.
var commandAliases = map[string][]string{"/quit": {"/exit"}}

// menuItems is the /-picker (S5.5 — the ported upstream /-autocomplete): the
// command-name-frecency-ranked merged commands. Nil when the menu is closed;
// an empty query (input == "/") returns the merged list frecency-ranked
// (deviation 226, extended per spec §3.3 — the order becomes the frecency
// rank); otherwise fuzzy.Find over the canonical + alias names, each score
// x2 for a prefix match and x (1 + command-name frecency) (the S4 fold,
// spec §3.3), sorted desc, capped at maxPickerOptions, deduped by the
// canonical name (an alias match maps to the canonical command).
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
	now := nowMillis()
	// Per-call command-name frecency index: O(1) lookup per command (mirrors
	// mentionOptions' per-call frecency index over the @-picker paths).
	fidx := make(map[string]*frecencyEntry, len(a.cmdFreq))
	for i := range a.cmdFreq {
		fidx[a.cmdFreq[i].Path] = &a.cmdFreq[i]
	}
	type scored struct {
		cmd   protocol.Command
		score float64
	}
	var ranked []scored
	if q := a.prompt.input.Value()[1:]; q == "" {
		// deviation 226 (extended per spec §3.3): the empty query lists all
		// merged, frecency-ranked.
		ranked = make([]scored, len(cmds))
		for i, c := range cmds {
			ranked[i] = scored{cmd: c, score: frecencyScore(fidx[c.Name], now)}
		}
	} else {
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
		seen := make(map[string]bool, len(cmds))
		for _, m := range fuzzy.Find(q, names) {
			c := byName[m.Str]
			if seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			s := float64(m.Score) // positive-score gate (threshold 0)
			if strings.HasPrefix(m.Str[1:], q) {
				s *= 2
			}
			s *= 1 + frecencyScore(fidx[c.Name], now)
			ranked = append(ranked, scored{cmd: c, score: s})
		}
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

// slashRows renders the slash menu's surface for a placement (width w, the
// space-above clamp): the bordered dropdown's rows (the matches case) or the
// single no-match line (the empty case). Nil (the menu is closed) returns no
// rows. The rows are the dropdown's view, split; the caller owns the vertical
// placement (bottom-aligned to the anchor's top edge). S3 owns the anchors —
// the placement supplies the real width + spaceAbove (home: the box's left
// edge / width and the pre-box rows; session: the terminal width and the
// viewport rows), so the space-above clamp is real, not a no-op.
func (pm *promptModel) slashRows(items []protocol.Command, w, spaceAbove int, th theme.Theme) []string {
	if items == nil {
		return nil
	}
	if len(items) == 0 {
		return []string{th.TextMuted().Render("  No matching items")}
	}
	rows := make([]dropdownRow, len(items))
	for i, c := range items {
		rows[i] = dropdownRow{label: c.Name, description: c.Description}
	}
	d := newDropdown(rows, pm.sel, w, spaceAbove, th)
	if d.vis == 0 {
		return nil
	}
	return strings.Split(d.view(), "\n")
}

// menuView renders the slash menu's box (the standalone render the unit pins
// use): the box chrome, the selection row (primary bg + the SelectedForeground),
// the over-wide row truncated at the box content width (no wrap). The no-match
// line keeps the current text (the "No matching items" text lands in S7). The
// space-above clamp is the menu's own height (a no-op at this standalone
// placement — S3's placement supplies the real spaceAbove via slashRows).
func (pm *promptModel) menuView(items []protocol.Command, w int, th theme.Theme) string {
	return strings.Join(pm.slashRows(items, w, len(items), th), "\n")
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
		// the recall is an input change: re-arm a tab-completed menu (S6).
		a.prompt.slashDone = false
		a.histText = ""
		return
	}
	text := a.hist[len(a.hist)+next]
	a.prompt.input.SetValue(text)
	// the recall is an input change: re-arm a tab-completed menu (S6).
	a.prompt.slashDone = false
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
