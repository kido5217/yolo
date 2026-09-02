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

// homeFooterLine is the home footer line (the homeModel.footer seam
// body): S6.4 the destination part only; S6.5 joins the hint part.
func (a *App) homeFooterLine(w int) string {
	d := a.sessionDestination()
	if d == "" {
		return ""
	}
	return dimWrapped(a.theme, d, w)
}
