package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/sahilm/fuzzy"
)

const (
	// maxWalkFiles caps the number of files the @-picker walk collects.
	maxWalkFiles = 1000
	// maxWalkDepth caps the number of root-relative path segments the walk
	// descends into.
	maxWalkDepth = 8
	// maxPickerOptions caps the @-picker's option rows (the ported limit 10).
	maxPickerOptions = 10
)

// walkIgnore is the @-picker walk's static ignore set (deviation 225 — the
// TUI-local walk replaces upstream's FFF server-side file search).
var walkIgnore = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, "target": true, ".next": true, "coverage": true,
	"__pycache__": true, ".venv": true, "venv": true,
}

// mentionTriggerIndex is the ported upstream display.ts rule: the last "@"
// at the start of the value or preceded by whitespace, whose following text
// (from the "@") carries no whitespace. It returns the trigger index and
// whether an @-trigger is active.
func mentionTriggerIndex(value string) (int, bool) {
	idx := strings.LastIndex(value, "@")
	if idx == -1 {
		return -1, false
	}
	if idx > 0 && !unicode.IsSpace(rune(value[idx-1])) {
		return -1, false
	}
	for _, r := range value[idx:] {
		if unicode.IsSpace(r) {
			return -1, false
		}
	}
	return idx, true
}

// gitignorePatterns is the minimal .gitignore parse (deviation 225): the
// non-comment non-blank lines of <root>/.gitignore, trailing "/" stripped.
// Empty when the file is absent.
func gitignorePatterns(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, strings.TrimSuffix(l, "/"))
	}
	return out
}

// ignoredByGitignore reports whether a pattern equals one of rel's path
// segments — the sanctioned minimal parse (deviation 225).
func ignoredByGitignore(patterns []string, rel string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		for _, p := range patterns {
			if seg == p {
				return true
			}
		}
	}
	return false
}

// walkFiles walks root (the scope dir) depth- and file-capped, skipping the
// static ignore set and .gitignore-matched dirs and files, and returns the
// slash-relative paths (deviation 225).
func walkFiles(root string) []string {
	patterns := gitignorePatterns(root)
	var out []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		depth := strings.Count(rel, "/") + 1
		if depth > maxWalkDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if walkIgnore[d.Name()] || ignoredByGitignore(patterns, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if walkIgnore[d.Name()] || ignoredByGitignore(patterns, rel) {
			return nil
		}
		out = append(out, rel)
		if len(out) >= maxWalkFiles {
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

// walkedFiles is the cached @-picker walk of the scope dir (deviation 225);
// it re-walks only when the scope dir changes.
func (a *App) walkedFiles() []string {
	if a.walkRoot != a.Dir {
		a.walkRoot = a.Dir
		a.walked = walkFiles(a.walkRoot)
	}
	return a.walked
}

// freqFor returns the frecency entry for relPath (nil when absent).
func (a *App) freqFor(relPath string) *frecencyEntry {
	for i := range a.freq {
		if a.freq[i].Path == relPath {
			return &a.freq[i]
		}
	}
	return nil
}

// mentionOptions builds the @-picker rows: fuzzy.Find over the walked files
// by the @-query, each score x2 for a prefix match x (1 + frecencyScore)
// (the ported upstream scoreFn), sorted desc, capped at maxPickerOptions.
// An empty query lists all walked files, frecency-ranked. Each option's
// value is the path string (plain-text insert — deviation-222 class).
func (a *App) mentionOptions() []selectOption {
	if !a.prompt.mentionActive() {
		return nil
	}
	files := a.walkedFiles()
	if len(files) == 0 {
		return nil
	}
	now := nowMillis()
	type scored struct {
		path  string
		score float64
	}
	var ranked []scored
	if q := a.prompt.acQuery(); q == "" {
		ranked = make([]scored, len(files))
		for i, f := range files {
			ranked[i] = scored{path: f, score: frecencyScore(a.freqFor(f), now)}
		}
	} else {
		for _, m := range fuzzy.Find(q, files) {
			s := float64(m.Score)
			if strings.HasPrefix(m.Str, q) {
				s *= 2
			}
			s *= 1 + frecencyScore(a.freqFor(m.Str), now)
			ranked = append(ranked, scored{path: m.Str, score: s})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > maxPickerOptions {
		ranked = ranked[:maxPickerOptions]
	}
	opts := make([]selectOption, 0, len(ranked))
	for _, r := range ranked {
		opts = append(opts, selectOption{value: r.path})
	}
	return opts
}

// acInsert replaces the @-query with the path text (plain text, no
// parts/chips — deviation-222 class), moves the cursor to the end, resets
// the recall + picker selection, and records the selection in the frecency.
func (a *App) acInsert(rel string) {
	v := a.prompt.input.Value()
	idx, ok := mentionTriggerIndex(v)
	if !ok {
		return
	}
	next := v[:idx] + rel
	a.prompt.input.SetValue(next)
	a.prompt.input.SetCursor(len([]rune(next)))
	a.histIdx = 0
	a.histText = ""
	a.prompt.sel = 0
	a.freq = updateFrecency(a.freq, rel, nowMillis())
	a.saveFrecency()
}
