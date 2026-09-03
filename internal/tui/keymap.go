// keymap.go — the keymap registry (S4): the single source for every TUI
// binding. S4.1 ports the upstream default bindings (config/keybind.ts @
// v1.18.18) verbatim + the value-shape decoder + the seq matcher/formatter;
// S4.2 adds the Keymap (context groups, the mode stack, the leader, the
// runtime remap); S4.3 wires the yolo.jsonc keybinds schema.

package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// The upstream keymap constants (keybind.ts:41, config/index.tsx:21,
// keymap.tsx:20-21).
const (
	LeaderDefault = "ctrl+x"
	LeaderToken   = "leader"
	LeaderTimeout = 2000 * time.Millisecond
	BaseMode      = "base"
)

// BindingValue is the upstream BindingValueSchema (keybind.ts:28-33):
// false | "none" (disabled) | a sequence string | a list of items | a
// keystroke object | a binding object — carried raw (any) until
// resolveValue normalizes it.
type BindingValue any

type keybindDef struct {
	Default     BindingValue
	Description string
}

func keybind(def BindingValue, description string) keybindDef {
	return keybindDef{Default: def, Description: description}
}

// Definitions is the ported upstream default bindings (keybind.ts:45-240) —
// verbatim (names, defaults, descriptions) — plus the yolo-specific
// prompt_soft_newline display entry (deviation 208): the V1-pinned
// trailing-backslash soft-enter has no upstream referent (input_newline is
// the upstream newline binding); the entry carries the gesture for the
// registry-driven /help + which-key rendering (display-only sentinel — the
// gesture is handled by the prompt fallback, not the matcher).
var Definitions = map[string]keybindDef{
	"leader": keybind(LeaderDefault, "Leader key for keybind combinations"),

	"app_exit":                            keybind("ctrl+c,ctrl+d,<leader>q", "Exit the application"),
	"app_debug":                           keybind("none", "Toggle debug panel"),
	"app_console":                         keybind("none", "Toggle console"),
	"app_heap_snapshot":                   keybind("none", "Write heap snapshot"),
	"app_toggle_animations":               keybind("none", "Toggle animations"),
	"app_toggle_file_context":             keybind("none", "Toggle file context"),
	"app_toggle_diffwrap":                 keybind("none", "Toggle diff wrapping"),
	"app_toggle_paste_summary":            keybind("none", "Toggle paste summary"),
	"app_toggle_session_directory_filter": keybind("none", "Toggle session directory filtering"),
	"command_list":                        keybind("ctrl+p", "List available commands"),
	"help_show":                           keybind("none", "Open help dialog"),
	"docs_open":                           keybind("none", "Open documentation"),
	"diff_open":                           keybind("none", "Open diff viewer"),
	"diff_close":                          keybind("escape,q", "Close diff viewer"),
	"diff_toggle":                         keybind("enter,space", "Toggle diff viewer item"),
	"diff_expand":                         keybind("right", "Expand diff viewer item"),
	"diff_expand_all":                     keybind("E", "Expand all diff viewer folders"),
	"diff_collapse":                       keybind("left", "Collapse diff viewer item"),
	"diff_switch_focus":                   keybind("tab", "Switch diff viewer focus"),
	"diff_next_hunk":                      keybind("]", "Jump to next diff hunk"),
	"diff_previous_hunk":                  keybind("[", "Jump to previous diff hunk"),
	"diff_next_file":                      keybind("n", "Jump to next diff file"),
	"diff_previous_file":                  keybind("p", "Jump to previous diff file"),
	"diff_toggle_file_tree":               keybind("b", "Toggle diff viewer file tree"),
	"diff_single_patch":                   keybind("s", "Toggle single patch view"),
	"diff_switch_source":                  keybind("d", "Switch diff viewer source"),
	"diff_toggle_view":                    keybind("v", "Toggle diff viewer split or unified view"),
	"diff_help":                           keybind("?", "Show more diff viewer shortcuts"),

	"editor_open":       keybind("<leader>e", "Open external editor"),
	"theme_list":        keybind("<leader>t", "List available themes"),
	"theme_switch_mode": keybind("none", "Switch between light and dark theme mode"),
	"theme_mode_lock":   keybind("none", "Lock or unlock theme mode"),
	"sidebar_toggle":    keybind("<leader>b", "Toggle sidebar"),
	"scrollbar_toggle":  keybind("none", "Toggle session scrollbar"),
	"status_view":       keybind("<leader>s", "View status"),
	"debug_view":        keybind("none", "View debug info"),

	"session_export":                     keybind("<leader>x", "Export session to editor"),
	"session_copy":                       keybind("none", "Copy session transcript"),
	"session_move":                       keybind("none", "Move session"),
	"session_new":                        keybind("<leader>n", "Create a new session"),
	"session_list":                       keybind("<leader>l", "List all sessions"),
	"session_timeline":                   keybind("<leader>g", "Show session timeline"),
	"session_fork":                       keybind("none", "Fork session from message"),
	"session_rename":                     keybind("ctrl+r", "Rename session"),
	"session_delete":                     keybind("ctrl+d", "Delete session"),
	"session_share":                      keybind("none", "Share current session"),
	"session_unshare":                    keybind("none", "Unshare current session"),
	"session_interrupt":                  keybind("escape", "Interrupt current session"),
	"session_background":                 keybind("ctrl+b", "Background synchronous subagents"),
	"session_compact":                    keybind("<leader>c", "Compact the session"),
	"session_toggle_timestamps":          keybind("none", "Toggle message timestamps"),
	"session_toggle_generic_tool_output": keybind("none", "Toggle generic tool output"),
	"session_queued_prompts":             keybind("<leader>q", "Manage queued prompts"),
	"session_child_first":                keybind("<leader>down", "Go to first child session"),
	"session_child_cycle":                keybind("right", "Go to next child session"),
	"session_child_cycle_reverse":        keybind("left", "Go to previous child session"),
	"session_parent":                     keybind("up", "Go to parent session"),
	"session_pin_toggle":                 keybind("ctrl+f", "Pin or unpin session in the session list"),
	"session_quick_switch_1":             keybind("<leader>1", "Switch to session in quick slot 1"),
	"session_quick_switch_2":             keybind("<leader>2", "Switch to session in quick slot 2"),
	"session_quick_switch_3":             keybind("<leader>3", "Switch to session in quick slot 3"),
	"session_quick_switch_4":             keybind("<leader>4", "Switch to session in quick slot 4"),
	"session_quick_switch_5":             keybind("<leader>5", "Switch to session in quick slot 5"),
	"session_quick_switch_6":             keybind("<leader>6", "Switch to session in quick slot 6"),
	"session_quick_switch_7":             keybind("<leader>7", "Switch to session in quick slot 7"),
	"session_quick_switch_8":             keybind("<leader>8", "Switch to session in quick slot 8"),
	"session_quick_switch_9":             keybind("<leader>9", "Switch to session in quick slot 9"),

	"stash_delete": keybind("ctrl+d", "Delete stash entry"),

	"model_provider_list":          keybind("ctrl+a", "Open provider list from model dialog"),
	"model_favorite_toggle":        keybind("ctrl+f", "Toggle model favorite status"),
	"model_list":                   keybind("<leader>m", "List available models"),
	"model_cycle_recent":           keybind("f2", "Next recently used model"),
	"model_cycle_recent_reverse":   keybind("shift+f2", "Previous recently used model"),
	"model_cycle_favorite":         keybind("none", "Next favorite model"),
	"model_cycle_favorite_reverse": keybind("none", "Previous favorite model"),
	"mcp_list":                     keybind("none", "List MCP servers"),
	"provider_connect":             keybind("none", "Connect provider"),
	"console_org_switch":           keybind("none", "Switch console organization"),
	"agent_list":                   keybind("<leader>a", "List agents"),
	"agent_cycle":                  keybind("tab", "Next agent"),
	"agent_cycle_reverse":          keybind("shift+tab", "Previous agent"),
	"variant_cycle":                keybind("ctrl+t", "Cycle model variants"),
	"variant_list":                 keybind("none", "List model variants"),

	"messages_page_up":        keybind("pageup,ctrl+alt+b", "Scroll messages up by one page"),
	"messages_page_down":      keybind("pagedown,ctrl+alt+f", "Scroll messages down by one page"),
	"messages_line_up":        keybind("ctrl+alt+y", "Scroll messages up by one line"),
	"messages_line_down":      keybind("ctrl+alt+e", "Scroll messages down by one line"),
	"messages_half_page_up":   keybind("ctrl+alt+u", "Scroll messages up by half page"),
	"messages_half_page_down": keybind("ctrl+alt+d", "Scroll messages down by half page"),
	"messages_first":          keybind("ctrl+g,home", "Navigate to first message"),
	"messages_last":           keybind("ctrl+alt+g,end", "Navigate to last message"),
	"messages_next":           keybind("none", "Navigate to next message"),
	"messages_previous":       keybind("none", "Navigate to previous message"),
	"messages_last_user":      keybind("none", "Navigate to last user message"),
	"messages_copy":           keybind("<leader>y", "Copy message"),
	"messages_undo":           keybind("<leader>u", "Undo message"),
	"messages_redo":           keybind("<leader>r", "Redo message"),
	"messages_toggle_conceal": keybind("<leader>h", "Toggle code block concealment in messages"),
	"tool_details":            keybind("none", "Toggle tool details visibility"),
	"display_thinking":        keybind("none", "Toggle thinking blocks visibility"),

	"prompt_submit":               keybind("none", "Submit prompt"),
	"prompt_editor_context_clear": keybind("none", "Clear editor context"),
	"prompt_skills":               keybind("none", "Open skill selector"),
	"prompt_stash":                keybind("none", "Stash prompt"),
	"prompt_stash_pop":            keybind("none", "Pop stashed prompt"),
	"prompt_stash_list":           keybind("none", "List stashed prompts"),
	"workspace_set":               keybind("none", "Set workspace"),

	"input_clear":                   keybind("ctrl+c", "Clear input field"),
	"input_paste":                   keybind(map[string]any{"key": "ctrl+v", "preventDefault": false}, "Paste from clipboard"),
	"input_submit":                  keybind("return", "Submit input"),
	"input_newline":                 keybind("shift+return,ctrl+return,alt+return,ctrl+j", "Insert newline in input"),
	"input_move_left":               keybind("left,ctrl+b", "Move cursor left in input"),
	"input_move_right":              keybind("right,ctrl+f", "Move cursor right in input"),
	"input_move_up":                 keybind("up", "Move cursor up in input"),
	"input_move_down":               keybind("down", "Move cursor down in input"),
	"input_select_left":             keybind("shift+left", "Select left in input"),
	"input_select_right":            keybind("shift+right", "Select right in input"),
	"input_select_up":               keybind("shift+up", "Select up in input"),
	"input_select_down":             keybind("shift+down", "Select down in input"),
	"input_line_home":               keybind("ctrl+a", "Move to start of line in input"),
	"input_line_end":                keybind("ctrl+e", "Move to end of line in input"),
	"input_select_line_home":        keybind("ctrl+shift+a", "Select to start of line in input"),
	"input_select_line_end":         keybind("ctrl+shift+e", "Select to end of line in input"),
	"input_visual_line_home":        keybind("alt+a", "Move to start of visual line in input"),
	"input_visual_line_end":         keybind("alt+e", "Move to end of visual line in input"),
	"input_select_visual_line_home": keybind("alt+shift+a", "Select to start of visual line in input"),
	"input_select_visual_line_end":  keybind("alt+shift+e", "Select to end of visual line in input"),
	"input_buffer_home":             keybind("home", "Move to start of buffer in input"),
	"input_buffer_end":              keybind("end", "Move to end of buffer in input"),
	"input_select_buffer_home":      keybind("shift+home", "Select to start of buffer in input"),
	"input_select_buffer_end":       keybind("shift+end", "Select to end of buffer in input"),
	"input_delete_line":             keybind("ctrl+shift+d", "Delete line in input"),
	"input_delete_to_line_end":      keybind("ctrl+k", "Delete to end of line in input"),
	"input_delete_to_line_start":    keybind("ctrl+u", "Delete to start of line in input"),
	"input_backspace":               keybind("backspace,shift+backspace", "Backspace in input"),
	"input_delete":                  keybind("ctrl+d,delete,shift+delete", "Delete character in input"),
	"input_undo":                    keybind("ctrl+-,super+z", "Undo in input"),
	"input_redo":                    keybind("ctrl+.,super+shift+z", "Redo in input"),
	"input_word_forward":            keybind("alt+f,alt+right,ctrl+right", "Move word forward in input"),
	"input_word_backward":           keybind("alt+b,alt+left,ctrl+left", "Move word backward in input"),
	"input_select_word_forward":     keybind("alt+shift+f,alt+shift+right", "Select word forward in input"),
	"input_select_word_backward":    keybind("alt+shift+b,alt+shift+left", "Select word backward in input"),
	"input_delete_word_forward":     keybind("alt+d,alt+delete,ctrl+delete", "Delete word forward in input"),
	"input_delete_word_backward":    keybind("ctrl+w,ctrl+backspace,alt+backspace", "Delete word backward in input"),
	"input_select_all":              keybind("super+a", "Select all in input"),
	"history_previous":              keybind("up", "Previous history item"),
	"history_next":                  keybind("down", "Next history item"),

	"dialog.select.prev":           keybind("up,ctrl+p", "Move to previous dialog item"),
	"dialog.select.next":           keybind("down,ctrl+n", "Move to next dialog item"),
	"dialog.select.page_up":        keybind("pageup", "Move up one page in dialog"),
	"dialog.select.page_down":      keybind("pagedown", "Move down one page in dialog"),
	"dialog.select.home":           keybind("home", "Move to first dialog item"),
	"dialog.select.end":            keybind("end", "Move to last dialog item"),
	"dialog.select.submit":         keybind("return", "Submit selected dialog item"),
	"dialog.prompt.submit":         keybind("return", "Submit dialog prompt"),
	"dialog.mcp.toggle":            keybind("space", "Toggle MCP in MCP dialog"),
	"dialog.move_session.new":      keybind("ctrl+m", "New project copy"),
	"dialog.move_session.delete":   keybind("ctrl+d", "Delete project copy"),
	"dialog.move_session.refresh":  keybind("ctrl+r", "Refresh project copies"),
	"prompt.autocomplete.prev":     keybind("up,ctrl+p", "Move to previous autocomplete item"),
	"prompt.autocomplete.next":     keybind("down,ctrl+n", "Move to next autocomplete item"),
	"prompt.autocomplete.hide":     keybind("escape", "Hide autocomplete"),
	"prompt.autocomplete.select":   keybind("return", "Select autocomplete item"),
	"prompt.autocomplete.complete": keybind("tab", "Complete autocomplete item"),
	"permission.prompt.fullscreen": keybind("ctrl+f", "Toggle permission prompt fullscreen"),
	"plugins.toggle":               keybind("space", "Toggle plugin"),
	"dialog.plugins.install":       keybind("shift+i", "Install plugin from plugin dialog"),

	"terminal_suspend":      keybind("ctrl+z", "Suspend terminal"),
	"terminal_title_toggle": keybind("none", "Toggle terminal title"),
	"tips_toggle":           keybind("<leader>h", "Toggle tips on home screen"),
	"plugin_manager":        keybind("none", "Open plugin manager dialog"),
	"plugin_install":        keybind("none", "Install plugin"),

	"which_key_toggle":         keybind("ctrl+alt+k", "Toggle which-key panel"),
	"which_key_layout_toggle":  keybind("ctrl+alt+shift+k", "Switch which-key layout"),
	"which_key_pending_toggle": keybind("ctrl+alt+shift+p", "Toggle which-key pending preview"),
	"which_key_group_previous": keybind("ctrl+alt+left,ctrl+alt+[", "Previous which-key group"),
	"which_key_group_next":     keybind("ctrl+alt+right,ctrl+alt+]", "Next which-key group"),
	"which_key_scroll_up":      keybind("ctrl+alt+up,ctrl+alt+p", "Scroll which-key up"),
	"which_key_scroll_down":    keybind("ctrl+alt+down,ctrl+alt+n", "Scroll which-key down"),
	"which_key_page_up":        keybind("ctrl+alt+pageup", "Page which-key up"),
	"which_key_page_down":      keybind("ctrl+alt+pagedown", "Page which-key down"),
	"which_key_home":           keybind("ctrl+alt+home", "Jump to first which-key binding"),
	"which_key_end":            keybind("ctrl+alt+end", "Jump to last which-key binding"),

	// The yolo-specific display entry (deviation 208) — display-only.
	"prompt_soft_newline": keybind("\\+enter", "Soft-enter a newline (trailing backslash)"),
}

