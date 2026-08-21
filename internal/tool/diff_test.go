package tool

import "testing"

// TestDiffCountsPins locks the exact added/removed counts diffCounts reports
// so that a cheaper LCS implementation must reproduce them byte-for-byte
// (write/edit part meta are the contract).
func TestDiffCountsPins(t *testing.T) {
	cases := []struct {
		old, new  string
		added, rm int
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
		added, removed := diffCounts(c.old, c.new)
		if added != c.added || removed != c.rm {
			t.Errorf("case %d: diffCounts(%q,%q) = (%d,%d), want (%d,%d)",
				i, c.old, c.new, added, removed, c.added, c.rm)
		}
	}
}
