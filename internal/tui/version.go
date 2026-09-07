package tui

import "strings"

// plainSemver renders the git-describe build string as plain semver for
// the home footer (upstream app.version is already plain; yolo injects
// `git describe --tags --always --dirty`, main.go:36): strip the leading
// "v" and cut at the first "-" (the -N-g<sha> distance suffix and any
// -dirty suffix). "0.0.0-dev" -> "0.0.0", "v0.8.0" -> "0.8.0",
// "v0.8.0-4-gabcdef" -> "0.8.0", "v0.8.0-dirty" -> "0.8.0", "" -> "".
func plainSemver(s string) string {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[:i]
	}
	return s
}