// CommandMap is the ported upstream binding→command map (keybind.ts:256-420)
// — verbatim.
var CommandMap = map[string]string{
	"app_exit":                            "app.exit",
	"app_debug":                           "app.debug",
	"app_console":                         "app.console",
	"app_heap_snapshot":                   "app.heap_snapshot",
	"app_toggle_animations":               "app.toggle.animations",
	"app_toggle_file_context":             "app.toggle.file_context",
	"app_toggle_diffwrap":                 "app.toggle.diffwrap",
	"app_toggle_paste_summary":            "app.toggle.paste_summary",
	"app_toggle_session_directory_filter": "app.toggle.session_directory_filter",
	"command_list":                        "command.palette.show",
	"help_show":                           "help.show",
	"docs_open":                           "docs.open",
	"diff_open":                           "diff.open",
	"diff_close":                          "diff.close",
	"diff_toggle":                         "diff.toggle",
	"diff_expand":                         "diff.expand",
	"diff_expand_all":                     "diff.expand_all",
	"diff_collapse":                       "diff.collapse",
	"diff_switch_focus":                   "diff.switch_focus",
	"diff_next_hunk":                      "diff.next_hunk",
	"diff_previous_hunk":                  "diff.previous_hunk",
	"diff_next_file":                      "diff.next_file",
	"diff_previous_file":                  "diff.previous_file",
	"diff_toggle_file_tree":               "diff.toggle_file_tree",
	"diff_single_patch":                   "diff.single_patch",
	"diff_switch_source":                  "diff.switch_source",
	"diff_toggle_view":                    "diff.toggle_view",
	"diff_help":                           "diff.help",
	"editor_open":                         "prompt.editor",
	"theme_list":                          "theme.switch",
	"theme_switch_mode":                   "theme.switch_mode",
	"theme_mode_lock":                     "theme.mode.lock",
	"sidebar_toggle":                      "session.sidebar.toggle",
	"scrollbar_toggle":                    "session.toggle.scrollbar",
	"status_view":                         "opencode.status",
	"debug_view":                          "opencode.debug",
	"session_export":                      "session.export",
	"session_copy":                        "session.copy",
	"session_move":                        "session.move",
	"session_new":                         "session.new",
	"session_list":                        "session.list",
	"session_timeline":                    "session.timeline",
	"session_fork":                        "session.fork",
	"session_rename":                      "session.rename",
	"session_delete":                      "session.delete",
	"session_share":                       "session.share",
	"session_unshare":                     "session.unshare",
	"session_interrupt":                   "session.interrupt",
	"session_background":                  "session.background",
	"session_compact":                     "session.compact",
	"session_toggle_timestamps":           "session.toggle.timestamps",
	"session_toggle_generic_tool_output":  "session.toggle.generic_tool_output",
	"session_queued_prompts":              "session.queued_prompts",
	"session_child_first":                 "session.child.first",
	"session_child_cycle":                 "session.child.next",
	"session_child_cycle_reverse":         "session.child.previous",
	"session_parent":                      "session.parent",
	"session_pin_toggle":                  "session.pin.toggle",
	"session_quick_switch_1":              "session.quick_switch.1",
	"session_quick_switch_2":              "session.quick_switch.2",
	"session_quick_switch_3":              "session.quick_switch.3",
	"session_quick_switch_4":              "session.quick_switch.4",
	"session_quick_switch_5":              "session.quick_switch.5",
	"session_quick_switch_6":              "session.quick_switch.6",
	"session_quick_switch_7":              "session.quick_switch.7",
	"session_quick_switch_8":              "session.quick_switch.8",
	"session_quick_switch_9":              "session.quick_switch.9",
	"stash_delete":                        "stash.delete",
	"model_provider_list":                 "model.dialog.provider",
	"model_favorite_toggle":               "model.dialog.favorite",
	"model_list":                          "model.list",
	"model_cycle_recent":                  "model.cycle_recent",
	"model_cycle_recent_reverse":          "model.cycle_recent_reverse",
	"model_cycle_favorite":                "model.cycle_favorite",
	"model_cycle_favorite_reverse":        "model.cycle_favorite_reverse",
	"mcp_list":                            "mcp.list",
	"provider_connect":                    "provider.connect",
	"console_org_switch":                  "console.org.switch",
	"agent_list":                          "agent.list",
	"agent_cycle":                         "agent.cycle",
	"agent_cycle_reverse":                 "agent.cycle.reverse",
	"variant_cycle":                       "variant.cycle",
	"variant_list":                        "variant.list",
	"messages_page_up":                    "session.page.up",
	"messages_page_down":                  "session.page.down",
	"messages_line_up":                    "session.line.up",
	"messages_line_down":                  "session.line.down",
	"messages_half_page_up":               "session.half.page.up",
	"messages_half_page_down":             "session.half.page.down",
	"messages_first":                      "session.first",
	"messages_last":                       "session.last",
	"messages_next":                       "session.message.next",
	"messages_previous":                   "session.message.previous",
	"messages_last_user":                  "session.messages_last_user",
	"messages_copy":                       "messages.copy",
	"messages_undo":                       "session.undo",
	"messages_redo":                       "session.redo",
	"messages_toggle_conceal":             "session.toggle.conceal",
	"tool_details":                        "session.toggle.actions",
	"display_thinking":                    "session.toggle.thinking",
	"prompt_submit":                       "prompt.submit",
	"prompt_editor_context_clear":         "prompt.editor_context.clear",
	"prompt_skills":                       "prompt.skills",
	"prompt_stash":                        "prompt.stash",
	"prompt_stash_pop":                    "prompt.stash.pop",
	"prompt_stash_list":                   "prompt.stash.list",
	"workspace_set":                       "workspace.set",
	"input_clear":                         "prompt.clear",
	"input_paste":                         "prompt.paste",
	"input_submit":                        "input.submit",
	"input_newline":                       "input.newline",
	"input_move_left":                     "input.move.left",
	"input_move_right":                    "input.move.right",
	"input_move_up":                       "input.move.up",
	"input_move_down":                     "input.move.down",
	"input_select_left":                   "input.select.left",
	"input_select_right":                  "input.select.right",
	"input_select_up":                     "input.select.up",
	"input_select_down":                   "input.select.down",
	"input_line_home":                     "input.line.home",
	"input_line_end":                      "input.line.end",
	"input_select_line_home":              "input.select.line.home",
	"input_select_line_end":               "input.select.line.end",
	"input_visual_line_home":              "input.visual.line.home",
	"input_visual_line_end":               "input.visual.line.end",
	"input_select_visual_line_home":       "input.select.visual.line.home",
	"input_select_visual_line_end":        "input.select.visual.line.end",
	"input_buffer_home":                   "input.buffer.home",
	"input_buffer_end":                    "input.buffer.end",
	"input_select_buffer_home":            "input.select.buffer.home",
	"input_select_buffer_end":             "input.select.buffer.end",
	"input_delete_line":                   "input.delete.line",
	"input_delete_to_line_end":            "input.delete.to.line.end",
	"input_delete_to_line_start":          "input.delete.to.line.start",
	"input_backspace":                     "input.backspace",
	"input_delete":                        "input.delete",
	"input_undo":                          "input.undo",
	"input_redo":                          "input.redo",
	"input_word_forward":                  "input.word.forward",
	"input_word_backward":                 "input.word.backward",
	"input_select_word_forward":           "input.select.word.forward",
	"input_select_word_backward":          "input.select.word.backward",
	"input_delete_word_forward":           "input.delete.word.forward",
	"input_delete_word_backward":          "input.delete.word.backward",
	"input_select_all":                    "input.select.all",
	"history_previous":                    "prompt.history.previous",
	"history_next":                        "prompt.history.next",
	"terminal_suspend":                    "terminal.suspend",
	"terminal_title_toggle":               "terminal.title.toggle",
	"tips_toggle":                         "tips.toggle",
	"plugin_manager":                      "plugins.list",
	"plugin_install":                      "plugins.install",
	"which_key_toggle":                    "which-key.toggle",
	"which_key_layout_toggle":             "which-key.layout.toggle",
	"which_key_pending_toggle":            "which-key.pending.toggle",
	"which_key_group_previous":            "which-key.group.previous",
	"which_key_group_next":                "which-key.group.next",
	"which_key_scroll_up":                 "which-key.scroll.up",
	"which_key_scroll_down":               "which-key.scroll.down",
	"which_key_page_up":                   "which-key.page.up",
	"which_key_page_down":                 "which-key.page.down",
	"which_key_home":                      "which-key.home",
	"which_key_end":                       "which-key.end",
}

