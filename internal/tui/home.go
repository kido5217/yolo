package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

// boxBorder renders one border rune (the ┃ left edge / the ╹ bottom edge) in
// the secondary token (the upstream agent/border color, prompt/index.tsx:1309
// borderHighlight = the agent color at full fade); a zero Theme degrades to
// the plain glyph.
func (a *App) boxBorder(ch string) string {
	if c, ok := a.theme.Color("secondary"); ok && c.A != 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()[:7])).Render(ch)
	}
	return ch
}

// boxFill renders the box interior fill in the backgroundElement token
// (the upstream prompt interior, prompt/index.tsx:1350-1512); a zero Theme
// degrades to the plain text.
func (a *App) boxFill(s string) string {
	if c, ok := a.theme.Color("backgroundElement"); ok && c.A != 0 {
		return lipgloss.NewStyle().Background(lipgloss.Color(c.Hex()[:7])).Render(s)
	}
	return s
}

// homeBox renders the 5 box rows (the 0.8.0 prompt box, mock rows 24..28 at
// 200x50): the ┃ left border (rows 0..3) + the ╹ bottom edge (row 4), the
// backgroundElement interior fill, and the blank interior (Task 4 — the
// boxInputLine seam is the Task-5 placeholder, empty here). It reads a.size
// itself (the house idiom, a.termWidth).
func (a *App) homeBox() []string {
	w := a.termWidth()
	contentW := w - 4
	if contentW < 0 {
		contentW = 0
	}
	boxW := min(homeBoxMaxWidth, contentW)
	if boxW < 1 {
		boxW = 1
	}
	fill := strings.Repeat(" ", boxW-1)
	border := a.boxBorder("┃")
	bottom := a.boxBorder("╹")
	return []string{
		border + a.boxFill(fill),
		border + a.boxFill(fill),
		border + a.boxFill(fill),
		border + a.boxFill(fill),
		bottom + a.boxFill(strings.Repeat("▀", boxW-1)),
	}
}

// boxInputLine is the Task-5 seam: the box placeholder row (the mode pool).
// Task 4 lands the empty-value stub so the gate is green; Task 5 replaces
// the body in place (the homeBox interior picks it up).
func (a *App) boxInputLine() string { return "" }

// homeHintLine is the Task-5 seam: the box hint row (the shortcuts line).
// Task 4 lands the blank stub; Task 5 fills it.
func (a *App) homeHintLine() string { return "" }

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
func (a *App) homeView(menu, acMenu, perm, toasts, dlg, wk string) string {
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
	boxW := min(homeBoxMaxWidth, contentW)
	if boxW < 1 {
		boxW = 1
	}
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
	for _, r := range a.homeBox() {    // 5 box
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
	// the overlay rows (left-aligned, in the session-route order).
	var overlays []string
	for _, o := range []string{menu, acMenu, perm, toasts, dlg, wk} {
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

// handleHomeKey dispatches home-route keys: enter creates (Task 4 — the
// start screen has no list to select from, so enter always creates; Task 6
// rewrites it to the decision-2 submit), n creates, esc clears the prompt;
// unhandled keys fall through to the prompt input (up/down now recall the
// prompt history — the start screen has no list to navigate). (ctrl+c is
// handled app-wide in handleKey.)
func (a *App) handleHomeKey(k tea.KeyPressMsg) ([]tea.Cmd, bool) {
	switch {
	case key.Matches(k, homeKeyMap.Enter):
		return a.homeEnter(), true
	case key.Matches(k, homeKeyMap.NewSess):
		return a.emit(a.createSessionCmd()), true
	case key.Matches(k, escBinding):
		a.clearPrompt()
		return nil, true
	}
	return nil, false
}

// homeEnter creates a new session (Task 4 — the old cursor-0 "New session"
// path; the start screen has no list to select from, so enter always
// creates). Task 6 rewrites this to the decision-2 submit (the prompt text).
func (a *App) homeEnter() []tea.Cmd {
	return a.emit(a.createSessionCmd())
}
