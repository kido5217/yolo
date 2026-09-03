package tool

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kido5217/yolo/internal/glob"
)

//go:embed desc/grep.txt
var grepDesc string

const (
	grepLimit     = 100
	grepMaxSize   = 10 * 1024 * 1024
	grepBinWindow = 8000
)

type grepMatch struct {
	Path string
	Line int
	Text string
}

type grepTool struct{}

var _ Tool = grepTool{}

func (grepTool) ID() string         { return "grep" }
func (grepTool) Permission() string { return "grep" }
func (grepTool) Desc() string       { return grepDesc }

func (grepTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The regex pattern to search for in file contents",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "The directory to search in. Defaults to the current working directory.",
			},
			"include": map[string]any{
				"type":        "string",
				"description": "File pattern to include in the search (e.g. \"*.js\", \"*.{ts,tsx}\")",
			},
		},
		"required": []string{"pattern"},
	}
}

func (grepTool) Patterns(raw json.RawMessage) ([]string, []string, error) {
	pattern, err := grepPattern(raw)
	if err != nil {
		return nil, nil, err
	}
	return []string{pattern}, []string{"*"}, nil
}

func (grepTool) External(raw json.RawMessage) ([]string, error) {
	p, err := grepArgPath(raw)
	if err != nil {
		return nil, err
	}
	if p != "" {
		return []string{p}, nil
	}
	return []string{"*"}, nil
}

func grepPattern(raw json.RawMessage) (string, error) {
	m, err := argsMap(raw)
	if err != nil {
		return "", err
	}
	v, ok := m["pattern"].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("pattern is required")
	}
	return v, nil
}

// grepArgPath extracts the optional path param (as given, Task 11
// interface: no env here).
func grepArgPath(raw json.RawMessage) (string, error) {
	m, err := argsMap(raw)
	if err != nil {
		return "", err
	}
	p, _ := m["path"].(string)
	return p, nil
}

func (grepTool) Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error) {
	_ = ctx
	if env == nil {
		env = &Env{}
	}
	pattern, err := grepPattern(raw)
	if err != nil {
		return Output{}, err
	}
	m, err := argsMap(raw)
	if err != nil {
		return Output{}, err
	}
	path, _ := m["path"].(string)
	include, _ := m["include"].(string)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Output{}, err
	}

	empty := Output{
		Title: pattern,
		Text:  "No files found",
		Meta:  map[string]any{"matches": 0, "truncated": false},
	}
	requested := path
	if requested == "" {
		requested = env.Dir
	}
	if requested == "" {
		requested = "."
	}
	if !filepath.IsAbs(requested) {
		requested = filepath.Join(env.Dir, requested)
	}
	info, serr := os.Stat(requested)
	if serr != nil {
		return Output{}, fmt.Errorf("grep: cannot access %s: %w", requested, serr)
	}
	searchDir := requested
	if !info.IsDir() {
		searchDir = filepath.Dir(requested)
	}
	if env.Log != nil {
		env.Log.Info("grep", "pattern", pattern, "path", searchDir)
	}
	matches, truncated, werr := grepWalk(searchDir, re, include)
	if werr != nil {
		return Output{}, werr
	}
	if env.Log != nil {
		env.Log.Info("grep results", "pattern", pattern, "matches", len(matches))
	}
	if len(matches) == 0 {
		return empty, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d matches", len(matches))
	if truncated {
		b.WriteString(" (more matches available)")
	}
	current := ""
	for _, g := range matches {
		if g.Path != current {
			b.WriteString("\n\n" + g.Path + ":")
			current = g.Path
		}
		fmt.Fprintf(&b, "\n  Line %d: %s", g.Line, g.Text)
	}
	if truncated {
		b.WriteString("\n\n(Results truncated. Consider using a more specific path or pattern.)")
	}
	return Output{
		Title: pattern,
		Text:  b.String(),
		Meta:  map[string]any{"matches": len(matches), "truncated": truncated},
	}, nil
}

// grepWalk walks searchDir depth-first, collecting up to grepLimit
// matches. Skips hidden entries (ripgrep --hidden default false), files
// over 10MB, and NUL-binary files (first 8000 bytes).
func grepWalk(searchDir string, re *regexp.Regexp, include string) (matches []grepMatch, truncated bool, werr error) {
	werr = filepath.WalkDir(searchDir, func(dpath string, d fs.DirEntry, err error) error {
		if len(matches) == grepLimit {
			truncated = true
			return fs.SkipAll
		}
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(searchDir, dpath)
		if rerr != nil || rel == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if include != "" && !glob.Match(include, filepath.ToSlash(rel)) {
			return nil
		}
		fi, ierr := d.Info()
		if ierr != nil || fi.Size() > grepMaxSize {
			return nil
		}
		b, rerr := os.ReadFile(dpath)
		if rerr != nil {
			return nil
		}
		if bytes.IndexByte(b[:min(len(b), grepBinWindow)], 0) >= 0 {
			return nil
		}
		// Walk lines with IndexByte instead of Split(string(b), "\n"): no
		// whole-file string copy and no per-line slice array; the segment
		// split (including the trailing empty one after a final newline)
		// matches strings.Split exactly, and only matched lines are kept.
		off, lineNo := 0, 1
		for {
			p := bytes.IndexByte(b[off:], '\n')
			end := len(b)
			if p >= 0 {
				end = off + p
			}
			if re.Match(b[off:end]) {
				matches = append(matches, grepMatch{Path: dpath, Line: lineNo, Text: string(b[off:end])})
				if len(matches) == grepLimit {
					truncated = true
					return fs.SkipAll
				}
			}
			if p < 0 {
				break
			}
			off = end + 1
			lineNo++
			if off > len(b) {
				break
			}
		}
		return nil
	})
	// Per-entry errors are swallowed above (skip unreadable entries); the
	// root-level error is load-bearing: surface it when nothing matched.
	if werr != nil && len(matches) == 0 {
		return nil, false, werr
	}
	return
}
