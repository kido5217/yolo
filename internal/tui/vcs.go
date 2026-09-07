package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// gitProbeEnvDropped are the env keys a repo-local or hook-set value must
// not carry into the git probe: each one can redirect the probe away from
// the scope dir (decision 4, yolo-dhf.2). GIT_TERMINAL_PROMPT is dropped too
// so the appended GIT_TERMINAL_PROMPT=0 is never shadowed by an inherited
// value (no credential prompt on a read-only probe).
var gitProbeEnvDropped = map[string]bool{
	"GIT_DIR":                 true,
	"GIT_WORK_TREE":           true,
	"GIT_INDEX_FILE":          true,
	"GIT_CEILING_DIRECTORIES": true,
	"GIT_COMMON_DIR":          true,
	"GIT_TERMINAL_PROMPT":     true,
}

// gitBranchArgs is the pinned upstream arg prefix (v1.18.18
// packages/opencode/src/git/index.ts `cfg`, verbatim): it keeps the read-only
// probe from taking locks or touching core settings.
var gitBranchArgs = []string{
	"--no-optional-locks",
	"-c", "core.autocrlf=false",
	"-c", "core.fsmonitor=false",
	"-c", "core.longpaths=true",
	"-c", "core.symlinks=true",
	"-c", "core.quotepath=false",
	"symbolic-ref", "--quiet", "--short", "HEAD",
}

// findGitRoot walks up from dir (to the filesystem root) and returns the
// nearest ancestor containing a .git entry (a file OR a directory —
// worktrees and submodules carry a .git file), ("", false) when none (incl.
// dir == "").
func findGitRoot(dir string) (string, bool) {
	for dir != "" {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
	return "", false
}

// gitBranch resolves the attached branch of the repo containing dir ("" +
// ok=false when none): it requires findGitRoot(dir) to succeed (no .git ->
// no spawn), then runs `git <gitBranchArgs>` with cwd = dir (git walks up
// itself from there), the caller-supplied env, and the timeout ctx. Exit
// table (decision 4): 0 -> the trimmed stdout ("" -> none); 1 -> detached
// HEAD -> none; 128 -> non-repo -> none; exec.ErrNotFound -> no git binary
// -> none; ctx deadline exceeded -> none; any other exit -> none. ok=false
// for every none case (the caller renders the path-only footer).
func gitBranch(ctx context.Context, dir string, timeout time.Duration, env []string) (string, bool) {
	if _, ok := findGitRoot(dir); !ok {
		return "", false
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", gitBranchArgs...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", false
	}
	return branch, true
}

// sanitizedGitEnv builds the git child env: os.Environ() minus the
// repo-redirecting GIT_* vars (GIT_DIR/GIT_WORK_TREE/GIT_INDEX_FILE/
// GIT_CEILING_DIRECTORIES/GIT_COMMON_DIR — a repo-local or hook-set GIT_*
// must not redirect the probe away from the scope dir) plus
// GIT_TERMINAL_PROMPT=0 (no credential prompt on a read-only probe; an
// inherited GIT_TERMINAL_PROMPT is dropped first, so exactly one is
// present).
func sanitizedGitEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if gitProbeEnvDropped[key] {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_TERMINAL_PROMPT=0")
}
