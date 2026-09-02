package tui

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/kido5217/yolo/internal/tui/theme"
)

// noModelsTip is the no-provider nudge (upstream NO_MODELS_TIP,
// tips-view.tsx:71, verbatim).
const noModelsTip = "Run {highlight}/connect{/highlight} to add an AI provider and start coding"

// tips is the ported tips set (S6.2 — root principle 3 pinned data; the
// pin records THIS ported content, the deviation 234 reduction). Order =
// the upstream TIPS order (tips-view.tsx:164-283) filtered to the yolo
// referent set. {highlight}…{/highlight} marks the bright runs; the
// <binding> and {theme_count} tokens are substituted at render time
// (tipText, S6.3).
var tips = []string{
	"Type {highlight}@{/highlight} followed by a filename to fuzzy search and reference files",
	"Use {highlight}/model{/highlight} or {highlight}<model_list>{/highlight} to switch between available AI models",
	"Use {highlight}/themes{/highlight} or {highlight}<theme_list>{/highlight} to switch between {theme_count} built-in themes",
	"Use {highlight}/new{/highlight} or {highlight}<session_new>{/highlight} to start a fresh conversation session",
	"Use {highlight}/sessions{/highlight} or {highlight}<session_list>{/highlight} to list and continue sessions",
	"Press {highlight}<command_list>{/highlight} to see all available actions and commands",
	"Run {highlight}/connect{/highlight} to add API keys for supported LLM providers",
	"The leader key is {highlight}<leader>{/highlight}; combine with other keys for quick actions",
	"Use {highlight}<messages_page_up>{/highlight}/{highlight}<messages_page_down>{/highlight} to navigate through conversation history",
	"Press {highlight}<prompt_soft_newline>{/highlight} to add newlines in your prompt",
	"Press {highlight}<session_interrupt>{/highlight} to stop the AI mid-response",
	"Switch to {highlight}Plan{/highlight} agent for suggestions without making changes",
	"Create {highlight}yolo.jsonc{/highlight} for server and TUI settings",
	"Place your global settings in {highlight}~/.config/yolo/{/highlight}",
	"Configure {highlight}model{/highlight} in config to set your default model",
	"Override any keybind in {highlight}yolo.jsonc{/highlight} via the {highlight}keybinds{/highlight} section",
	"Set any keybind to {highlight}none{/highlight} to disable it completely",
	"Configure per-agent permissions for {highlight}edit{/highlight}, {highlight}bash{/highlight}, and {highlight}glob{/highlight} tools",
	`Use patterns like {highlight}"git *": "allow"{/highlight} for granular bash permissions`,
	`Set {highlight}"rm -rf *": "deny"{/highlight} to block destructive commands`,
	`Configure {highlight}"git push": "ask"{/highlight} to require approval before pushing`,
	"Run {highlight}yolo serve{/highlight} for headless API access to the core server",
	"Run {highlight}yolo auth list{/highlight} to see all configured providers",
	`Use {highlight}"theme": "system"{/highlight} to match your terminal's colors`,
	"Create JSON theme files in the {highlight}.yolo/themes/{/highlight} directory",
	"Themes support dark/light variants for both modes",
	"Use numeric xterm color codes 0-255 in custom theme JSON",
	"Use {highlight}{env:VAR_NAME}{/highlight} for environment variables in config",
	"Use {highlight}instructions{/highlight} in config to load additional rules files",
	"Permission {highlight}doom_loop{/highlight} prevents infinite tool call loops",
	"Permission {highlight}external_directory{/highlight} protects files outside project",
	"Set {highlight}YOLO_PRINT_LOGS=1{/highlight} to see detailed logs in stderr",
	"Use {highlight}/status{/highlight} or {highlight}<status_view>{/highlight} to see system status info",
	"Use {highlight}/connect{/highlight} with OpenCode Zen for curated, tested models",
	"Commit your project's {highlight}AGENTS.md{/highlight} file to Git for team sharing",
	"Use {highlight}/help{/highlight} to show the help dialog",
	"Press {highlight}<session_rename>{/highlight} to rename the current session",
}

// tipBindings is the <binding> token set the tips templates may use
// (tipText substitutes each with keymap.Format(name); the integrity test
// enforces both directions).
var tipBindings = []string{
	"model_list", "theme_list", "session_new", "session_list", "command_list",
	"leader", "messages_page_up", "messages_page_down", "prompt_soft_newline",
	"session_interrupt", "status_view", "session_rename",
}

// themeCount is the {theme_count} token value (the upstream themeCount,
// the built-in theme count — the yolo referent theme.AllThemes()).
var themeCount = func() int {
	m, err := theme.AllThemes()
	if err != nil {
		return 0
	}
	return len(m)
}()

var (
	tipHighlightRe = regexp.MustCompile(`\{highlight\}(.+?)\{/highlight\}`)
	tipTokenRe     = regexp.MustCompile(`<([a-z_]+)>`)
)

// tipPart is one run of a parsed tip: the text + the highlight flag
// (the bright run; the rest renders muted).
type tipPart struct {
	text string
	hi   bool
}

