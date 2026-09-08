package tui

import (
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// homeKeyMap is the home-route key set. Up/Down are SHARED bindings (the
// slash menu, @-picker and prompt history recall all match them — keys.go);
// the 0.8.0 start screen has no list to navigate, so handleHomeKey no longer
// consumes up/down (they fall through to the prompt history recall, Task 4).
var homeKeyMap = struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	NewSess key.Binding
}{
	Up:      key.NewBinding(key.WithKeys("up")),
	Down:    key.NewBinding(key.WithKeys("down")),
	Enter:   key.NewBinding(key.WithKeys("enter")),
	NewSess: key.NewBinding(key.WithKeys("n")),
}

func nowMillis() int64 { return time.Now().UnixMilli() }

// homeBoxMaxWidth is the 0.8.0 prompt box maxWidth (home.tsx:36 default 75) —
// the tip box shares the width (tips.tsx:27). homeBoxPad is the box interior
// paddingLeft/Right (prompt/index.tsx:1361-1362; Task 5's interior content
// seam reads it).
const (
	homeBoxMaxWidth = 75
	homeBoxPad      = 2
)

// boxWidth is the prompt box width (homeBoxMaxWidth clamped to the content
// width; it reads a.size itself — the house idiom, a.termWidth).
func (a *App) boxWidth() int {
	contentW := a.termWidth() - 4
	if contentW < 0 {
		contentW = 0
	}
	boxW := min(homeBoxMaxWidth, contentW)
	if boxW < 1 {
		boxW = 1
	}
	return boxW
}

// boxInnerWidth is the box interior width in display cols (boxW-1-
// 2*homeBoxPad, min 1 — the Task-4 constants).
func (a *App) boxInnerWidth() int {
	w := a.boxWidth() - 1 - 2*homeBoxPad
	if w < 1 {
		w = 1
	}
	return w
}

// boxHighlight is the border/agent-segment color token (upstream
// prompt/index.tsx:1288-1293 highlight()): the leader pending state wins
// ("border"), then the shell mode ("primary"), else the agent-color referent
// (the "secondary" token — the mock's #5c9cf5).
func (a *App) boxHighlight() string {
	switch {
	case a.pendingLeader:
		return "border"
	case a.prompt.mode == "shell":
		return "primary"
	default:
		return "secondary"
	}
}

// boxThemeReady reports whether the box interior can paint (a resolved
// theme with a visible backgroundElement token); a zero Theme (the whitebox
// tests) degrades every box run to plain text — the house zero-theme
// convention (the SGR bytes would break the width-exact plain assertions).
func (a *App) boxThemeReady() bool {
	c, ok := a.theme.Color("backgroundElement")
	return ok && c.A != 0
}

// boxInterior renders an interior run s: the fg token (when non-empty) +
// the backgroundElement bg; a zero Theme degrades to the plain text.
func (a *App) boxInterior(fgToken, s string) string {
	bg, ok := a.theme.Color("backgroundElement")
	if !ok || bg.A == 0 {
		return s
	}
	st := lipgloss.NewStyle().Background(lipgloss.Color(bg.Hex()[:7]))
	if fgToken != "" {
		if c, ok := a.theme.Color(fgToken); ok && c.A != 0 {
			st = st.Foreground(lipgloss.Color(c.Hex()[:7]))
		}
	}
	return st.Render(s)
}

// boxFg renders a fg-only run s (no bg — the ▀ bottom edge); a zero Theme
// degrades to the plain text.
func (a *App) boxFg(fgToken, s string) string {
	if c, ok := a.theme.Color(fgToken); ok && c.A != 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()[:7])).Render(s)
	}
	return s
}

// boxBorder renders one border rune (the ┃ left edge / the ╹ bottom edge) in
// the highlight token (upstream prompt/index.tsx:1309 borderHighlight); a
// zero Theme degrades to the plain glyph.
func (a *App) boxBorder(ch string) string {
	if c, ok := a.theme.Color(a.boxHighlight()); ok && c.A != 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()[:7])).Render(ch)
	}
	return ch
}

