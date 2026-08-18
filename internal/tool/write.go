package tool

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// diffCounts is a minimal LCS line diff (upstream edit/write use the JS
// diff package's diffLines): added/removed = lines not shared by the two
// contents, split on "\n".
func diffCounts(old, new string) (added, removed int) {
	oa, na := strings.Split(old, "\n"), strings.Split(new, "\n")
	common := lcsLen(oa, na)
	return len(na) - common, len(oa) - common
}

// lcsLen is a rolling two-row LCS length over lines: O(len(a)*len(b))
// time, O(len(b)) memory.
func lcsLen(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				cur[j] = prev[j+1] + 1
			case prev[j] > cur[j+1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j+1]
			}
		}
		prev, cur = cur, prev
	}
	return prev[0]
}
