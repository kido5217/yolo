package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runGit runs a git fixture command in dir and fails the test on non-zero
// exit (local-only setup: real git in a t.TempDir repo, no network).
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// TestFindGitRoot pins the upward .git walk: the nearest ancestor with a
// .git entry (a FILE or a directory — worktrees and submodules carry a
// .git file) wins, ("", false) at the filesystem root or for an empty dir.
// Every fixture lives inside t.TempDir() so the walk can never escape into
// the test environment's real repo.
func TestFindGitRoot(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T) (dir, want string)
		ok    bool
	}{
		{
			name: "git dir at dir",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
				return dir, dir
			},
			ok: true,
		},
		{
			name: "worktree git file at ancestor",
			setup: func(t *testing.T) (string, string) {
				base := t.TempDir()
				if err := os.WriteFile(filepath.Join(base, ".git"), []byte("gitdir: /nonexistent"), 0o644); err != nil {
					t.Fatal(err)
				}
				work := filepath.Join(base, "work")
				if err := os.Mkdir(work, 0o755); err != nil {
					t.Fatal(err)
				}
				return work, base
			},
			ok: true,
		},
		{
			name: "subdir under repo",
			setup: func(t *testing.T) (string, string) {
				base := t.TempDir()
				if err := os.Mkdir(filepath.Join(base, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
				sub := filepath.Join(base, "a", "b")
				if err := os.MkdirAll(sub, 0o755); err != nil {
					t.Fatal(err)
				}
				return sub, base
			},
			ok: true,
		},
		{
			name: "no git anywhere",
			setup: func(t *testing.T) (string, string) {
				return t.TempDir(), ""
			},
			ok: false,
		},
		{
			name: "empty dir",
			setup: func(t *testing.T) (string, string) {
				return "", ""
			},
			ok: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir, want := tt.setup(t)
			got, ok := findGitRoot(dir)
			if got != want || ok != tt.ok {
				t.Fatalf("findGitRoot(%q) = (%q, %v), want (%q, %v)", dir, got, ok, want, tt.ok)
			}
		})
	}
}

// TestGitBranch pins the branch probe: the pinned upstream arg prefix, the
// decision-4 exit table (0 -> the trimmed stdout, "" or any other exit ->
// none), the findGitRoot no-spawn short-circuit, and the timeout bound.
func TestGitBranch(t *testing.T) {
	t.Parallel()
	env := []string{"PATH=" + os.Getenv("PATH")}
	const timeout = 2 * time.Second

	t.Run("attached branch", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		runGit(t, dir, "init", "-q")
		runGit(t, dir, "checkout", "-q", "-b", "yolo-test-branch")
		got, ok := gitBranch(t.Context(), dir, timeout, env)
		if got != "yolo-test-branch" || !ok {
			t.Fatalf("gitBranch = (%q, %v), want (yolo-test-branch, true)", got, ok)
		}
	})

	t.Run("subdir of repo", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		runGit(t, dir, "init", "-q")
		runGit(t, dir, "checkout", "-q", "-b", "yolo-test-branch")
		sub := filepath.Join(dir, "sub")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		got, ok := gitBranch(t.Context(), sub, timeout, env)
		if got != "yolo-test-branch" || !ok {
			t.Fatalf("gitBranch = (%q, %v), want (yolo-test-branch, true)", got, ok)
		}
	})

	t.Run("detached head", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		runGit(t, dir, "init", "-q")
		runGit(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x")
		runGit(t, dir, "checkout", "-q", "--detach")
		got, ok := gitBranch(t.Context(), dir, timeout, env)
		if got != "" || ok {
			t.Fatalf("gitBranch = (%q, %v), want (\"\", false)", got, ok)
		}
	})

	t.Run("broken gitdir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// A .git file pointing nowhere: the probe spawns (findGitRoot sees
		// the entry), git exits 128 -> none.
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /nonexistent-yolo-vcs-test"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, ok := gitBranch(t.Context(), dir, timeout, env)
		if got != "" || ok {
			t.Fatalf("gitBranch = (%q, %v), want (\"\", false)", got, ok)
		}
	})

	t.Run("no git dir skips spawn", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		binDir := t.TempDir()
		marker := filepath.Join(binDir, "spawned")
		script := filepath.Join(binDir, "git")
		// A fake git that leaves a marker and answers "HEAD" on stdout: if
		// the probe ever spawns with no .git in the walk, the marker lands.
		content := "#!/bin/sh\necho spawned > " + marker + "\necho HEAD\n"
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
		fakeEnv := []string{"PATH=" + binDir + ":" + os.Getenv("PATH")}
		got, ok := gitBranch(t.Context(), dir, timeout, fakeEnv)
		if got != "" || ok {
			t.Fatalf("gitBranch = (%q, %v), want (\"\", false)", got, ok)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("git spawned despite no .git (findGitRoot short-circuit broken)")
		}
	})

	t.Run("no git binary", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		empty := t.TempDir()
		noGitEnv := []string{"PATH=" + empty}
		got, ok := gitBranch(t.Context(), dir, timeout, noGitEnv)
		if got != "" || ok {
			t.Fatalf("gitBranch = (%q, %v), want (\"\", false)", got, ok)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		script := filepath.Join(dir, "git")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		slowEnv := []string{"PATH=" + dir + ":" + os.Getenv("PATH")}
		got, ok := gitBranch(t.Context(), dir, 200*time.Millisecond, slowEnv)
		if got != "" || ok {
			t.Fatalf("gitBranch = (%q, %v), want (\"\", false)", got, ok)
		}
	})
}

