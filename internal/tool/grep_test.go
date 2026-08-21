package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func grepFixture(t *testing.T) (*Env, string) {
	t.Helper()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "a.txt"), []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "b.md"), []byte("alpha here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}, d
}

func TestGrepTool(t *testing.T) {
	t.Parallel()
	t.Run("matches", func(t *testing.T) {
		t.Parallel()
		env, d := grepFixture(t)
		out, err := runTool(t, "grep", env, map[string]any{"pattern": "alpha"})
		if err != nil {
			t.Fatal(err)
		}
		joined := out.Text
		if !strings.Contains(joined, "Found 2 matches") ||
			!strings.Contains(joined, filepath.Join(d, "a.txt")+":") ||
			!strings.Contains(joined, "  Line 1: alpha") ||
			!strings.Contains(joined, filepath.Join(d, "b.md")+":") {
			t.Fatalf("output = %q", joined)
		}
	})
	t.Run("include filter", func(t *testing.T) {
		t.Parallel()
		env, _ := grepFixture(t)
		out, _ := runTool(t, "grep", env, map[string]any{"pattern": "alpha", "include": "*.md"})
		if !strings.Contains(out.Text, "Found 1 matches") || strings.Contains(out.Text, "a.txt") {
			t.Fatalf("include filter broken: %q", out.Text)
		}
	})
	t.Run("no match", func(t *testing.T) {
		t.Parallel()
		env, _ := grepFixture(t)
		out, _ := runTool(t, "grep", env, map[string]any{"pattern": "nope"})
		if out.Text != "No files found" {
			t.Fatalf("no match = %q", out.Text)
		}
	})
}

func TestGrepLimit100(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	var b strings.Builder
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&b, "hit\n")
	}
	if err := os.WriteFile(filepath.Join(d, "big.txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, _ := runTool(t, "grep", env, map[string]any{"pattern": "hit"})
	if !strings.Contains(out.Text, "Found 100 matches (more matches available)") ||
		!strings.Contains(out.Text, "(Results truncated. Consider using a more specific path or pattern.)") {
		t.Fatalf("limit output = %q", out.Text[:300])
	}
}

func TestGrepExactBlock(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "only.txt"), []byte("x\nalpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, _ := runTool(t, "grep", env, map[string]any{"pattern": "alpha"})
	want := "Found 1 matches\n\n" + filepath.Join(d, "only.txt") + ":\n  Line 2: alpha"
	if out.Text != want {
		t.Fatalf("block = %q", out.Text)
	}
}
