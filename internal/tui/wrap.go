package tui

import (
	"strings"
)

// wrapLine word-wraps a single plain text line to at most w display columns:
// it breaks at the last word boundary before the limit so words stay intact,
// and a single token longer than w is hard-split at w. A line that fits is
// returned unchanged. Leading spaces stay on the first line only;
// continuation lines start flush left. Plain text only — callers must not
// pass ANSI-styled strings.
func wrapLine(s string, w int) string {
	if w < 1 || s == "" {
		return s
	}
	if runeWidth(s) <= w {
		return s
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	lead := s[:len(s)-len(strings.TrimLeft(s, " \t"))]
	effW := w - runeWidth(lead)
	if effW < 1 {
		effW = 1
	}
	var (
		lines []string
		cur   string
		curW  int
	)
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur, curW = "", 0
		}
	}
	for _, f := range fields {
		fw := runeWidth(f)
		if fw > effW {
			flush()
			for len(f) > 0 {
				chunk, rest := cutWidth(f, effW)
				lines = append(lines, chunk)
				f = rest
			}
			continue
		}
		switch {
		case cur == "":
			cur, curW = f, fw
		case curW+1+fw <= effW:
			cur, curW = cur+" "+f, curW+1+fw
		default:
			flush()
			cur, curW = f, fw
		}
	}
	flush()
	return lead + strings.Join(lines, "\n")
}

// cutWidth splits s into the longest rune prefix of width <= w and the rest.
func cutWidth(s string, w int) (string, string) {
	ww := 0
	for i, r := range s {
		rw := runeWidth(string(r))
		if ww+rw > w {
			return s[:i], s[i:]
		}
		ww += rw
	}
	return s, ""
}

// runeWidth sums the terminal display width of the runes in s: 2 for the
// common East Asian wide ranges (and emoji), 1 otherwise (a tab counts as 1).
func runeWidth(s string) int {
	ww := 0
	for _, r := range s {
		if isWide(r) {
			ww += 2
		} else {
			ww++
		}
	}
	return ww
}

// isWide reports whether r renders two columns in a terminal (the compact
// East Asian width set).
func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0x303E, // CJK radicals, Kangxi, CJK symbols
		r >= 0x3041 && r <= 0x33FF, // kana, CJK compatibility
		r >= 0x3400 && r <= 0x4DBF, // CJK unified extension A
		r >= 0x4E00 && r <= 0x9FFF, // CJK unified ideographs
		r >= 0xA960 && r <= 0xA97F, // Hangul Jamo extended-A
		r >= 0xAC00 && r <= 0xD7A3, // Hangul syllables
		r >= 0xF900 && r <= 0xFAFF, // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE4F, // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60, // fullwidth forms
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1FAFF, // emoji
		r >= 0x20000 && r <= 0x3FFFD: // CJK unified extensions B+
		return true
	}
	return false
}
