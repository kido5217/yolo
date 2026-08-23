package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// Implements the plan's IMPLEMENTATION FIX (locked): one shell for the whole
// test; cwd and env persistence are asserted across calls on it.
func TestBashCwdPersistsAcrossCalls(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	t.Cleanup(func() {
		if err := env.Shell.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, _ := json.Marshal(map[string]any{"command": "cd sub"})
	_, err := Registry()["bash"].Run(context.Background(), raw, env)
	if err != nil {
		t.Fatal(err)
	}
	raw2, _ := json.Marshal(map[string]any{"command": "pwd"})
	out, err := Registry()["bash"].Run(context.Background(), raw2, env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.Text) != filepath.Join(d, "sub") {
		t.Fatalf("cwd not persisted: %q", out.Text)
	}
	// env persistence too
	raw3, _ := json.Marshal(map[string]any{"command": "FOO=bar"})
	_, _ = Registry()["bash"].Run(context.Background(), raw3, env)
	raw4, _ := json.Marshal(map[string]any{"command": "echo $FOO"})
	out2, _ := Registry()["bash"].Run(context.Background(), raw4, env)
	if strings.TrimSpace(out2.Text) != "bar" {
		t.Fatalf("env not persisted: %q", out2.Text)
	}
}

func TestBashNonZeroExitIsSuccessWithMeta(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	t.Cleanup(func() {
		if err := env.Shell.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, _ := json.Marshal(map[string]any{"command": "echo oops; exit 3"})
	out, err := Registry()["bash"].Run(context.Background(), raw, env)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Meta["exit"]; got != 3 {
		t.Fatalf("exit = %v", got)
	}
	if !strings.Contains(out.Text, "oops") {
		t.Fatalf("text = %q", out.Text)
	}
}

// TestBashTruncatedOutputSavedWithMarker pins the upstream shell.ts
// contract: a truncated run stores the FULL output under
// Env.OutputDir/tool_... and the model-visible text starts with the
// verbatim marker pointing at the file — without it the model sees a
// silent mid-stream start and re-runs the command in a loop.
func TestBashTruncatedOutputSavedWithMarker(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	env := &Env{
		Dir:       d,
		Limits:    Limits{5, 1024}, // tiny: force truncation of 50 lines
		Shell:     NewShell(d, Limits{2000, 50 * 1024}),
		OutputDir: filepath.Join(d, "tool-output"),
	}
	t.Cleanup(func() {
		if err := env.Shell.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, _ := json.Marshal(map[string]any{"command": "seq 1 50"})
	out, err := Registry()["bash"].Run(context.Background(), raw, env)
	if err != nil {
		t.Fatal(err)
	}
	if cut, _ := out.Meta["truncated"].(bool); !cut {
		t.Fatalf("truncated = %v, want true", out.Meta["truncated"])
	}
	const marker = "...output truncated...\n\nFull output saved to: "
	if !strings.HasPrefix(out.Text, marker) {
		t.Fatalf("text does not start with the truncation marker:\n%q", out.Text[:min(120, len(out.Text))])
	}
	rest := strings.TrimPrefix(out.Text, marker)
	path, tail, _ := strings.Cut(rest, "\n")
	if path == "" || !filepath.IsAbs(path) {
		t.Fatalf("marker path = %q", path)
	}
	if p, ok := out.Meta["outputPath"].(string); !ok || p != path {
		t.Fatalf("outputPath = %v, want %q", out.Meta["outputPath"], path)
	}
	full, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("full output file missing: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(full), "\n"), "\n")
	if len(lines) != 50 || lines[0] != "1" || lines[49] != "50" {
		t.Fatalf("file has %d lines (first=%q last=%q), want 1..50", len(lines), lines[0], lines[len(lines)-1])
	}
	if !strings.Contains(tail, "50") {
		t.Fatalf("visible tail lost its end: %q", tail)
	}
}

// TestBashMarkerExitCode pins the marker-path exit decoding: the second
// command (marker counter n=1) must report the REAL exit 4 from the marker,
// not the counter. Regression guard for decodeMarker mis-group decoding.
func TestBashMarkerExitCode(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	t.Cleanup(func() {
		if err := env.Shell.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, _ := json.Marshal(map[string]any{"command": "true"})
	if _, err := Registry()["bash"].Run(context.Background(), raw, env); err != nil {
		t.Fatal(err)
	}
	// Subshell exit: the persistent shell survives, so the exit comes from
	// the end-marker (not the process-death path).
	raw2, _ := json.Marshal(map[string]any{"command": "(exit 4)"})
	out, err := Registry()["bash"].Run(context.Background(), raw2, env)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Meta["exit"]; got != 4 {
		t.Fatalf("exit = %v, want 4", got)
	}
}

// TestShellCwdSurvivesKillRespawn pins cwd tracking through the end-marker:
// after a timeout kill, the respawned shell must start in the last cd'd
// directory, not the original root.
func TestShellCwdSurvivesKillRespawn(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	t.Cleanup(func() {
		if err := env.Shell.Close(); err != nil {
			t.Error(err)
		}
	})
	run := func(cmd string, timeout int) (Output, error) {
		raw, _ := json.Marshal(map[string]any{"command": cmd, "timeout": timeout})
		return Registry()["bash"].Run(context.Background(), raw, env)
	}
	if _, err := run("cd sub", 10000); err != nil {
		t.Fatal(err)
	}
	// Timeout kill: the shell dies mid-command.
	if _, err := run("sleep 5", 300); err == nil {
		t.Fatal("want timeout error")
	}
	out, err := run("pwd", 10000)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(out.Text), filepath.Join(d, "sub"); got != want {
		t.Fatalf("pwd after respawn = %q, want %q", got, want)
	}
}

func TestBashStderrMerged(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	t.Cleanup(func() {
		if err := env.Shell.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, _ := json.Marshal(map[string]any{"command": "echo err >&2"})
	out, _ := Registry()["bash"].Run(context.Background(), raw, env)
	if !strings.Contains(out.Text, "err") {
		t.Fatalf("stderr missing: %q", out.Text)
	}
}

func TestBashTimeoutKillsAndReports(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	t.Cleanup(func() {
		if err := env.Shell.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, _ := json.Marshal(map[string]any{"command": "sleep 5", "timeout": 300})
	_, err := Registry()["bash"].Run(context.Background(), raw, env)
	if err == nil {
		t.Fatal("want timeout error")
	}
	want := "shell tool terminated command after exceeding timeout 300 ms. If this command is expected to take longer and is not waiting for interactive input, retry with a larger timeout value in milliseconds"
	if err.Error() != want {
		t.Fatalf("err = %q", err)
	}
	// shell respawned cleanly afterward
	raw2, _ := json.Marshal(map[string]any{"command": "echo alive"})
	out, err2 := Registry()["bash"].Run(context.Background(), raw2, env)
	if err2 != nil || strings.TrimSpace(out.Text) != "alive" {
		t.Fatalf("respawn failed: %v %q", err2, out.Text)
	}
}

func TestShellOutputCapCutsAtRuneBoundary(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	s := NewShell(d, Limits{2000, 50 * 1024})
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	// One line of 10MiB-1 'a' bytes followed by a two-byte rune: the
	// 10MiB output cap lands between the rune's bytes, so the cut must
	// back off to the rune boundary instead of leaving a dangling
	// continuation byte (invalid UTF-8) in the model-visible output.
	cmd := `head -c 10485759 /dev/zero | tr '\0' 'a'; printf '\303\251\n'`
	_, out, err := s.Exec(context.Background(), cmd, 20000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(out) {
		t.Fatal("output cap left invalid UTF-8 at the cut")
	}
	if len(out) != 10485759 {
		t.Fatalf("len(out) = %d, want %d", len(out), 10485759)
	}
}

func TestBashWhitespaceOnlyCommandRejected(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal(map[string]any{"command": "   "})
	if _, _, err := Registry()["bash"].Patterns(raw); err == nil {
		t.Fatal("want error for whitespace-only command")
	}
}

func TestBashPermissionPatterns(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal(map[string]any{"command": "git commit -m x"})
	res, always, err := Registry()["bash"].Patterns(raw)
	if err != nil {
		t.Fatalf("patterns err = %v", err)
	}
	if len(res) != 1 || res[0] != "git *" {
		t.Fatalf("patterns = %v, want [git *]", res)
	}
	if len(always) != 1 || always[0] != "git *" {
		t.Fatalf("always = %v, want [git *]", always)
	}
	raw2, _ := json.Marshal(map[string]any{"command": "ls"})
	res2, _, _ := Registry()["bash"].Patterns(raw2)
	if len(res2) != 1 || res2[0] != "ls" {
		t.Fatalf("single-token = %v, want [ls]", res2)
	}
	if Registry()["bash"].Permission() != "bash" {
		t.Fatal("perm action")
	}
}

// TestBashTimeoutRejectedAndClamped pins ⑥: timeout <= 0 is rejected with a
// tool-result error naming the constraint; values above 2^31-1 ms are
// clamped to that ceiling (the int64 ns Duration cannot wrap).
func TestBashTimeoutRejectedAndClamped(t *testing.T) {
	cases := []struct {
		name    string
		timeout any
		wantErr bool
		wantMS  int
	}{
		{"zero rejected", 0, true, 0},
		{"negative rejected", -5, true, 0}, // argInt rejects f<0
		{"default when absent", nil, false, defaultBashTimeoutMS},
		{"small ok", 300, false, 300},
		{"ceiling kept", 1<<31 - 1, false, 1<<31 - 1},
		{"above ceiling clamped", 1 << 32, false, 1<<31 - 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := map[string]any{"command": "ls"}
			if c.timeout != nil {
				m["timeout"] = c.timeout
			}
			raw, _ := json.Marshal(m)
			_, gotMS, err := bashArgs(raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("timeout %v: want error, got nil (ms=%d)", c.timeout, gotMS)
				}
				if !strings.Contains(err.Error(), "positive integer") {
					t.Fatalf("timeout %v: error %q does not name the constraint", c.timeout, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("timeout %v: unexpected error %v", c.timeout, err)
			}
			if gotMS != c.wantMS {
				t.Fatalf("timeout %v: ms = %d, want %d", c.timeout, gotMS, c.wantMS)
			}
		})
	}
}