// TestSanitizedGitEnv pins the probe env hardening (decision 4): the 5
// repo-redirecting GIT_* vars are stripped (a repo-local or hook-set value
// must not redirect the probe away from the scope dir), GIT_TERMINAL_PROMPT=0
// is present exactly once (an inherited value is dropped, not duplicated),
// and the rest of the env passes through.
func TestSanitizedGitEnv(t *testing.T) {
	t.Setenv("GIT_DIR", "/vcs-test/.git")
	t.Setenv("GIT_WORK_TREE", "/vcs-test")
	t.Setenv("GIT_INDEX_FILE", "/vcs-test/index")
	t.Setenv("GIT_CEILING_DIRECTORIES", "/vcs-test")
	t.Setenv("GIT_COMMON_DIR", "/vcs-test/.git")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("YOLO_VCS_TEST_KEEP", "yes")

	seen := map[string]string{}
	counts := map[string]int{}
	for _, kv := range sanitizedGitEnv() {
		key, val, _ := strings.Cut(kv, "=")
		counts[key]++
		seen[key] = val
	}
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_CEILING_DIRECTORIES", "GIT_COMMON_DIR"} {
		if n := counts[key]; n != 0 {
			t.Fatalf("sanitizedGitEnv kept %s %d times, want 0", key, n)
		}
	}
	if n := counts["GIT_TERMINAL_PROMPT"]; n != 1 {
		t.Fatalf("GIT_TERMINAL_PROMPT appears %d times, want exactly 1", n)
	}
	if got := seen["GIT_TERMINAL_PROMPT"]; got != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, want \"0\"", got)
	}
	if got := seen["YOLO_VCS_TEST_KEEP"]; got != "yes" {
		t.Fatalf("YOLO_VCS_TEST_KEEP = %q, want \"yes\" (passthrough broken)", got)
	}
}

// TestGitBranchIgnoresInheritedGitDir is the hardening proof (decision 4):
// an unrelated repo B on branch "elsewhere" + the probe dir A on branch
// "yolo-test-branch"; the parent env points GIT_DIR at B (a repo-local or
// hook-set value) and sanitizedGitEnv must strip it, so the probe still
// resolves A's branch.
func TestGitBranchIgnoresInheritedGitDir(t *testing.T) {
	a := t.TempDir()
	runGit(t, a, "init", "-q")
	runGit(t, a, "checkout", "-q", "-b", "yolo-test-branch")
	b := t.TempDir()
	runGit(t, b, "init", "-q")
	runGit(t, b, "checkout", "-q", "-b", "elsewhere")

	t.Setenv("GIT_DIR", filepath.Join(b, ".git"))
	env := sanitizedGitEnv()
	got, ok := gitBranch(context.Background(), a, 2*time.Second, env)
	if got != "yolo-test-branch" || !ok {
		t.Fatalf("gitBranch = (%q, %v), want (yolo-test-branch, true)", got, ok)
	}
}