// boxCursor renders the cursor cell: the char with Reverse(true) — the same
// reverse-block idiom the rest of the app's static cursor uses (the bubbles
// cursor.View, Blink=false), no explicit fg/bg (the terminal default pen
// supplies them); a zero Theme degrades to the plain char.
func (a *App) boxCursor(ch string) string {
	if !a.boxThemeReady() {
		return ch
	}
	return lipgloss.NewStyle().Reverse(true).Render(ch)
}

// homeBox renders the 5 prompt-box rows (mock rows mockBoxTop..+4):
//
//	row0 border + interior fill
//	row1 border + 2 pad + the input line (boxInputLine) + the 2-col
//	     right pad
//	row2 border + fill
//	row3 border + 2 pad + the meta line (boxMetaLine) + the 2-col
//	     right pad
//	row4 the corner (╹, the highlight color) + the bottom (▀ x boxW-1,
//	     fg backgroundElement)
//
// The interior runs (fill/pad/text/meta) carry the backgroundElement bg; the
// border/corner fg is the highlight token (boxHighlight) with no bg. Every
// interior cell is a styled run — NO unstyled gap inside the box: every row
// is width-exact at boxW display cols from boxL (the content rows' right pad
// is emitted, never left to the frame padding — the unpainted 2-col
// right-edge chip; pinned by TestHomeBoxRightEdge). A zero Theme degrades
// to plain runs.
func (a *App) homeBox() []string {
	boxW := a.boxWidth()
	rowFill := a.boxInterior("", strings.Repeat(" ", boxW-1))
	pad := a.boxInterior("", strings.Repeat(" ", homeBoxPad))
	rightPad := a.boxInterior("", strings.Repeat(" ", homeBoxPad))
	border := a.boxBorder("┃")
	bottom := a.boxBorder("╹")
	return []string{
		border + rowFill,
		border + pad + a.boxInputLine() + rightPad,
		border + rowFill,
		border + pad + a.boxMetaLine() + rightPad,
		bottom + a.boxFg("backgroundElement", strings.Repeat("▀", boxW-1)),
	}
}

// boxInputLine renders the value/placeholder at the interior width (a custom
// render — NOT input.View(): bubbles v2.2.1's View/placeholderView render
// Width+1 display cols (an off-by-one padding quirk) and the textinput's
// scroll offset is not exported; the box needs width-exact rows, the mock
// contract):
//
//	value == "" -> the placeholder (fg textMuted, bg backgroundElement) +
//	               fill; NO cursor cell (the mock contract: the placeholder
//	               state shows no reverse block — upstream's placeholder
//	               cursor is same-styled and the mock is the visual contract)
//	value != "" -> pre (fg text, bg) + the cursor cell + post (fg text, bg)
//	               + fill; the cursor cell = the char at input.Position()
//	               (a " " when at the end), the boxCursor reverse block.
//	Scroll: when the value's DISPLAY width (runeWidth) > innerW, the visible
//	window is innerW columns starting at column min(posCols, valueW-innerW)
//	where posCols = runeWidth(value[:pos]) — the value end-anchored when the
//	cursor is at/near the end, the cursor kept in view when moved left (the
//	session route's own textinput scroll is a richer referent; the box
//	surface pins this simpler window — logged as a deviation, Task 12).
//	Width-exact: pre+cursor+post+fill = innerW display cols, every cell a
//	styled run (no unstyled gap). It computes innerW itself from the size —
//	the Task-4 constants; it takes no width argument.
func (a *App) boxInputLine() string {
	innerW := a.boxInnerWidth()
	value := a.prompt.input.Value()
	if value == "" {
		ph := a.prompt.placeholderText()
		if len(ph) > innerW {
			ph, _ = cutWidth(ph, innerW)
		}
		return a.boxInterior("textMuted", ph) + a.boxInterior("", strings.Repeat(" ", innerW-len(ph)))
	}
	pos := a.prompt.input.Position()
	vW := runeWidth(value)
	posCols := runeWidth(value[:pos])
	start := 0
	if vW > innerW {
		start = min(posCols, vW-innerW)
	}
	_, tail := cutWidth(value, start)
	window, _ := cutWidth(tail, innerW)
	rel := posCols - start
	if rel > innerW {
		rel = innerW
	}
	preW := min(rel, innerW-1)
	pre, rest := cutWidth(window, preW)
	cur, post := " ", ""
	if pos < len(value) {
		// the char at the cursor: the first rune of rest (the cursor col
		// is inside the window here — rel < innerW). At the end (pos ==
		// len(value)) the cursor block occupies the window's last cell and
		// the trailing char is pushed out (post stays "").
		r, size := utf8.DecodeRuneInString(rest)
		cur = string(r)
		post = rest[size:]
	}
	line := a.boxInterior("text", pre) + a.boxCursor(cur) + a.boxInterior("text", post)
	return line + a.boxInterior("", strings.Repeat(" ", innerW-preW-1-runeWidth(post)))
}

