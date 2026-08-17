package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Implements the plan's IMPLEMENTATION FIX (locked): one shell for the whole
// test; cwd and env persistence are asserted across calls on it.
func TestBashCwdPersistsAcrossCalls(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Shell: NewShell(d, Limits{2000, 50 * 1024})}
	t.Cleanup(func() { env.Shell.Close() })
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
	want := "shell tool terminated command after exceeding timeout 300 ms. If this command is expected to take longer and is not waiting for interactive input, retry with a larger timeout value in milliseconds."
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
