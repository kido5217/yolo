// Package glob implements the segment-matched pattern matcher used by the
// permission engine. Ported from opencode's glob semantics: "*" and "?" never
// cross "/", a lone "**" segment matches zero or more segments, and patterns
// without "/" anchor on the basename.
package glob

import (
	"fmt"
	"regexp"
	"strings"
)

// Match reports whether name matches the glob pattern.
//
//   - pattern == "*" -> always true
//   - pattern without "/" -> matches the last path segment (basename) of name
//   - pattern with "/" -> anchored: matches the whole name (relative or absolute)
//   - tokens: "*" any run within a segment, "?" one non-"/" char,
//     "[...]" char class ("!..." negates), "**" as a whole segment = zero or more segments
func Match(pattern, name string) bool {
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "/") {
		return segmentMatch(pattern, lastSegment(name))
	}
	return segmentsMatch(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

// ToRegex compiles pattern into an anchored regexp. For slash-free patterns
// the regexp is meant to be applied to the basename; for slash patterns it
// matches the whole name. Match is the canonical entry point; this is
// exported for callers that need the compiled form.
func ToRegex(pattern, name string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, fmt.Errorf("glob: empty pattern")
	}
	if pattern == "*" {
		return regexp.MustCompile(`^.*$`), nil
	}
	if !strings.Contains(pattern, "/") {
		return segmentRe(pattern)
	}
	var b strings.Builder
	b.WriteString("^")
	for i, seg := range strings.Split(pattern, "/") {
		switch {
		case seg == "**" && i == 0:
			b.WriteString(`(?:[^/]+/)*`)
		case seg == "**":
			b.WriteString(`(?:/[^/]+)*`)
		default:
			if i > 0 {
				b.WriteString("/")
			}
			b.WriteString(bare(seg))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// lastSegment returns the last non-empty path segment of name.
func lastSegment(name string) string {
	segs := strings.Split(name, "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i] != "" {
			return segs[i]
		}
	}
	return ""
}

// segmentsMatch walks pattern and name segments with backtracking for "**".
func segmentsMatch(ps, ns []string) bool {
	for len(ps) > 0 {
		if ps[0] == "**" {
			if len(ps) == 1 {
				return true
			}
			for i := 0; i <= len(ns); i++ {
				if segmentsMatch(ps[1:], ns[i:]) {
					return true
				}
			}
			return false
		}
		if len(ns) == 0 || !segmentMatch(ps[0], ns[0]) {
			return false
		}
		ps, ns = ps[1:], ns[1:]
	}
	return len(ns) == 0
}

// segmentMatch reports whether a single segment glob matches a segment.
func segmentMatch(pat, seg string) bool {
	re := mustCompile(segmentRe(pat))
	return re.MatchString(seg)
}

// bare returns the unanchored regex for one segment glob.
func bare(seg string) string {
	re := mustCompile(segmentRe(seg))
	s := re.String()
	// strip the ^ and $ anchors added by segmentRe
	return s[1 : len(s)-1]
}

// segmentRe converts one segment glob into an anchored regexp string.
// Within a segment there is no "/" to cross, so "*" -> ".*" and "?" -> ".".
func segmentRe(seg string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i + 1
			neg := false
			if j < len(seg) && (seg[j] == '!' || seg[j] == '^') {
				neg = true
				j++
			}
			start := j
			for j < len(seg) && seg[j] != ']' {
				j++
			}
			if j >= len(seg) {
				b.WriteString(`\[`)
			} else {
				b.WriteString("[")
				if neg {
					b.WriteString("^")
				}
				b.WriteString(seg[start:j])
				b.WriteString("]")
				i = j
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func mustCompile(re *regexp.Regexp, err error) *regexp.Regexp {
	_ = err
	return re
}
