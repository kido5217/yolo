package tui

import (
	"strings"
	"testing"
	"unicode/utf8"
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

// TestWrapLineANSIAware (yolo-kj6): a styled line wraps on its VISIBLE width
// (the SGR escape is zero-width glue, not a run of width-1 runes) and a
// hard-split never cuts inside an escape. The S2.8 permDlg.view styles a row
// then wraps it, so the wrapper must be ANSI-aware: pre-fix, the leading SGR
// inflated the first field's width (early wrap) and cutWidth could split the
// escape, emitting a corrupted `\x1b[...` fragment.
func TestWrapLineANSIAware(t *testing.T) {
	const sgr = "\x1b[38;5;215m" // ANSI256 fg (the warning token's shape)
	const reset = "\x1b[0m"
	token := strings.Repeat("x", 30) // a 30-col single token
	in := sgr + token + reset

	got := wrapLine(in, 10)

	// (a) every ESC opens a complete CSI — no truncated escape reached the
	// terminal (the pre-fix corruption: a line ending in `\x1b[38;5;21`).
	for _, l := range strings.Split(got, "\n") {
		for i := 0; i < len(l); {
			if l[i] != 0x1b {
				_, size := utf8.DecodeRuneInString(l[i:])
				i += size
				continue
			}
			n := csiLen(l[i:])
			if n == 0 {
				t.Fatalf("corrupted (truncated) escape in line %q", l)
			}
			i += n
		}
	}
	// (b) the visible text survives the wrap (strip the SGR; drop the wrap
	// newlines so the 30-col token — now three 10-col lines — is contiguous).
	plain := strings.ReplaceAll(stripANSI(got), "\n", "")
	if !strings.Contains(plain, token) {
		t.Fatalf("visible text lost the token:\n%q", plain)
	}
	// (c) the wrap is on visible width: the SGR contributes no columns, so a
	// 30-col token at w=10 is three 10-col lines (not more, which the
	// pre-fix width-1-per-SGR-byte accounting produced).
	if n := len(strings.Split(got, "\n")); n != 3 {
		t.Fatalf("want 3 visible lines (SGR is zero-width), got %d:\n%q", n, got)
	}
}

// TestWrapLineANSIAwarePlainUnchanged pins the regression guard: a line with
// no ESC behaves exactly as before (ansiWidth == runeWidth, identical splits).
func TestWrapLineANSIAwarePlainUnchanged(t *testing.T) {
	cases := []struct {
		in   string
		w    int
		want string
	}{
		{in: "one two three four five six", w: 20, want: "one two three four\nfive six"},
		{in: "abcdefghijkl", w: 5, want: "abcde\nfghij\nkl"},
		{in: "  x y z", w: 4, want: "  x\ny\nz"},
	}
	for _, tt := range cases {
		if got := wrapLine(tt.in, tt.w); got != tt.want {
			t.Fatalf("plain wrap changed: wrapLine(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
		}
	}
}
