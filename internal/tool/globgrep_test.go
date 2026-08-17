package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobTool(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "a", "b"), 0o755)
	os.MkdirAll(filepath.Join(d, ".git"), 0o755)
	for _, p := range []string{"a/x.go", "a/b/y.go", "a/z.md", ".git/skip.go"} {
		os.WriteFile(filepath.Join(d, p), []byte("x"), 0o644)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}

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

	_, err = runTool(t, "glob", env, map[string]any{"pattern": "*", "path": filepath.Join(d, "a", "x.go")})
	if err == nil || !strings.Contains(err.Error(), "glob path must be a directory") {
		t.Fatalf("err = %v", err)
	}

	out2, _ := runTool(t, "glob", env, map[string]any{"pattern": "nomatch*"})
	if out2.Text != "No files found" {
		t.Fatalf("empty = %q", out2.Text)
	}
}

func TestGrepTool(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "a.txt"), []byte("alpha\nbeta\n"), 0o644)
	os.WriteFile(filepath.Join(d, "b.md"), []byte("alpha here\n"), 0o644)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}

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

	out2, _ := runTool(t, "grep", env, map[string]any{"pattern": "alpha", "include": "*.md"})
	if !strings.Contains(out2.Text, "Found 1 matches") || strings.Contains(out2.Text, "a.txt") {
		t.Fatalf("include filter broken: %q", out2.Text)
	}

	out3, _ := runTool(t, "grep", env, map[string]any{"pattern": "nope"})
	if out3.Text != "No files found" {
		t.Fatalf("no match = %q", out3.Text)
	}
}

func TestGrepLimit100(t *testing.T) {
	d := t.TempDir()
	var b strings.Builder
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&b, "hit\n")
	}
	os.WriteFile(filepath.Join(d, "big.txt"), []byte(b.String()), 0o644)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, _ := runTool(t, "grep", env, map[string]any{"pattern": "hit"})
	if !strings.Contains(out.Text, "Found 100 matches (more matches available)") ||
		!strings.Contains(out.Text, "(Results truncated. Consider using a more specific path or pattern.)") {
		t.Fatalf("limit output = %q", out.Text[:300])
	}
}

func TestGrepExactBlock(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "only.txt"), []byte("x\nalpha\n"), 0o644)
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, _ := runTool(t, "grep", env, map[string]any{"pattern": "alpha"})
	want := "Found 1 matches\n\n" + filepath.Join(d, "only.txt") + ":\n  Line 2: alpha"
	if out.Text != want {
		t.Fatalf("block = %q", out.Text)
	}
}
