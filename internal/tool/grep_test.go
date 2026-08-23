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

func TestGrepLineSplitSemantics(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	// "a\n" splits into ["a", ""] and "" splits into [""]: a pattern that
	// matches empty lines must hit the trailing empty segment as its own
	// (last) line number.
	if err := os.WriteFile(filepath.Join(d, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "empty.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, _ := runTool(t, "grep", env, map[string]any{"pattern": "^$"})
	want := "Found 2 matches\n\n" + filepath.Join(d, "a.txt") + ":\n  Line 2: \n\n" +
		filepath.Join(d, "empty.txt") + ":\n  Line 1: "
	if out.Text != want {
		t.Fatalf("block = %q, want %q", out.Text, want)
	}
}

// TestGrepMissingRootIsError: a stat failure on the searched root is a tool
// error, not a "No files found" success (error-3).
func TestGrepMissingRootIsError(t *testing.T) {
	t.Parallel()
	env, _ := grepFixture(t)
	_, err := runTool(t, "grep", env, map[string]any{"pattern": "alpha", "path": "no_such_dir"})
	if err == nil {
		t.Fatal("grep on a nonexistent root succeeded — it must be a tool error")
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
