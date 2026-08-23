package tool

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/aymanbagabas/go-udiff"
)

//go:embed desc/write.txt
var writeDesc string

type writeTool struct{}

var _ Tool = writeTool{}

func (writeTool) ID() string         { return "write" }
func (writeTool) Permission() string { return "edit" }
func (writeTool) Desc() string       { return writeDesc }

func (writeTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to write (must be absolute, not relative)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		"required": []string{"filePath", "content"},
	}
}

func (writeTool) Patterns(raw json.RawMessage) ([]string, []string, error) {
	fp, _, err := writeArgs(raw)
	if err != nil {
		return nil, nil, err
	}
	return []string{fp}, []string{"*"}, nil
}

func (writeTool) External(raw json.RawMessage) ([]string, error) {
	fp, _, err := writeArgs(raw)
	if err != nil {
		return nil, err
	}
	return []string{fp}, nil
}

// writeArgs parses {filePath, content}; content may be "" (creates an empty
// file) but must be present.
func writeArgs(raw json.RawMessage) (fp, content string, err error) {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var m map[string]any
	if err = json.Unmarshal(raw, &m); err != nil {
		return
	}
	v, ok := m["filePath"].(string)
	if !ok || v == "" {
		err = errors.New("filePath is required")
		return
	}
	fp = v
	v, ok = m["content"].(string)
	if !ok {
		err = errors.New("content is required")
		return
	}
	content = v
	return
}

func (writeTool) Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error) {
	_ = ctx
	fp, content, err := writeArgs(raw)
	if err != nil {
		return Output{}, err
	}
	if env == nil {
		env = &Env{}
	}
	if !filepath.IsAbs(fp) {
		if env.Dir != "" {
			fp = filepath.Join(env.Dir, fp)
		} else if abs, aerr := filepath.Abs(fp); aerr == nil {
			fp = abs
		}
	}
	if env.Log != nil {
		env.Log.Info("write", "path", fp)
	}
	old := ""
	if b, rerr := os.ReadFile(fp); rerr == nil {
		old = string(b)
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		return Output{}, err
	}
	if err := os.WriteFile(fp, []byte(content), 0o644); err != nil {
		return Output{}, err
	}
	added, removed := diffCounts(old, content)
	return Output{
		Title: readTitle(env.Dir, fp),
		Text:  "Wrote file successfully.",
		Meta:  map[string]any{"added": added, "removed": removed},
	}, nil
}

// diffCounts is the line-based optimal diff (upstream edit/write use the JS
// diff package's diffLines): added/removed = the lines replaced, via
// go-udiff v0.4.1's Myers line diff (security-3: the O(n·m) DP blocked the
// engine for tens of seconds on a one-line edit of a 60k-line file).
func diffCounts(before, after string) (added, removed int) {
	for _, e := range udiff.Lines(before, after) {
		removed += countLines(before[e.Start:e.End])
		added += countLines(e.New)
	}
	return added, removed
}

// countLines counts the lines a line-boundary diff segment contributes:
// a leading newline is a terminator (not a line), "" is zero lines, and a
// trailing newline does not open an extra line.
func countLines(s string) int {
	s = strings.TrimPrefix(s, "\n")
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