// bindingValue is the resolved form of one binding: the matchable sequences
// (empty = disabled).
type bindingValue struct {
	enabled bool
	seqs    []string
}

// resolveValue normalizes a BindingValue into matchable sequences
// (false/"none" → disabled; a string → one seq; a list → each item; a map
// with a "key" field → a binding object; a map with a "name" field → a
// keystroke object). The object flags (event/preventDefault/fallthrough)
// have no yolo referent (deviation 209) — the matcher is press-only.
func resolveValue(v BindingValue) (bindingValue, error) {
	switch t := v.(type) {
	case nil:
		return bindingValue{}, nil
	case bool:
		if !t {
			return bindingValue{}, nil
		}
		return bindingValue{}, fmt.Errorf("invalid keybind value: %v", v)
	case string:
		if t == "" || t == "none" {
			return bindingValue{}, nil
		}
		// A string may carry multiple comma-separated sequences (the upstream
		// default format, e.g. "ctrl+c,ctrl+d,<leader>q", "escape,q");
		// split them into distinct matchable seqs ("+" is the modifier join —
		// never a sequence separator).
		var seqs []string
		for _, part := range strings.Split(t, ",") {
			s := strings.TrimSpace(part)
			if s == "" || s == "none" {
				continue
			}
			seqs = append(seqs, s)
		}
		if len(seqs) == 0 {
			return bindingValue{}, nil
		}
		return bindingValue{enabled: true, seqs: seqs}, nil
	case []any:
		out := bindingValue{enabled: true}
		for _, item := range t {
			sub, err := resolveValue(item)
			if err != nil {
				return bindingValue{}, err
			}
			out.seqs = append(out.seqs, sub.seqs...)
		}
		return out, nil
	case map[string]any:
		if key, ok := t["key"]; ok {
			switch k := key.(type) {
			case string:
				if k == "" || k == "none" {
					return bindingValue{}, nil
				}
				return bindingValue{enabled: true, seqs: []string{k}}, nil
			case map[string]any:
				ks, err := stringifyKeyStroke(k)
				if err != nil {
					return bindingValue{}, err
				}
				return bindingValue{enabled: true, seqs: []string{ks}}, nil
			default:
				return bindingValue{}, fmt.Errorf("invalid keybind object key: %T", key)
			}
		}
		if _, ok := t["name"]; ok {
			ks, err := stringifyKeyStroke(t)
			if err != nil {
				return bindingValue{}, err
			}
			return bindingValue{enabled: true, seqs: []string{ks}}, nil
		}
		return bindingValue{}, fmt.Errorf("invalid keybind object: missing key")
	default:
		return bindingValue{}, fmt.Errorf("invalid keybind value: %T", v)
	}
}

