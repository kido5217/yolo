package tui

import (
	"strconv"
	"unicode"
)

// number ports the upstream Locale.number (packages/tui/src/util/locale.ts:46-54):
// the ≥1e6 → "1.2M" / ≥1e3 → "1.2K" compact form, the plain string below.
func number(n int64) string {
	switch {
	case n >= 1_000_000:
		return strconv.FormatFloat(float64(n)/1e6, 'f', 1, 64) + "M"
	case n >= 1_000:
		return strconv.FormatFloat(float64(n)/1e3, 'f', 1, 64) + "K"
	default:
		return strconv.FormatInt(n, 10)
	}
}

// truncateMiddle is the port of upstream Locale.truncateMiddle (locale.ts):
// the head and tail survive, the middle collapses to "…" (width in columns,
// via runeWidth; short strings pass through).
func truncateMiddle(s string, w int) string {
	if w < 1 {
		return ""
	}
	if runeWidth(s) <= w {
		return s
	}
	r := []rune(s)
	if w == 1 {
		return string(r[:1])
	}
	room := w - 1
	head := room / 2
	tail := room - head
	out := make([]rune, 0, w)
	out = append(out, r[:head]...)
	out = append(out, '…')
	out = append(out, r[len(r)-tail:]...)
	return string(out)
}

// titlecase is the port of upstream Locale.titlecase (locale.ts): the first
// rune uppercased, the rest untouched.
func titlecase(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
