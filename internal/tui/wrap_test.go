package tui

import (
	"strings"
	"testing"
)

func TestWrapLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		w    int
		want string
	}{
		{name: "fits unchanged", in: "hello world", w: 80, want: "hello world"},
		{name: "exact fit unchanged", in: strings.Repeat("a", 20), w: 20, want: strings.Repeat("a", 20)},
		{name: "word wrap", in: "one two three four five six", w: 20, want: "one two three four\nfive six"},
		{name: "break at last boundary before the limit", in: "aaaa bbbbb c", w: 10, want: "aaaa bbbbb\nc"},
		{name: "over-long token hard split", in: "abcdefghijkl", w: 5, want: "abcde\nfghij\nkl"},
		{name: "over-long token then word", in: "abcdefghij q", w: 5, want: "abcde\nfghij\nq"},
		{name: "leading spaces stay on the first line only", in: "  x y z", w: 4, want: "  x\ny\nz"},
		{name: "tab is a word separator", in: "a\tbb cc", w: 4, want: "a bb\ncc"},
		{name: "whitespace only collapses to empty", in: "   ", w: 2, want: ""},
		{name: "empty unchanged", in: "", w: 80, want: ""},
		{name: "invalid width unchanged", in: "hello world", w: 0, want: "hello world"},
		{name: "cjk counts two columns", in: "日本語のテキスト", w: 8, want: "日本語の\nテキスト"},
		{name: "cjk and ascii mix", in: "ab日本語cd", w: 6, want: "ab日本\n語cd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapLine(tt.in, tt.w)
			if got != tt.want {
				t.Fatalf("wrapLine(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
			}
			for _, l := range strings.Split(got, "\n") {
				if w := runeWidth(l); w > tt.w && tt.w > 0 {
					t.Errorf("line %q has width %d > %d", l, w, tt.w)
				}
			}
		})
	}
}
