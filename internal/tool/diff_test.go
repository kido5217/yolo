package tool

import "testing"

// TestDiffCountsPins locks the exact added/removed counts diffCounts reports
// so that a cheaper LCS implementation must reproduce them byte-for-byte
// (write/edit part meta are the contract).
func TestDiffCountsPins(t *testing.T) {
	cases := []struct {
		oldText, newText string
		added, removed   int
	}{
		{"", "a\nb\n", 2, 0},
		{"one\ntwo\nthree\n", "one\nTWO\nthree\n", 1, 1},
		{"a\nb\na\n", "b\na\nb\n", 1, 1},
		{"a\na\na\n", "a\n", 0, 2},
		{"x", "", 1, 1},
		{"", "", 0, 0},
		{"a\n", "a", 0, 1},
		{"hit\nhit\n", "hit\n", 0, 1},
		{"p\nq\nr", "q\nr\ns", 1, 1},
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
