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
// slash-relative paths (deviation 225). Files are recorded as walked;
// directories are recorded preorder (at visit, before descending) with a
// trailing "/" (the display marker — the fuzzy target and the insert value
// are the path WITHOUT it). The maxWalkFiles cap counts FILE entries only:
// dir rows do not consume the cap.
func walkFiles(root string) []string {
	patterns := gitignorePatterns(root)
	out := []string{}
	fileCount := 0
	// the walk callback swallows every per-path error (returns nil), so the
	// outer WalkDir error is structurally always nil.
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
			out = append(out, rel+"/")
			return nil
		}
		if walkIgnore[d.Name()] || ignoredByGitignore(patterns, rel) {
			return nil
		}
		out = append(out, rel)
		fileCount++
		if fileCount >= maxWalkFiles {
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

// walkedFiles is the cached @-picker walk of the scope dir (deviation 225);
// it re-walks only when the scope dir changes. The rows are the slash-
// relative paths — files, and directories with the trailing-"/" display
// marker (the walkFiles preorder).
func (a *App) walkedFiles() []string {
	if a.walkRoot != a.Dir {
		a.walkRoot = a.Dir
		a.walked = walkFiles(a.walkRoot)
	}
	return a.walked
}

// mentionOption is the @-picker's option value (spec §3.1): the path (the
// insert value — the slash-relative path, no trailing "/") + the directory
// flag (the trailing-"/" row marker + the tab-expand branch, S3).
type mentionOption struct {
	path  string
	isDir bool
}

// mentionOptions builds the @-picker rows: fuzzy.Find over the merged
// file+directory pool (the walked entries — the dir rows' trailing-"/" marker
// stripped for the fuzzy target, the frecency key and the insert value) by
// the @-query, each POSITIVE-score match (the threshold-0.5 port, spec §3.4)
// x2 for a prefix match x (1 + frecencyScore) (the ported upstream scoreFn),
// sorted desc, capped at maxPickerOptions. An empty query lists all walked
// files + directories, frecency-ranked (walk order where frecency is zero).
// Each option's value is the mentionOption carrier (deviation-222 class —
// the plain-text insert is reworked to the @-prefixed form in S3).
func (a *App) mentionOptions() []selectOption {
	if !a.prompt.mentionActive() {
		return nil
	}
	entries := a.walkedFiles()
	if len(entries) == 0 {
		return nil
	}
	now := nowMillis()
	// Per-call frecency index: O(1) lookup instead of a linear scan per
	// entry (mentionOptions was O(entries x frecency)).
	idx := make(map[string]*frecencyEntry, len(a.freq))
	for i := range a.freq {
		idx[a.freq[i].Path] = &a.freq[i]
	}
	// The pool: the merged file+directory entries (the dir rows carry the
	// trailing-"/" display marker; the path is WITHOUT it).
	pool := make([]mentionOption, len(entries))
	for i, e := range entries {
		isDir := strings.HasSuffix(e, "/")
		pool[i] = mentionOption{path: strings.TrimSuffix(e, "/"), isDir: isDir}
	}
	type scored struct {
		opt   mentionOption
		score float64
	}
	var ranked []scored
	if q := a.prompt.acQuery(); q == "" {
		ranked = make([]scored, len(pool))
		for i, e := range pool {
			ranked[i] = scored{opt: e, score: frecencyScore(idx[e.path], now)}
		}
	} else {
		targets := make([]string, len(pool))
		isDir := make(map[string]bool, len(pool))
		for i, e := range pool {
			targets[i] = e.path
			isDir[e.path] = e.isDir
		}
		for _, m := range fuzzy.Find(q, targets) {
			if m.Score <= 0 {
				continue // the positive-score gate (the threshold-0.5 port)
			}
			s := float64(m.Score)
			if strings.HasPrefix(m.Str, q) {
				s *= 2
			}
			s *= 1 + frecencyScore(idx[m.Str], now)
			ranked = append(ranked, scored{opt: mentionOption{path: m.Str, isDir: isDir[m.Str]}, score: s})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > maxPickerOptions {
		ranked = ranked[:maxPickerOptions]
	}
	opts := make([]selectOption, 0, len(ranked))
	for _, r := range ranked {
		opts = append(opts, selectOption{value: r.opt})
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
