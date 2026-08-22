package tool

import "strings"

// Truncate keeps the TAIL of text: the last up-to Limits.MaxLines lines
// within Limits.MaxBytes UTF-8 bytes. cut is true when anything was removed.
// Port of upstream shell.ts tail() (v1.18.18), including the UTF-8-boundary
// cut of a single over-long line.
func Truncate(text string, l Limits) (string, bool) {
	l = l.withDefaults()
	lines := strings.Split(text, "\n")
	if len(lines) <= l.MaxLines && len(text) <= l.MaxBytes {
		return text, false
	}
	out := make([]string, 0, l.MaxLines)
	bytes := 0
	for i := len(lines) - 1; i >= 0 && len(out) < l.MaxLines; i-- {
		size := len(lines[i])
		if len(out) > 0 {
			size++ // joining newline
		}
		if bytes+size > l.MaxBytes {
			if len(out) == 0 {
				b := []byte(lines[i])
				start := len(b) - l.MaxBytes
				if start < 0 {
					start = 0
				}
				for start < len(b) && b[start]&0xc0 == 0x80 {
					start++
				}
				out = append(out, string(b[start:]))
			}
			break
		}
		out = append(out, lines[i])
		bytes += size
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return strings.Join(out, "\n"), true
}
