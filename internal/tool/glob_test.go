package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// TestGlobBoundedWalkEarlyStop pins ⑫: with more than the cap of matches,
// the walk stops at the (cap+1)th match (walk order, no global sort).
// Fixture: 99 root files f000..f098, a zz/ dir with 100 files, and a root
// zz.txt. Walk order visits zz/* BEFORE zz.txt (the entry "zz" sorts before
// "zz.txt"), so the 100th walk-order match is zz/000.txt and the walk stops
// at zz/001.txt. The old full-walk+sort output kept zz.txt ("." < "/" so
// zz.txt sorts before zz/*) and dropped zz/000.txt — the inverse.
func TestGlobBoundedWalkEarlyStop(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	for i := 0; i < 99; i++ {
		if err := os.WriteFile(filepath.Join(d, fmt.Sprintf("f%03d.txt", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, "zz.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d, "zz"), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(d, "zz", fmt.Sprintf("%03d.txt", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runTool(t, "glob", &Env{Dir: d}, map[string]any{"pattern": "*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "zz/000.txt") {
		t.Fatalf("zz/000.txt (100th walk-order match) missing: %q", out.Text)
	}
	if strings.Contains(out.Text, "zz.txt") {
		t.Fatalf("zz.txt (sorts first, walk-order last) must be dropped by the capped walk, but was returned:\n%s", out.Text)
	}
	if out.Meta["truncated"] != true {
		t.Fatalf("truncated = %v, want true", out.Meta["truncated"])
	}
	if out.Meta["count"] != 100 {
		t.Fatalf("count = %v, want 100", out.Meta["count"])
	}
}

// TestGlobSchemaEmittedBytes pins the glob tool's emitted JSON schema
// byte-for-byte: the "path" description is verbatim upstream text, so a
// line-layout change in the Go source must not change the emitted bytes
// (style-007).
func TestGlobSchemaEmittedBytes(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(globTool{}.Schema())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != "7be2f83680be0648b924faaf2a003118a79903dc865b7a4d6262c6b0c5be1684" {
		t.Fatalf("glob schema sha256 = %s, want 7be2f83680be0648b924faaf2a003118a79903dc865b7a4d6262c6b0c5be1684 (emitted bytes changed)", got)
	}
}
