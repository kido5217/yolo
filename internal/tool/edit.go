package tool

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed desc/edit.txt
var editDesc string

type editTool struct{}

var _ Tool = editTool{}

func (editTool) ID() string         { return "edit" }
func (editTool) Permission() string { return "edit" }
func (editTool) Desc() string       { return editDesc }

func (editTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to modify",
			},
			"oldString": map[string]any{
				"type":        "string",
				"description": "The text to replace",
			},
			"newString": map[string]any{
				"type":        "string",
				"description": "The text to replace it with (must be different from oldString)",
			},
			"replaceAll": map[string]any{
				"type":        "boolean",
				"description": "Replace all occurrences of oldString (default false)",
			},
		},
		"required": []string{"filePath", "oldString", "newString"},
	}
}

func (editTool) Patterns(raw json.RawMessage) ([]string, []string, error) {
	fp, err := editFilePath(raw)
	if err != nil {
		return nil, nil, err
	}
	return []string{fp}, []string{"*"}, nil
}

func (editTool) External(raw json.RawMessage) ([]string, error) {
	fp, err := editFilePath(raw)
	if err != nil {
		return nil, err
	}
	return []string{fp}, nil
}

// editFilePath extracts just the filePath (upstream builds permission
// patterns from it alone); full arg validation happens in Run.
func editFilePath(raw json.RawMessage) (string, error) {
	m, err := argsMap(raw)
	if err != nil {
		return "", err
	}
	v, ok := m["filePath"].(string)
	if !ok || v == "" {
		return "", errors.New("filePath is required")
	}
	return v, nil
}

// editArgs parses {filePath, oldString, newString, replaceAll?}; oldString
// may be "" (upstream: create the file from newString when it does not
// exist).
func editArgs(raw json.RawMessage) (fp, oldText, newText string, replaceAll bool, err error) {
	if fp, err = editFilePath(raw); err != nil {
		return
	}
	m, uerr := argsMap(raw)
	if uerr != nil {
		err = uerr
		return
	}
	v, ok := m["oldString"].(string)
	if !ok {
		err = errors.New("oldString is required")
		return
	}
	oldText = v
	v, ok = m["newString"].(string)
	if !ok {
		err = errors.New("newString is required")
		return
	}
	newText = v
	replaceAll, _ = m["replaceAll"].(bool)
	return
}

// editLocks ports upstream edit.ts's per-resolved-path Semaphore(1): one
// mutex per file, kept in a sync.Map for the process lifetime.
var editLocks sync.Map

// fileLock locks (and returns) the mutex for fp. Caller defers Unlock.
func fileLock(fp string) *sync.Mutex {
	v, _ := editLocks.LoadOrStore(fp, &sync.Mutex{})
	m, ok := v.(*sync.Mutex)
	if !ok {
		// Invariant: the map only ever stores *sync.Mutex; keep one anyway
		// so a future second writer cannot turn this into a nil panic.
		m = &sync.Mutex{}
		editLocks.Store(fp, m)
	}
	m.Lock()
	return m
}

func (editTool) Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error) {
	_ = ctx
	fp, oldText, newText, replaceAll, err := editArgs(raw)
	if err != nil {
		return Output{}, err
	}
	if oldText == newText {
		return Output{}, errors.New("no changes to apply: oldString and newString are identical")
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
		env.Log.Info("edit", "path", fp)
	}
	l := fileLock(fp)
	defer l.Unlock()

	contentOld, contentNew, aerr := editApply(fp, oldText, newText, replaceAll)
	if aerr != nil {
		return Output{}, aerr
	}
	added, removed := diffCounts(contentOld, contentNew)
	return Output{
		Title: readTitle(env.Dir, fp),
		Text:  "Edit applied successfully.",
		Meta:  map[string]any{"added": added, "removed": removed},
	}, nil
}

// editApply runs the v1 exact-match replacer (upstream's exact
// MultiOccurrenceReplacer path, simplified per the plan): strings.Count
// uniqueness check, then single Replace or ReplaceAll.
func editApply(fp, oldText, newText string, replaceAll bool) (contentOld, contentNew string, err error) {
	if oldText == "" {
		if _, serr := os.Stat(fp); serr == nil {
			return "", "", errors.New(
				"oldString cannot be empty when editing an existing file. " +
					"Provide the exact text to replace, or use write for an " +
					"intentional full-file replacement")
		}
		if merr := os.MkdirAll(filepath.Dir(fp), 0o755); merr != nil {
			return "", "", merr
		}
		if werr := os.WriteFile(fp, []byte(newText), 0o644); werr != nil {
			return "", "", werr
		}
		return "", newText, nil
	}
	fi, serr := os.Stat(fp)
	if serr != nil {
		// The missing-file text is model-facing and test-pinned; other stat
		// failures (e.g. permission) keep their context via %w.
		if errors.Is(serr, os.ErrNotExist) {
			return "", "", fmt.Errorf("file %s not found", fp)
		}
		return "", "", fmt.Errorf("stat %s: %w", fp, serr)
	}
	if fi.IsDir() {
		return "", "", fmt.Errorf("path is a directory, not a file: %s", fp)
	}
	b, rerr := os.ReadFile(fp)
	if rerr != nil {
		return "", "", rerr
	}
	content := string(b)
	switch n := strings.Count(content, oldText); {
	case n == 0:
		return "", "", errors.New(
			"could not find oldString in the file. It must match exactly, " +
				"including whitespace, indentation, and line endings")
	case n > 1 && !replaceAll:
		return "", "", errors.New(
			"found multiple matches for oldString. Provide more surrounding " +
				"context to make the match unique")
	}
	if replaceAll {
		content = strings.ReplaceAll(content, oldText, newText)
	} else {
		content = strings.Replace(content, oldText, newText, 1)
	}
	if werr := os.WriteFile(fp, []byte(content), 0o644); werr != nil {
		return "", "", werr
	}
	return string(b), content, nil
}
