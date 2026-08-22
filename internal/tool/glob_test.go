package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func globFixture(t *testing.T) (*Env, string) {
	t.Helper()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a/x.go", "a/b/y.go", "a/z.md", ".git/skip.go"} {
		if err := os.WriteFile(filepath.Join(d, p), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}, d
}

func TestGlobTool(t *testing.T) {
	t.Parallel()
	t.Run("matches nested, skips hidden", func(t *testing.T) {
		t.Parallel()
		env, _ := globFixture(t)
		out, err := runTool(t, "glob", env, map[string]any{"pattern": "**/*.go"})
		if err != nil {
			t.Fatal(err)
		}
		// Plan test bug: expected 3 lines, but only 2 files can match
		// "**/*.go" — .git/skip.go is excluded by the hidden-skip rule the
		// same test asserts, and a/z.md is not a .go file.
		lines := strings.Split(strings.TrimSpace(out.Text), "\n")
		if len(lines) != 2 {
			t.Fatalf("lines = %d: %v", len(lines), lines)
		}
		for _, frag := range []string{"/a/x.go", "/a/b/y.go"} {
			if !strings.Contains(out.Text, frag) {
				t.Fatalf("missing %q in %q", frag, out.Text)
			}
		}
		if strings.Contains(out.Text, ".git") {
			t.Fatal(".git leaked")
		}
	})
	t.Run("path must be a directory", func(t *testing.T) {
		t.Parallel()
		env, d := globFixture(t)
		_, err := runTool(t, "glob", env, map[string]any{"pattern": "*", "path": filepath.Join(d, "a", "x.go")})
		if err == nil || !strings.Contains(err.Error(), "glob path must be a directory") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("no match", func(t *testing.T) {
		t.Parallel()
		env, _ := globFixture(t)
		out, _ := runTool(t, "glob", env, map[string]any{"pattern": "nomatch*"})
		if out.Text != "No files found" {
			t.Fatalf("empty = %q", out.Text)
		}
	})
}