// stringifyKeyStroke is the port of the upstream stringifyKeyStroke: the
// name + the modifier flags (the KeyStroke schema field order,
// keybind.ts:8-15) joined with "+".
func stringifyKeyStroke(m map[string]any) (string, error) {
	name, _ := m["name"].(string)
	if name == "" {
		return "", fmt.Errorf("invalid keystroke: missing name")
	}
	var mods []string
	for _, field := range []string{"ctrl", "shift", "meta", "super", "hyper"} {
		if b, ok := m[field].(bool); ok && b {
			mods = append(mods, field)
		}
	}
	if len(mods) == 0 {
		return name, nil
	}
	return strings.Join(append(mods, name), "+"), nil
}

// keyNameAliases is the upstream KEY_ALIASES (keymap.tsx:112-117) — the
// matching-side normalization (both sides go through it).
var keyNameAliases = map[string]string{
	"enter":  "return",
	"esc":    "escape",
	"pgdown": "pagedown",
	"pgup":   "pageup",
}

// keyAliasesDisplay is the yolo display alias set (deviation 214): the
// upstream keyNameAliases display {pageup→pgup, pagedown→pgdn, delete→del}
// + escape→esc (the yolo surface convention — the select hint "esc close").
var keyAliasesDisplay = map[string]string{
	"pageup":   "pgup",
	"pagedown": "pgdn",
	"delete":   "del",
	"escape":   "esc",
}

