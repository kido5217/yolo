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
	// an unresolvable home degrades to "" (abbrevHome then returns the
	// raw dir — no abbreviation, no error surface).
	h, _ := os.UserHomeDir()
	return h
}

// sessionDestination is the new-session destination (the scope dir,
// home-abbreviated — the upstream selected ?? cwd default; the
// selection state machine has no yolo referent, deviation 236).
// "" when the scope dir is unknown (a.Service.Dir == ""). The 0.8.0 home
// footer dir segment reuses it (App.homeFooterContentRow appends the
// ":branch" suffix, decision 4).
func (a *App) sessionDestination() string {
	return abbrevHome(a.Service.Dir, a.homeDir())
}