// homeMeta returns the meta-line segments (decision 6: NO auto word):
//
//	agent    = titlecase(pendingAgentName()) — locale.titlecase, the
//	           "build" -> "Build" referent
//	model    = the config model ref's catalog NAME (Provider.Models[mid]
//	           .Name, the modelOptions referent), falling back to the
//	           ref's modelID; an unparseable config model is the raw ref
//	           segment (no provider segment); when config has no model ref:
//	           the FIRST catalog provider's first model (modelsOf order —
//	           the upstream fallbackModel's final step); when there is no
//	           provider at all: the model+provider segments are omitted
//	           (the upstream no-currentModel shape — agent alone)
//	provider = the ref's provider ID (the mock contract: the ID, NOT the
//	           catalog name — the deviation is logged in Task 12; upstream
//	           parsed() uses name ?? id)
func (a *App) homeMeta() (agent, model, provider string) {
	// shell mode renders "Shell" alone (upstream prompt/index.tsx:1450:
	// store.mode === "shell" ? "Shell" : titlecase(agent.name)) — the
	// model/provider box is inside the normal-mode Show.
	if a.prompt.mode == "shell" {
		return "Shell", "", ""
	}
	agent = titlecase(a.pendingAgentName())
	if s, ok := a.store.Config["model"].(string); ok && s != "" {
		pid, mid, parsed := splitModelRef(s)
		if !parsed {
			return agent, s, ""
		}
		if name, found := catalogModelName(&a.store, pid, mid); found {
			return agent, name, pid
		}
		return agent, mid, pid
	}
	if len(a.store.Providers) > 0 {
		if ms := modelsOf(a.store.Providers[0]); len(ms) > 0 {
			return agent, ms[0].Name, a.store.Providers[0].ID
		}
	}
	return agent, "", ""
}

// pendingAgentName is the home pending agent: the pinned name (a.pendingAgent,
// set by Task 7's cycle) when set, else the config default (the
// store.Config["agent"] string), else "build" (the storage column default —
// internal/storage/migrate.go:23).
func (a *App) pendingAgentName() string {
	if a.pendingAgent != "" {
		return a.pendingAgent
	}
	if s, ok := a.store.Config["agent"].(string); ok && s != "" {
		return s
	}
	return "build"
}

// catalogModelName is the catalog NAME of a model ref (the
// Provider.Models[mid].Name referent — modelOptions): found when both the
// provider and the model are in the catalog.
func catalogModelName(st *store.State, pid, mid string) (string, bool) {
	for _, p := range st.Providers {
		if p.ID != pid {
			continue
		}
		if m, ok := p.Models[mid]; ok {
			return m.Name, true
		}
	}
	return "", false
}