// modifierAliasDisplay is the upstream modifierAliases (keymap.tsx:200-202).
var modifierAliasDisplay = map[string]string{"meta": "alt"}

func normalizeKeyName(name string) string {
	name = strings.ToLower(name)
	if a, ok := keyNameAliases[name]; ok {
		return a
	}
	return name
}

// parseSeq splits a sequence into the modifier set + the alias-normalized
// base key. The base is a single token; extra tokens after the base →
// invalid (false).
func parseSeq(seq string) (mods map[string]bool, base string, ok bool) {
	parts := strings.Split(seq, "+")
	mods = map[string]bool{}
	for i, p := range parts {
		switch strings.ToLower(p) {
		case "ctrl", "alt", "meta", "shift", "super", "hyper":
			mods[strings.ToLower(p)] = true
			continue
		default:
			if i+1 != len(parts) {
				return nil, "", false
			}
			return mods, normalizeKeyName(p), true
		}
	}
	return nil, "", false
}

// pressedBase returns the pressed key's modifier set + alias-normalized base
// name from the Keystroke() string (the fixed mod order ctrl, alt, shift,
// meta, hyper, super — the base is the last token).
func pressedBase(k tea.KeyPressMsg) (mods map[string]bool, base string) {
	mods = map[string]bool{}
	parts := strings.Split(k.Keystroke(), "+")
	for i, p := range parts {
		if i == len(parts)-1 {
			base = normalizeKeyName(p)
		} else {
			mods[strings.ToLower(p)] = true
		}
	}
	return mods, base
}

