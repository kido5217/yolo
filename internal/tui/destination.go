package tui

import (
	"os"
	"path/filepath"
	"strings"
)

// abbrevHome is the ported abbreviateHome (upstream runtime.tsx:3-10):
// "~" at the home dir, "~/rel" under it, the raw path outside (or when
// home is unknown).
func abbrevHome(dir, home string) string {
	if dir == "" || home == "" {
		return dir
	}
	rel, err := filepath.Rel(home, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
		return dir
	}
	if rel == "." {
		return "~"
	}
	return "~/" + rel
}

// homeDir is the home dir for the abbreviation (the test seam
// homeDirFunc overrides; default os.UserHomeDir).
func (a *App) homeDir() string {
	if a.homeDirFunc != nil {
		return a.homeDirFunc()
	}
	h, _ := os.UserHomeDir()
	return h
}

// sessionDestination is the new-session destination (the scope dir,
// home-abbreviated — the upstream selected ?? cwd default; the
// selection state machine has no yolo referent, deviation 236).
// "" when the scope dir is unknown (a.Service.Dir == "").
func (a *App) sessionDestination() string {
	return abbrevHome(a.Service.Dir, a.homeDir())
}

// homeShortcutsHint is the registry-rendered hint (the upstream
// which-key HomeHint text, deviation 238): the trigger is the leader
// key (the which-key overlay's opener — the upstream which_key_toggle
// is inert, deviation 207); "" when the leader binding is disabled
// (Format "none" — the overlay is then unreachable).
func (a *App) homeShortcutsHint() string {
	trigger := a.keymap.Format("leader")
	if trigger == "none" {
		return ""
	}
	return "Show keyboard shortcuts with " + trigger
}

// homeFooterLine is the home footer line (the homeModel.footer seam
// body): the S6.4 destination + S6.5 hint parts, " · "-joined (the
// help-line separator convention; each part omittable), dimmed, "" when
// both are omitted.
func (a *App) homeFooterLine(w int) string {
	var parts []string
	if d := a.sessionDestination(); d != "" {
		parts = append(parts, d)
	}
	if h := a.homeShortcutsHint(); h != "" {
		parts = append(parts, h)
	}
	if len(parts) == 0 {
		return ""
	}
	return dimWrapped(a.theme, strings.Join(parts, " \u00B7 "), w)
}
