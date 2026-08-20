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
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
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

func TestBashStderrMerged(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	raw, _ := json.Marshal(map[string]any{"command": "echo err >&2"})
	out, _ := Registry()["bash"].Run(context.Background(), raw, env)
	if !strings.Contains(out.Text, "err") {
		t.Fatalf("stderr missing: %q", out.Text)
	}
}

func TestBashTimeoutKillsAndReports(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
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
	raw, _ := json.Marshal(map[string]any{"command": "   "})
	if _, _, err := Registry()["bash"].Patterns(raw); err == nil {
		t.Fatal("want error for whitespace-only command")
	}
}

func TestBashPermissionPatterns(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"command": "git commit -m x"})
	res, always, err := Registry()["bash"].Patterns(raw)
	if err != nil || res[0] != "git *" || always[0] != "git *" {
		t.Fatalf("patterns %v %v %v", res, always, err)
	}
	raw2, _ := json.Marshal(map[string]any{"command": "ls"})
	res2, _, _ := Registry()["bash"].Patterns(raw2)
	if res2[0] != "ls" {
		t.Fatalf("single-token = %v", res2)
	}
	if Registry()["bash"].Permission() != "bash" {
		t.Fatal("perm action")
	}
}