// keyMatchesSeq reports whether k matches one binding sequence (the
// <leader> token never matches raw — the pending mechanism owns it; the
// caller passes non-leader sequences).
func keyMatchesSeq(k tea.KeyPressMsg, seq string) bool {
	if strings.Contains(seq, "<"+LeaderToken+">") {
		return false
	}
	sm, sb, ok := parseSeq(seq)
	if !ok {
		return false
	}
	pm, pb := pressedBase(k)
	if sb != pb {
		return false
	}
	if len(sm) != len(pm) {
		return false
	}
	for m := range sm {
		if !pm[m] {
			return false
		}
	}
	return true
}

// leaderSplit separates a stored <leader> token: (has, rest) — rest is the
// sequence remainder after the token (the second key of the pending
// sequence).
func leaderSplit(seq string) (has bool, rest string) {
	const tok = "<" + LeaderToken + ">"
	i := strings.Index(seq, tok)
	if i < 0 {
		return false, seq
	}
	return true, strings.TrimPrefix(seq[i+len(tok):], "+")
}

// formatKeySequence is the port of the upstream formatKeySequence
// (keymap.tsx:206-208) with the yolo display aliases (deviation 214): the
// <leader> token → the resolved leader key (the remainder space-joined),
// the display aliases, the modifier alias meta→alt.
func formatKeySequence(seq, leader string) string {
	const tok = "<" + LeaderToken + ">"
	if i := strings.Index(seq, tok); i >= 0 {
		rest := strings.TrimPrefix(seq[i+len(tok):], "+")
		if rest == "" {
			return leader
		}
		return leader + " " + rest
	}
	parts := strings.Split(seq, "+")
	out := make([]string, len(parts))
	for i, p := range parts {
		low := strings.ToLower(p)
		if i == len(parts)-1 {
			if d, ok := keyAliasesDisplay[low]; ok {
				p = d
			}
		} else if d, ok := modifierAliasDisplay[low]; ok {
			p = d
		}
		out[i] = p
	}
	return strings.Join(out, "+")
}

