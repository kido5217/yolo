package tui

import (
	"regexp"

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