// boxMetaLine renders the meta line at the interior width: the agent segment
// (the highlight color) + (normal mode only, when a model is resolved)
// " " plain + "·" muted + " " plain + model (text) + (when a provider is
// resolved) " " plain + provider (muted); the tail is the interior fill.
// Shell mode renders the "Shell" label alone (upstream
// prompt/index.tsx:1450-1482: the model/provider box is inside the
// normal-mode Show). Width-exact: the line is boxInnerWidth display cols
// (an over-wide line on a narrow terminal is cut, not wrapped).
func (a *App) boxMetaLine() string {
	type seg struct {
		text string
		fg   string // "" = plain (the interior bg only)
	}
	agent, model, provider := a.homeMeta()
	label := agent
	segs := []seg{{label, a.boxHighlight()}}
	if a.prompt.mode != "shell" && model != "" {
		segs = append(segs,
			seg{" ", ""},
			seg{"·", "textMuted"},
			seg{" ", ""},
			seg{model, "text"},
		)
		if provider != "" {
			segs = append(segs, seg{" ", ""}, seg{provider, "textMuted"})
		}
	}
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(a.boxInterior(s.fg, s.text))
	}
	innerW := a.boxInnerWidth()
	content := b.String()
	cw := ansiWidth(content)
	if cw > innerW {
		content, _ = ansiCutWidth(content, innerW)
		cw = innerW
	}
	if cw < innerW {
		content += a.boxInterior("", strings.Repeat(" ", innerW-cw))
	}
	return content
}

// homeHintLine renders the hint row (mock row mockHintRow, origin at the box
// LEFT border col, upstream prompt/index.tsx:1655-1690):
//
//	normal mode: {Format("agent_cycle")} {word}  {Format("command_list")}
//	commands — the first segment is context-aware (S6, spec §3.2): the
//	word is " complete" while the slash menu is open, " agents" when
//	closed (the shortcut stays Format("agent_cycle") = tab); the words' fg
//	is textMuted, the shortcuts' fg text, the 2-col gap (box gap 2). A
//	"none" Format (a user-disabled binding) drops its segment; both none ->
//	blank row.
//	shell mode: `esc` (fg text) + ` exit shell mode` (fg textMuted) — the
//	upstream color split (the yolo-dhf.2 note's "(esc muted)" is a
//	shorthand; the strict-copy bar pins the upstream source).
func (a *App) homeHintLine() string {
	if a.prompt.mode == "shell" {
		return a.boxFg("text", "esc") + a.boxFg("textMuted", " exit shell mode")
	}
	var b strings.Builder
	seg := func(name, word string) {
		f := a.keymap.Format(name)
		if f == "none" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("  ") // the 2-col gap
		}
		b.WriteString(a.boxFg("text", f))
		b.WriteString(a.boxFg("textMuted", word))
	}
	// S6: the first segment's word is context-aware — "tab complete" while
	// the slash menu is open, "tab agents" when closed.
	agentWord := " agents"
	if a.prompt.slashActive() {
		agentWord = " complete"
	}
	seg("agent_cycle", agentWord)
	seg("command_list", " commands")
	return b.String()
}

// homeFooterContentRow renders the 0.8.0 frame footer content row (the
// 3-row footer block's middle row): the dir (muted) at col 2 =
// a.sessionDestination() + ":"+a.branch when a.branch != "" (decision 4: the
// ":" with NO space), the plain-semver version (muted) right-aligned ending
// at col w-2 (left col w-2-len(ver)); when the dir would collide with the
// version (dir width > w-2-len(ver)-2-3) the dir is silently cut at
// w-2-len(ver)-5 (no ellipsis — the cutWidth house convention); the version
// is omitted (no right segment) when empty. The returned string is the
// content (NOT padded to w — homeView pads the row).
func (a *App) homeFooterContentRow(w int) string {
	dir := ""
	if a.Service.Dir != "" {
		dir = a.sessionDestination()
		if a.branch != "" {
			dir += ":" + a.branch
		}
	}
	ver := plainSemver(a.version)
	verCol := 0
	if ver != "" {
		verCol = w - 2 - len(ver)
		if verCol < 0 {
			verCol = 0
		}
	}
	if ver != "" && dir != "" && 2+runeWidth(dir) > verCol-3 {
		cut := verCol - 5
		if cut < 0 {
			cut = 0
		}
		dir, _ = cutWidth(dir, cut)
	}
	var b strings.Builder
	if dir != "" {
		b.WriteString("  " + dir)
	}
	if ver != "" {
		pad := verCol - b.Len()
		for i := 0; i < pad; i++ {
			b.WriteByte(' ')
		}
		b.WriteString(ver)
	}
	return b.String()
}