// formatJoin is the multi-sequence display join (deviation 214: the upstream
// keymap-library join is not visible from the repo).
const formatJoin = " / "

// formatSequences renders the resolved sequences for display ("" =
// disabled → the caller renders "none").
func formatSequences(seqs []string, leader string) string {
	formatted := make([]string, 0, len(seqs))
	for _, seq := range seqs {
		formatted = append(formatted, formatKeySequence(seq, leader))
	}
	return strings.Join(formatted, formatJoin)
}

// modeEntry is one mode-stack frame (the ported createOpencodeModeStack
// entry: the identity id + the mode name).
type modeEntry struct {
	id   int
	mode string
}

// Keymap is the runtime keymap (S4.2): the resolved bindings (the single
// source for every ported upstream binding), the mode stack, and the display
// helpers. Every keypress re-reads the table, so a Set is immediately
// effective (the ported runtime-remap semantics).
type Keymap struct {
	bindings map[string]bindingValue
	modes    []modeEntry
	nextID   int
}

// NewKeymap is the port of the upstream parse (keybind.ts:449-458): the
// unknown keys error (sorted, the Go lowercase convention), then every
// default with its override. A nil/empty overrides map = the defaults.
func NewKeymap(overrides map[string]any) (*Keymap, error) {
	if overrides != nil {
		var unknown []string
		for name := range overrides {
			if _, ok := Definitions[name]; !ok {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return nil, fmt.Errorf("unrecognized keybind(s): %s", strings.Join(unknown, ", "))
		}
	}
	km := &Keymap{bindings: make(map[string]bindingValue, len(Definitions))}
	for name, def := range Definitions {
		v := def.Default
		if overrides != nil {
			if ov, ok := overrides[name]; ok {
				v = ov
			}
		}
		bv, err := resolveValue(v)
		if err != nil {
			return nil, fmt.Errorf("keybind %q: %w", name, err)
		}
		km.bindings[name] = bv
	}
	return km, nil
}

// Set is the runtime remap: it re-resolves the named binding to v (immediately
// effective — every keypress re-reads the table). An unknown name errors (the
// same unrecognized message as NewKeymap).
func (km *Keymap) Set(name string, v BindingValue) error {
	if _, ok := Definitions[name]; !ok {
		return fmt.Errorf("unrecognized keybind: %s", name)
	}
	bv, err := resolveValue(v)
	if err != nil {
		return fmt.Errorf("keybind %q: %w", name, err)
	}
	km.bindings[name] = bv
	return nil
}

// Seqs returns the named binding's matchable seqs (the <leader> seqs included;
// the caller filters — Match/MatchPending do).
func (km *Keymap) Seqs(name string) []string { return km.bindings[name].seqs }

// Match reports whether k matches any seq of the named binding (keyMatchesSeq
// already rejects the <leader> seqs — the pending mechanism owns them).
func (km *Keymap) Match(name string, k tea.KeyPressMsg) bool {
	for _, seq := range km.bindings[name].seqs {
		if keyMatchesSeq(k, seq) {
			return true
		}
	}
	return false
}

// MatchPending reports whether k matches the named binding's <leader>
// continuation (the remainder after the token — the second key of the pending
// sequence).
func (km *Keymap) MatchPending(name string, k tea.KeyPressMsg) bool {
	for _, seq := range km.bindings[name].seqs {
		has, rest := leaderSplit(seq)
		if !has || rest == "" {
			continue
		}
		if keyMatchesSeq(k, rest) {
			return true
		}
	}
	return false
}

// Format is the display form of the named binding: "none" when disabled, else
// the formatted seqs joined by the formatJoin (" / ").
func (km *Keymap) Format(name string) string {
	bv := km.bindings[name]
	if !bv.enabled || len(bv.seqs) == 0 {
		return "none"
	}
	return formatSequences(bv.seqs, km.leaderDisplay())
}

// leaderDisplay is the resolved leader key display (the "leader" binding's
// first seq; the default "ctrl+x"). It does NOT recurse through Format (which
// would be circular for the leader itself).
func (km *Keymap) leaderDisplay() string {
	bv := km.bindings["leader"]
	if !bv.enabled || len(bv.seqs) == 0 {
		return LeaderDefault
	}
	return formatKeySequence(bv.seqs[0], LeaderDefault)
}

// Current is the top mode or base (the ported createOpencodeModeStack
// current).
func (km *Keymap) Current() string {
	if n := len(km.modes); n > 0 {
		return km.modes[n-1].mode
	}
	return BaseMode
}

// Push registers a mode and returns its release func (the ported identity
// splice — it removes THAT frame by id, not by mode name).
func (km *Keymap) Push(mode string) func() {
	id := km.nextID
	km.nextID++
	km.modes = append(km.modes, modeEntry{id: id, mode: mode})
	return func() {
		for i := range km.modes {
			if km.modes[i].id == id {
				km.modes = append(km.modes[:i], km.modes[i+1:]...)
				return
			}
		}
	}
}

// contextGroups is the yolo context→binding-name groups (the upstream
// context/mode-scoped bindings have no single referent file — the groups are
// the yolo port; deviation 211). The base group is the app-level openers (any
// route, no dialog, no pending permission) in match order; the session group
// is the session-route registry keys.
var contextGroups = map[string][]string{
	BaseMode: {
		"which_key_toggle", "which_key_layout_toggle", "which_key_pending_toggle",
		"command_list", "app_exit", "model_list", "agent_list", "status_view",
		"theme_list", "session_new", "session_list", "tips_toggle", "sidebar_toggle",
	},
	"session": {
		"messages_page_up", "messages_page_down", "session_interrupt", "session_rename",
	},
}
