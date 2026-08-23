package tool

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestDiffCountsPins locks the exact added/removed counts diffCounts reports
// (write/edit part meta are the contract). The counts are the optimal
// newline-terminated-line edit (upstream jsdiff diffLines parity — deviation
// 104): three former pins changed when the DP-LCS (bare-line Split) was
// replaced by go-udiff's line diff.
func TestDiffCountsPins(t *testing.T) {
	cases := []struct {
		oldText, newText string
		added, removed   int
	}{
		{"", "a\nb\n", 2, 0},
		{"one\ntwo\nthree\n", "one\nTWO\nthree\n", 1, 1},
		{"a\nb\na\n", "b\na\nb\n", 1, 1},
		{"a\na\na\n", "a\n", 0, 2},
		{"x", "", 0, 1},
		{"", "", 0, 0},
		{"a\n", "a", 1, 1},
		{"hit\nhit\n", "hit\n", 0, 1},
		{"p\nq\nr", "q\nr\ns", 2, 2},
		{"a\nb\nc\nd\n", "b\nd\na\nc\n", 2, 2},
	}
	for i, c := range cases {
		added, removed := diffCounts(c.oldText, c.newText)
		if added != c.added || removed != c.removed {
			t.Errorf("case %d: diffCounts(%q,%q) = (%d,%d), want (%d,%d)",
				i, c.oldText, c.newText, added, removed, c.added, c.removed)
		}
	}
}

// TestDiffCountsLargeFileGuard: the security-3 hot path — a one-line edit of
// a ~60k-line file must stay interactive (the old DP was ~3.6e9 cells,
// tens of seconds).
func TestDiffCountsLargeFileGuard(t *testing.T) {
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