// blankRow is one blank (w-wide) frame row.
func blankRow(w int) string { return strings.Repeat(" ", w) }

// framePadRight pads s to width w (left-aligned, the frame's row padding).
func framePadRight(s string, w int) string {
	cw := ansiWidth(s)
	if cw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-cw)
}

// frameCountLines is the line count of a (possibly multi-line) frame surface
// (0 when empty).
func frameCountLines(s string) int {
	if s == "" {
		return 0
	}
	return 1 + strings.Count(s, "\n")
}

// placeRow places content at column left and pads the right to width w (the
// frame's per-row layout; ansiWidth counts the styled content's display
// width, ansiCutWidth truncates without splitting a CSI escape).
func placeRow(left int, content string, w int) string {
	if left < 0 {
		left = 0
	}
	cw := ansiWidth(content)
	if left+cw > w {
		content, _ = ansiCutWidth(content, w-left)
		cw = w - left
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", left))
	b.WriteString(content)
	b.WriteString(strings.Repeat(" ", w-left-cw))
	return b.String()
}

// homeView renders the 0.8.0 start-screen frame: exactly size.Height rows of
// width w — top spacer, the fixed stack (4 pad, 4 logo, 1 pad, 1 box pad,
// 5 box, 1 hint, 3 tip pad, the tip rows), the overlay rows (menu, acMenu,
// perm, toasts, dlg, wk, lastErr — in that order, left-aligned per the
// existing overlay rendering), the bottom spacer, the loading line
// (deviation 237 slot), and the 3-row footer block (pad / content / pad).
// Spacers split the free rows ceil-first (top gets the odd row — the fixture
// convention). When fixed content exceeds the terminal the spacers clamp to
// 0 and the frame drops the TOP rows (the alt-screen anchor is the bottom —
// the Q9 note); the full stack fits from 24 rows up.
func (a *App) homeView(items []protocol.Command, acMenu, perm, toasts, dlg, wk string) string {
	w := a.termWidth()
	h := a.size.Height
	if h < 1 {
		h = 24
	}
	contentW := w - 4
	if contentW < 0 {
		contentW = 0
	}
	logoPad := 2 + (contentW-logoWidth+1)/2
	if logoPad < 0 {
		logoPad = 0
	}
	boxW := a.boxWidth()
	boxL := 2 + (contentW-boxW+1)/2
	if boxL < 0 {
		boxL = 0
	}
	// the fixed stack (each row w-wide).
	var stack []string
	stack = append(stack, blankRow(w), blankRow(w), blankRow(w), blankRow(w)) // 4 pad
	for _, l := range strings.Split(renderLogo(a.theme), "\n") {              // 4 logo
		stack = append(stack, placeRow(logoPad, l, w))
	}
	stack = append(stack, blankRow(w)) // 1 pad
	stack = append(stack, blankRow(w)) // 1 box pad
	// the first box row's stack index (= the pre-box region height — the slash
	// dropdown's real space-above; the box itself sits at stack[boxTop]).
	boxTop := len(stack)
	for _, r := range a.homeBox() { // 5 box
		stack = append(stack, placeRow(boxL, r, w))
	}
	stack = append(stack, placeRow(boxL, a.homeHintLine(), w))   // 1 hint
	stack = append(stack, blankRow(w), blankRow(w), blankRow(w)) // 3 tip pad
	tips := a.homeTipsRows()
	if len(tips) == 0 {
		stack = append(stack, blankRow(w)) // 1 blank tip row (hidden)
	} else {
		for _, r := range tips {
			tipL := boxL + (boxW-ansiWidth(r)+1)/2
			stack = append(stack, placeRow(tipL, r, w))
		}
	}
	// the slash dropdown anchors above the box top edge at the box's left edge /
	// width (spec §6 S3), overlaying the logo rows while open (the bottom-
	// aligned pre-box rows it occupies are painted over; the box + hint stay
	// intact below). acMenu stays on the overlay rows until the @-epic re-
	// anchors it (out of scope here).
	if rows := a.prompt.slashRows(items, boxW, boxTop, a.theme); len(rows) > 0 {
		n := len(rows)
		for i, r := range rows {
			stack[boxTop-n+i] = placeRow(boxL, r, w)
		}
	}
	// the overlay rows (left-aligned, in the session-route order).
	var overlays []string
	for _, o := range []string{acMenu, perm, toasts, dlg, wk} {
		if o != "" {
			overlays = append(overlays, strings.Split(o, "\n")...)
		}
	}
	if a.lastErr != "" {
		for _, l := range strings.Split(wrapLine("! "+a.lastErr, w), "\n") {
			overlays = append(overlays, a.theme.Error().Render(l))
		}
	}
	// the loading line (deviation 237 slot).
	loading := a.loadingView(w)
	// the 3-row footer block (pad / content / pad).
	footer := []string{blankRow(w), framePadRight(a.homeFooterContentRow(w), w), blankRow(w)}
	// the free rows (the spacers split ceil-first — top gets the odd row).
	fixedLen := len(stack) + len(overlays) + frameCountLines(loading) + 3
	free := h - fixedLen
	var topSpacer, bottomSpacer int
	if free > 0 {
		topSpacer = (free + 1) / 2
		bottomSpacer = free - topSpacer
	}
	// assemble the frame (exactly h rows).
	var frame []string
	for i := 0; i < topSpacer; i++ {
		frame = append(frame, blankRow(w))
	}
	frame = append(frame, stack...)
	frame = append(frame, overlays...)
	if loading != "" {
		frame = append(frame, loading)
	}
	for i := 0; i < bottomSpacer; i++ {
		frame = append(frame, blankRow(w))
	}
	frame = append(frame, footer...)
	if len(frame) > h {
		// fixed content exceeds the terminal: drop the TOP rows (the
		// alt-screen anchor is the bottom).
		frame = frame[len(frame)-h:]
	}
	for len(frame) < h {
		frame = append(frame, blankRow(w))
	}
	return strings.Join(frame, "\n")
}

// handleHomeKey dispatches home-route keys: enter submits the typed text
// (decision 2: non-empty text mints the session seeded with the pending
// agent + model and sends it as the first message; empty text is a no-op),
// n creates, esc clears the prompt; unhandled keys fall through to the
// prompt input (up/down recall the prompt history — the start screen has
// no list to navigate). (ctrl+c is handled app-wide in handleKey.)
func (a *App) handleHomeKey(k tea.KeyPressMsg) ([]tea.Cmd, bool) {
	switch {
	case key.Matches(k, homeKeyMap.Enter):
		return a.homeEnter(), true
	case key.Matches(k, homeKeyMap.NewSess):
		return a.emit(a.createSessionCmd()), true
	case key.Matches(k, escBinding):
		// shell mode: esc EXITS the shell (decision 3) — it does NOT clear
		// the prompt (the normal-mode behavior).
		if a.prompt.mode == "shell" {
			a.exitShellMode()
			return nil, true
		}
		a.clearPrompt()
		return nil, true
	}
	return nil, false
}

// homeEnter is the home-route enter (decision 2): the trailing-backslash
// soft-enter draft parity (the promptEnter behavior), then: empty text
// (draft+trimmed value) -> no-op; shell mode -> homeShellCmd; normal ->
// homeSubmitCmd.
func (a *App) homeEnter() []tea.Cmd {
	val := a.prompt.input.Value()
	if strings.HasSuffix(val, "\\") {
		a.prompt.draft.WriteString(strings.TrimSuffix(val, "\\") + "\n")
		a.prompt.input.SetValue("")
		return nil
	}
	text := a.prompt.draft.String() + strings.TrimSpace(val)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if a.prompt.mode == "shell" {
		return a.emit(a.homeShellCmd(text))
	}
	return a.emit(a.homeSubmitCmd(text))
}
