package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runTool(t *testing.T, id string, env *Env, args map[string]any) (Output, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	return Registry()[id].Run(context.Background(), raw, env)
}

// TestDiffCountsPins locks the exact added/removed counts diffCounts reports
// (write/edit part meta are the contract). The counts are the optimal
// newline-terminated-line edit (upstream jsdiff diffLines parity — deviation
// 104): three former pins changed when the DP-LCS (bare-line Split) was
// replaced by go-udiff's line diff.
func TestDiffCountsPins(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		oldText, newText string
		added, removed   int
	}{
		{name: "empty to two lines", oldText: "", newText: "a\nb\n", added: 2, removed: 0},
		{name: "one line replaced", oldText: "one\ntwo\nthree\n", newText: "one\nTWO\nthree\n", added: 1, removed: 1},
		{name: "rotated lines", oldText: "a\nb\na\n", newText: "b\na\nb\n", added: 1, removed: 1},
		{name: "two lines removed", oldText: "a\na\na\n", newText: "a\n", added: 0, removed: 2},
		{name: "single line to empty", oldText: "x", newText: "", added: 0, removed: 1},
		{name: "both empty", oldText: "", newText: "", added: 0, removed: 0},
		{name: "trailing newline dropped", oldText: "a\n", newText: "a", added: 1, removed: 1},
		{name: "duplicate line removed", oldText: "hit\nhit\n", newText: "hit\n", added: 0, removed: 1},
		{name: "first out last in", oldText: "p\nq\nr", newText: "q\nr\ns", added: 2, removed: 2},
		{name: "pairwise swap", oldText: "a\nb\nc\nd\n", newText: "b\nd\na\nc\n", added: 2, removed: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			added, removed := diffCounts(c.oldText, c.newText)
			if added != c.added || removed != c.removed {
				t.Fatalf("diffCounts(%q,%q) = (%d,%d), want (%d,%d)",
					c.oldText, c.newText, added, removed, c.added, c.removed)
			}
		})
	}
}

// TestDiffCountsLargeFileGuard: the security-3 hot path — a one-line edit of
// a ~60k-line file must stay interactive (the old DP was ~3.6e9 cells,
// tens of seconds).
func TestDiffCountsLargeFileGuard(t *testing.T) {
	t.Parallel()
	const n = 60000
	before := make([]string, n)
	for i := range before {
		before[i] = fmt.Sprintf("line %d", i)
	}
	a := strings.Join(before, "\n")
	b := strings.Replace(a, "line 30000", "LINE 30000", 1)
	start := time.Now()
	added, removed := diffCounts(a, b)
	if added != 1 || removed != 1 {
		t.Fatalf("diffCounts = (%d,%d), want (1,1)", added, removed)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("diffCounts on a 60k-line file took %v, want < 2s (security-3)", d)
	}
}

// BenchmarkDiffCounts pins the line-diff hot path (security-3) for
// regression: a one-line edit of a ~60k-line file (the shape that blocked
// the engine for tens of seconds under the old O(n*m) DP).
func BenchmarkDiffCounts(b *testing.B) {
	const n = 60000
	before := make([]string, n)
	for i := range before {
		before[i] = fmt.Sprintf("line %d", i)
	}
	a := strings.Join(before, "\n")
	after := strings.Replace(a, "line 30000", "LINE 30000", 1)
	b.ReportAllocs()
	for b.Loop() {
		diffCounts(a, after)
	}
}

func TestWriteCreatesAndReports(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, err := runTool(t, "write", env, map[string]any{
		"filePath": filepath.Join(d, "new.txt"), "content": "a\nb\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "Wrote file successfully." {
		t.Fatalf("text = %q", out.Text)
	}
	metaAdded, _ := out.Meta["added"].(int)
	if metaAdded != 2 {
		t.Fatalf("added = %v", out.Meta)
	}
	b, _ := os.ReadFile(filepath.Join(d, "new.txt"))
	if string(b) != "a\nb\n" {
		t.Fatal("content mismatch")
	}
}

func TestWriteMissingDirCreated(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	fp := filepath.Join(d, "a", "b", "c.txt")
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	_, err := runTool(t, "write", env, map[string]any{
		"filePath": fp, "content": "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(b) != "x" {
		t.Fatalf("content = %q, want x", b)
	}
}