// parseTip splits the {highlight}…{/highlight} markup into its parts
// (upstream parse(), tips-view.tsx:47-66): the highlighted parts are the
// bright runs, the rest the muted.
func parseTip(s string) []tipPart {
	var parts []tipPart
	last := 0
	for _, m := range tipHighlightRe.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > last {
			parts = append(parts, tipPart{s[last:m[0]], false})
		}
		parts = append(parts, tipPart{s[m[2]:m[3]], true})
		last = m[1]
	}
	if last < len(s) {
		parts = append(parts, tipPart{s[last:], false})
	}
	return parts
}

// tipWord is one word of a tips line tagged with its run kind (0 the
// "● Tip " prefix, 1 muted, 2 the highlighted text).
type tipWord struct {
	word string
	kind int
}

// tipRun is one styled run of a visual tips line; the runs stay in
// SEQUENCE (the parts interleave — unlike rowLine's fixed-order
// buckets, joinTipLine merges consecutive same-kind words in order).
type tipRun struct {
	text string
	kind int
}

type tipLine struct {
	runs []tipRun
}

// tipLines wraps the "● Tip " prefix + the parsed parts at w with the
// rowLines word-wrap contract (word boundaries, over-long tokens
// hard-split at the width, single-space rejoin). The prefix word carries
// NO trailing space (the join space is the sole separator — the rowLines
// contract: no word carries a trailing space, deviation 242).
func tipLines(prefix string, parts []tipPart, w int) []tipLine {
	words := []tipWord{{strings.TrimSuffix(prefix, " "), 0}}
	for _, p := range parts {
		kind := 1
		if p.hi {
			kind = 2
		}
		for _, f := range strings.Fields(p.text) {
			words = append(words, tipWord{f, kind})
		}
	}
	plain := prefix
	for _, p := range parts {
		plain += p.text
	}
	if w < 1 || plain == "" {
		return []tipLine{{runs: []tipRun{{prefix, 0}}}}
	}
	effW := w
	if effW < 1 {
		effW = 1
	}
	var (
		lines []tipLine
		cur   []tipWord
		curW  int
	)
	flush := func() {
		if len(cur) == 0 {
			return
		}
		lines = append(lines, joinTipLine(cur))
		cur, curW = cur[:0], 0
	}
	for _, wd := range words {
		fw := runeWidth(wd.word)
		if fw > effW {
			flush()
			for rest := wd.word; len(rest) > 0; {
				chunk, r := cutWidth(rest, effW)
				lines = append(lines, joinTipLine([]tipWord{{chunk, wd.kind}}))
				rest = r
			}
			continue
		}
		switch {
		case len(cur) == 0:
			cur, curW = append(cur, wd), fw
		case curW+1+fw <= effW:
			cur, curW = append(cur, wd), curW+1+fw
		default:
			flush()
			cur, curW = append(cur, wd), fw
		}
	}
	flush()
	return lines
}

// joinTipLine joins one visual line's tagged words into its in-sequence
// runs (a join space belongs to the PRECEDING word's run; a
// line-boundary boundary drops it, the rowLine contract).
func joinTipLine(ws []tipWord) tipLine {
	var l tipLine
	for i, wd := range ws {
		r := tipRun{text: wd.word, kind: wd.kind}
		if i < len(ws)-1 {
			r.text += " "
		}
		if n := len(l.runs); n > 0 && l.runs[n-1].kind == wd.kind {
			l.runs[n-1].text += r.text
		} else {
			l.runs = append(l.runs, r)
		}
	}
	return l
}

// writeTipLine renders one visual line's runs: kind 0 the warning
// prefix, 1 muted, 2 the bright text.
func writeTipLine(b *strings.Builder, l tipLine, th theme.Theme) {
	for _, r := range l.runs {
		switch r.kind {
		case 0:
			b.WriteString(th.Warning().Render(r.text))
		case 2:
			b.WriteString(th.Text().Render(r.text))
		default:
			b.WriteString(th.TextMuted().Render(r.text))
		}
	}
}

// tipText is the token-substituted tip text (the <binding> tokens →
// keymap.Format, {theme_count} → the theme count), or the NO_MODELS
// force when !connected (the upstream connected === false force).
func (a *App) tipText() string {
	if !a.tipsConnected() {
		return noModelsTip
	}
	t := tips[a.tipIdx%len(tips)]
	for _, b := range tipBindings {
		t = strings.ReplaceAll(t, "<"+b+">", a.keymap.Format(b))
	}
	return strings.ReplaceAll(t, "{theme_count}", strconv.Itoa(themeCount))
}

// homeTipsLine is the home tips line (the homeModel.tips seam body):
// "" when hidden (the upstream (!first || !connected) && !hidden gate).
func (a *App) homeTipsLine(w int) string {
	if !a.tipsVisible() {
		return ""
	}
	lines := tipLines("● Tip ", parseTip(a.tipText()), w)
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
			b.WriteString("●")
		}
		writeTipLine(&b, l, a.theme)
	}
	return b.String()
}
