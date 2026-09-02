package tui

import (
	"sort"
	"strings"
)

// whichKeyEntry is one row of the which-key overlay (the port of the upstream
// Entry, which-key.tsx:66-71): the continuation key, the binding's display
// label, its category group and the continuation flag (deviation 213).
type whichKeyEntry struct {
	key       string
	label     string
	group     string
	continues bool
}

// whichKeyGroup is a category bucket (the port of the upstream Group,
// which-key.tsx:74-77).
type whichKeyGroup struct {
	label   string
	entries []whichKeyEntry
}

// whichKeyCategory maps a binding name to its overlay group by its prefix
// (deviation 213 — the upstream command category is not a yolo field; the
// binding-name prefix is the yolo referent).
func whichKeyCategory(name string) string {
	switch {
	case strings.HasPrefix(name, "which_key"):
		return "Keymap"
	case strings.HasPrefix(name, "app_"), strings.HasPrefix(name, "sidebar_"):
		return "App"
	case strings.HasPrefix(name, "command_"):
		return "Commands"
	case strings.HasPrefix(name, "help_"):
		return "Help"
	case strings.HasPrefix(name, "diff_"):
		return "Diff"
	case strings.HasPrefix(name, "editor_"):
		return "Editor"
	case strings.HasPrefix(name, "theme_"):
		return "Theme"
	case strings.HasPrefix(name, "status_"):
		return "Status"
	case strings.HasPrefix(name, "session_"):
		return "Session"
	case strings.HasPrefix(name, "stash_"):
		return "Stash"
	case strings.HasPrefix(name, "model_"):
		return "Model"
	case strings.HasPrefix(name, "mcp_"):
		return "MCP"
	case strings.HasPrefix(name, "provider_"):
		return "Provider"
	case strings.HasPrefix(name, "agent_"):
		return "Agent"
	case strings.HasPrefix(name, "messages_"):
		return "Messages"
	case strings.HasPrefix(name, "prompt_"):
		return "Prompt"
	case strings.HasPrefix(name, "input_"):
		return "Input"
	case strings.HasPrefix(name, "workspace_"):
		return "Workspace"
	case strings.HasPrefix(name, "dialog."), strings.HasPrefix(name, "permission."):
		return "Dialog"
	default:
		return "Other"
	}
}

// whichKeyGrouped buckets the entries by group (the port of the upstream
// grouped, which-key.tsx:144-156): groups sorted by label; entries within a
// group sorted continues-desc, then label, then key.
func whichKeyGrouped(entries []whichKeyEntry) []whichKeyGroup {
	m := map[string][]whichKeyEntry{}
	var order []string
	for _, e := range entries {
		if _, ok := m[e.group]; !ok {
			order = append(order, e.group)
		}
		m[e.group] = append(m[e.group], e)
	}
	sort.Strings(order)
	out := make([]whichKeyGroup, 0, len(order))
	for _, label := range order {
		es := m[label]
		sort.Slice(es, func(i, j int) bool {
			if es[i].continues != es[j].continues {
				return es[j].continues
			}
			if es[i].label != es[j].label {
				return es[i].label < es[j].label
			}
			return es[i].key < es[j].key
		})
		out = append(out, whichKeyGroup{label: label, entries: es})
	}
	return out
}

// whichKeyEntries returns the held leader's continuation bindings for the
// current context (S4.6): the enabled bindings in the current mode's context
// group whose sequence carries the <leader> prefix, one entry per binding
// (the continuation key + the binding description + the prefix-derived
// category). The overlay lists what the held leader can dispatch here
// (deviation 207(3): the context filter keeps the inert unwired bindings out).
func (km *Keymap) whichKeyEntries() []whichKeyEntry {
	leader := km.bindings["leader"]
	if !leader.enabled || len(leader.seqs) == 0 {
		return nil
	}
	var out []whichKeyEntry
	for _, name := range contextGroups[km.Current()] {
		if name == "leader" {
			continue
		}
		bv := km.bindings[name]
		if !bv.enabled {
			continue
		}
		for _, seq := range bv.seqs {
			has, rest := leaderSplit(seq)
			if !has {
				continue
			}
			out = append(out, whichKeyEntry{
				key:       formatKeySequence(rest, km.leaderDisplay()),
				label:     Definitions[name].Description,
				group:     whichKeyCategory(name),
				continues: false,
			})
			break // one entry per binding (the first leader seq)
		}
	}
	return out
}

// whichKeyView renders the pending prefix-group overlay (S4.6): the held
// leader's continuation bindings for the current context, grouped by
// category. Empty when the leader is not pending, a modal dialog is open (the
// modal frame owns the frame), or the current context has no
// leader-continuation bindings.
func (a *App) whichKeyView(w int) string {
	if !a.pendingLeader {
		return ""
	}
	if d, ok := a.dlg.top(); ok && d.modal {
		return ""
	}
	entries := a.keymap.whichKeyEntries()
	if len(entries) == 0 {
		return ""
	}
	groups := whichKeyGrouped(entries)
	var lines []string
	lines = append(lines, a.theme.TextMuted().Render("Leader ("+a.keymap.leaderDisplay()+")"))
	for _, g := range groups {
		var parts []string
		for _, e := range g.entries {
			parts = append(parts, e.key+" "+e.label)
		}
		body := "  " + g.label + ": " + strings.Join(parts, "   ")
		for _, l := range strings.Split(wrapLine(body, w), "\n") {
			lines = append(lines, a.theme.TextMuted().Render(l))
		}
	}
	return strings.Join(lines, "\n")
}
