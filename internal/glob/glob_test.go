package glob

import "testing"

func cases() map[string]map[string]bool {
	// pattern → name → matches
	return map[string]map[string]bool{
		"*":             {"/a/b/c.go": true, "x": true},
		"*.env":         {"a.env": true, "src/a.env": true, "a.env.bak": false, "a.go": false},
		"*.env.*":       {"a.env.local": true, "a.env": false, "b/env2": false},
		"src/**/*.go":   {"src/a.go": true, "src/x/y/b.go": true, "a.go": false, "src.go": false},
		"src/*":         {"src/a.go": true, "src/x/y.go": false},
		"path/file.txt": {"path/file.txt": true, "/w/path/file.txt": false}, // anchored relative form
		"/a/*/c":        {"/a/b/c": true, "/a/b/d/c": false},
		"a?c":           {"abc": true, "a/c": false, "abbc": false},
		"[abc].go":      {"b.go": true, "d.go": false},
		"**":            {"/x": true, "x/y": true},
	}
}

func TestMatch(t *testing.T) {
	for pat, names := range cases() {
		for name, want := range names {
			if got := Match(pat, name); got != want {
				t.Errorf("Match(%q, %q) = %v, want %v", pat, name, got, want)
			}
		}
	}
}
