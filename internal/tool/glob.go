package tool

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kido5217/yolo/internal/glob"
)

//go:embed desc/glob.txt
var globDesc string

const globLimit = 100

type globTool struct{}

var _ Tool = globTool{}

func (globTool) ID() string         { return "glob" }
func (globTool) Permission() string { return "glob" }
func (globTool) Desc() string       { return globDesc }

func (globTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The glob pattern to match files against",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "The directory to search in. If not specified, the current working directory will be used. IMPORTANT: Omit this field to use the default directory. DO NOT enter \"undefined\" or \"null\" - simply omit it for the default behavior. Must be a valid directory path if provided.",
			},
		},
		"required": []string{"pattern"},
	}
}

func (globTool) Patterns(raw json.RawMessage) ([]string, []string, error) {
	pattern, err := globPattern(raw)
	if err != nil {
		return nil, nil, err
	}
	return []string{pattern}, []string{"*"}, nil
}

func (globTool) External(raw json.RawMessage) ([]string, error) {
	_, p := globPathArg(raw)
	if p != "" {
		return []string{p}, nil
	}
	return []string{"*"}, nil
}

func globPattern(raw json.RawMessage) (string, error) {
	var m map[string]any
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	v, ok := m["pattern"].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("pattern is required")
	}
	return v, nil
}

func globPathArg(raw json.RawMessage) (pattern, path string) {
	var m map[string]any
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	pattern, _ = m["pattern"].(string)
	path, _ = m["path"].(string)
	return
}

func (globTool) Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error) {
	_ = ctx
	if env == nil {
		env = &Env{}
	}
	pattern, p := globPathArg(raw)
	if pattern == "" {
		return Output{}, fmt.Errorf("pattern is required")
	}
	search := p
	if search == "" {
		search = env.Dir
	}
	if search == "" {
		search = "."
	}
	if !filepath.IsAbs(search) {
		search = filepath.Join(env.Dir, search)
	}
	fi, serr := os.Stat(search)
	if serr != nil || !fi.IsDir() {
		return Output{}, fmt.Errorf("glob path must be a directory: %s", search)
	}
	if env.Log != nil {
		env.Log.Info("glob", "pattern", pattern, "path", search)
	}

	var files []string
	truncated := false
	werr := filepath.WalkDir(search, func(dpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(search, dpath)
		if rerr != nil {
			return nil
		}
		if rel == "." {
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
		if glob.Match(pattern, filepath.ToSlash(rel)) {
			if len(files) < globLimit {
				files = append(files, dpath)
				return nil
			}
			// ⑫: more than the cap were observed — stop the whole walk
			// (upstream ripgrep.ts takes limit+1 from the stream).
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if werr != nil {
		return Output{}, werr
	}
	if env.Log != nil {
		env.Log.Info("glob results", "pattern", pattern, "count", len(files))
	}

	text := "No files found"
	if len(files) > 0 {
		text = strings.Join(files, "\n")
		if truncated {
			text += "\n\n(Results are truncated: showing first " +
				fmt.Sprint(globLimit) +
				" results. Consider using a more specific path or pattern.)"
		}
	}
	rel, rerr := filepath.Rel(env.Dir, search)
	title := rel
	if rerr != nil {
		title = search
	}
	return Output{
		Title: title,
		Text:  text,
		Meta:  map[string]any{"count": len(files), "truncated": truncated},
	}, nil
}
