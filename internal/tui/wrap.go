package tui

import (
	"strings"
	"unicode/utf8"
)

// wrapLine word-wraps a single line to at most w display columns: it breaks
// at the last word boundary before the limit so words stay intact, and a
// single token longer than w is hard-split at w. A line that fits is returned
// unchanged. Leading spaces stay on the first line only; continuation lines
// start flush left.
//
// ANSI-aware (yolo-kj6): a CSI escape (SGR styling, \x1b[...m) is zero-width
// glue — it counts toward no display width and a hard-split never cuts inside
// it. So a styled line wraps on its visible width and no corrupted escape
// reaches the terminal. Plain text (no ESC) wraps exactly as before.
func wrapLine(s string, w int) string {
	if w < 1 || s == "" {
		return s
	}
	if ansiWidth(s) <= w {
		return s
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	lead := s[:len(s)-len(strings.TrimLeft(s, " \t"))]
	effW := w - ansiWidth(lead)
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
		fw := ansiWidth(f)
		if fw > effW {
			flush()
			for len(f) > 0 {
				chunk, rest := ansiCutWidth(f, effW)
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

// ansiWidth sums the terminal display width of s, treating a CSI escape
// sequence (SGR styling) as zero-width glue. For plain text (no ESC) it
// equals runeWidth.
func ansiWidth(s string) int {
	ww := 0
	i := 0
	for i < len(s) {
		if n := csiLen(s[i:]); n > 0 {
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if isWide(r) {
			ww += 2
		} else {
			ww++
		}
	}
	return ww
}

// ansiCutWidth splits s into the longest prefix of visible width <= w and the
// rest, never splitting inside a CSI escape: leading escape glue rides along
// with the prefix and the cut lands on a rune boundary after it.
func ansiCutWidth(s string, w int) (string, string) {
	ww := 0
	i := 0
	for i < len(s) {
		if n := csiLen(s[i:]); n > 0 {
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := 1
		if isWide(r) {
			rw = 2
		}
		if ww+rw > w {
			if i == 0 {
				// a single rune wider than w (a CJK glyph at w=1): emit it
				// alone so the split always makes progress (no zero-length
				// chunk loop)
				return s[:size], s[size:]
			}
			return s[:i], s[i:]
		}
		ww += rw
		i += size
	}
	return s, ""
}

// csiLen returns the byte length of the CSI escape sequence starting at s
// (ESC '[' <params> <intermediates> <final>), or 0 if s does not begin with a
// complete one. Parameter bytes are 0x30-0x3F, intermediate bytes 0x20-0x2F,
// and the final byte 0x40-0x7E; SGR styling is the common case (digits + ';'
// + 'm').
func csiLen(s string) int {
	if len(s) < 2 || s[0] != 0x1b || s[1] != '[' {
		return 0
	}
	i := 2
	for i < len(s) {
		c := s[i]
		switch {
		case (c >= 0x20 && c <= 0x2f) || (c >= 0x30 && c <= 0x3f):
			i++
		case c >= 0x40 && c <= 0x7e:
			return i + 1
		default:
			return 0
		}
	}
	return 0 // unterminated: not a complete CSI
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
