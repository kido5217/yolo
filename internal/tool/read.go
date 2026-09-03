package tool

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed desc/read.txt
var readDesc string

const (
	readMaxLineLen   = 2000
	binarySniffBytes = 8000 // v1 pin: NUL sniff window (upstream: 4096 sample + ext list + non-printable ratio)
)

type readTool struct{}

var _ Tool = readTool{}

func (readTool) ID() string         { return "read" }
func (readTool) Permission() string { return "read" }
func (readTool) Desc() string       { return readDesc }

func (readTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file or directory to read",
			},
			"offset": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "The line number to start reading from (1-indexed)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "The maximum number of lines to read (defaults to 2000)",
			},
		},
		"required": []string{"filePath"},
	}
}

func (readTool) Patterns(raw json.RawMessage) ([]string, []string, error) {
	fp, _, _, err := readArgs(raw)
	if err != nil {
		return nil, nil, err
	}
	return []string{fp}, []string{"*"}, nil
}

func (readTool) External(raw json.RawMessage) ([]string, error) {
	fp, _, _, err := readArgs(raw)
	if err != nil {
		return nil, err
	}
	return []string{fp}, nil
}

func readArgs(raw json.RawMessage) (fp string, offset, limit int, err error) {
	m, err := argsMap(raw)
	if err != nil {
		return
	}
	v, ok := m["filePath"].(string)
	if !ok || v == "" {
		err = errors.New("filePath is required")
		return
	}
	fp = v
	offset = 1
	if v, ok := m["offset"]; ok && v != nil {
		n, ok2 := argInt(v)
		if !ok2 {
			err = errors.New("offset must be a non-negative integer")
			return
		}
		offset = n
		if offset < 1 { // upstream: params.offset || 1
			offset = 1
		}
	}
	if v, ok := m["limit"]; ok && v != nil {
		n, ok2 := argInt(v)
		if !ok2 {
			err = errors.New("limit must be a non-negative integer")
			return
		}
		limit = n
	}
	return
}

func argInt(v any) (int, bool) {
	f, ok := v.(float64)
	if !ok || f < 0 || f != float64(int64(f)) {
		return 0, false
	}
	return int(f), true
}

func (readTool) Run(ctx context.Context, raw json.RawMessage, env *Env) (Output, error) {
	_ = ctx
	fp, offset, limit, err := readArgs(raw)
	if err != nil {
		return Output{}, err
	}
	if env == nil {
		env = &Env{}
	}
	lim := env.Limits.withDefaults()
	if limit == 0 {
		limit = lim.MaxLines
	}
	if !filepath.IsAbs(fp) {
		if env.Dir != "" {
			fp = filepath.Join(env.Dir, fp)
		} else {
			abs, aerr := filepath.Abs(fp)
			if aerr != nil {
				return Output{}, aerr
			}
			fp = abs
		}
	}
	if env.Log != nil {
		env.Log.Info("read", "path", fp)
	}
	title := readTitle(env.Dir, fp)

	fi, err := os.Stat(fp)
	if errors.Is(err, os.ErrNotExist) {
		return Output{}, notFoundWithSuggestions(fp)
	}
	if err != nil {
		return Output{}, err
	}
	if fi.IsDir() {
		return readDirListing(env.Dir, fp, offset, limit)
	}
	if isBinaryFile(fp) {
		return Output{}, fmt.Errorf("cannot read binary file: %s", fp)
	}

	rawLines, count, cut, more, err := readLines(fp, offset, limit, lim.MaxBytes)
	if err != nil {
		return Output{}, err
	}
	if count < offset && (count != 0 || offset != 1) {
		return Output{}, fmt.Errorf("offset %d is out of range for this file (%d lines)", offset, count)
	}

	last := offset + len(rawLines) - 1
	text, meta := renderFileBody(fp, rawLines, offset, last, count, cut, more, lim.MaxBytes)
	return Output{Title: title, Text: text, Meta: meta}, nil
}

// renderFileBody turns the read window into the model-visible file text
// (the <path>/<type>/<content> block incl. the window/cap trailer) and its
// meta map, in one place.
func renderFileBody(fp string, rawLines []string, offset, last, count int, cut, more bool, maxBytes int) (string, map[string]any) {
	var b strings.Builder
	fmt.Fprintf(&b, "<path>%s</path>\n<type>file</type>\n<content>\n", fp)
	for i, line := range rawLines {
		fmt.Fprintf(&b, "%d: %s", offset+i, line)
		if i+1 < len(rawLines) {
			b.WriteByte('\n')
		}
	}
	next := last + 1
	switch {
	case cut:
		fmt.Fprintf(&b, "\n\n(Output capped at %dKB. Showing lines %d-%d. Use offset=%d to continue.)", maxBytes/1024, offset, last, next)
	case more:
		fmt.Fprintf(&b, "\n\n(Showing lines %d-%d of %d. Use offset=%d to continue.)", offset, last, count, next)
	default:
		fmt.Fprintf(&b, "\n\n(End of file - total %d lines)", count)
	}
	b.WriteString("\n</content>")

	meta := displayMeta(rawLines, cut || more, map[string]any{
		"type":       "file",
		"path":       fp,
		"text":       strings.Join(rawLines, "\n"),
		"lineStart":  offset,
		"lineEnd":    last,
		"totalLines": count,
		"truncated":  cut || more,
	})
	return b.String(), meta
}

// displayMeta builds the shared meta map: the 20-line preview, the truncated
// flag, and the per-kind display map.
func displayMeta(previewLines []string, truncated bool, display map[string]any) map[string]any {
	preview := previewLines
	if len(preview) > 20 {
		preview = preview[:20]
	}
	return map[string]any{
		"preview":   strings.Join(preview, "\n"),
		"truncated": truncated,
		"display":   display,
	}
}

// readTitle mirrors upstream path.relative(worktree, filepath).
func readTitle(base, fp string) string {
	if base != "" {
		if rel, err := filepath.Rel(base, fp); err == nil && rel != "" && rel != "." {
			return rel
		}
	}
	return filepath.Base(fp)
}

// readLines ports upstream lines(): streams fp line by line starting at
// offset (1-indexed), taking at most limit lines. count is the number of
// lines seen before stopping (= total lines when not cut). The byte budget
// accounts rendered lines (after the 2000-char line cut) plus one join byte
// per separator. \r\n is handled like upstream splitLines.
func readLines(fp string, offset, limit, maxBytes int) ([]string, int, bool, bool, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, 0, false, false, err
	}
	defer f.Close()
	start := offset - 1
	// Preallocation hint only: limit is model-controlled and may be huge
	// (clamping limit itself would change line-count semantics).
	capHint := limit
	if capHint > 8192 {
		capHint = 8192
	}
	raw := make([]string, 0, capHint)
	count := 0
	bytes := 0
	var cut, more bool
	r := bufio.NewReader(f)
	for {
		line, rerr := r.ReadString('\n')
		if line != "" {
			line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			count++
			switch {
			case count <= start:
				// skip prefix, keep counting
			case len(raw) >= limit:
				more = true
			default:
				if len(line) > readMaxLineLen {
					line = line[:readMaxLineLen] + fmt.Sprintf("... (line truncated to %d chars)", readMaxLineLen)
				}
				size := len(line)
				if len(raw) > 0 {
					size++
				}
				if bytes+size <= maxBytes {
					raw = append(raw, line)
					bytes += size
				} else {
					cut = true
					more = true
				}
			}
			if cut {
				break
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				break
			}
			return nil, 0, false, false, rerr
		}
	}
	return raw, count, cut, more, nil
}

// readDirListing ports upstream list(): dirs get a "/" suffix, entries
// sorted case-insensitively (documented approximation of localeCompare).
func readDirListing(base, fp string, offset, limit int) (Output, error) {
	entries, err := os.ReadDir(fp)
	if err != nil {
		return Output{}, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			n += "/"
		}
		names = append(names, n)
	}
	sort.SliceStable(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	if offset < 1 {
		offset = 1
	}
	start := offset - 1
	sliced := []string{}
	if start < len(names) {
		end := start + limit
		// start < len(names) but start+limit may overflow int and go
		// negative; clamp to the tail instead of a bad slice bound.
		if end < 0 || end > len(names) {
			end = len(names)
		}
		sliced = names[start:end]
	}
	truncated := start+len(sliced) < len(names)
	out := []string{
		"<path>" + fp + "</path>",
		"<type>directory</type>",
		"<entries>",
		strings.Join(sliced, "\n"),
	}
	if truncated {
		out = append(out, fmt.Sprintf(
			"\n(Showing %d of %d entries. Use 'offset' parameter "+
				"to read beyond entry %d)",
			len(sliced), len(names), offset+len(sliced)))
	} else {
		out = append(out, fmt.Sprintf("\n(%d entries)", len(names)))
	}
	out = append(out, "</entries>")

	meta := displayMeta(sliced, truncated, map[string]any{
		"type":         "directory",
		"path":         fp,
		"entries":      sliced,
		"offset":       offset,
		"totalEntries": len(names),
		"truncated":    truncated,
	})
	return Output{Title: readTitle(base, fp), Text: strings.Join(out, "\n"), Meta: meta}, nil
}

// notFoundWithSuggestions ports upstream: up to 3 sibling entries whose name
// contains the missing basename (case-insensitive, either direction), else
// the plain not-found error.
func notFoundWithSuggestions(fp string) error {
	dir, base := filepath.Dir(fp), filepath.Base(fp)
	hits := []string{}
	if items, err := os.ReadDir(dir); err == nil {
		lowerName := strings.ToLower(base)
		for _, it := range items {
			name := strings.ToLower(it.Name())
			if strings.Contains(name, lowerName) || strings.Contains(lowerName, name) {
				hits = append(hits, filepath.Join(dir, it.Name()))
			}
			if len(hits) >= 3 {
				break
			}
		}
	}
	if len(hits) > 0 {
		return fmt.Errorf("file not found: %s\n\nDid you mean one of these?\n%s", fp, strings.Join(hits, "\n"))
	}
	return fmt.Errorf("file not found: %s", fp)
}

// isBinaryFile sniffs NUL bytes in the first binarySniffBytes. A file that
// cannot be read past sniff setup is not binary: the later readLines fails
// and surfaces the read error.
func isBinaryFile(fp string) bool {
	f, err := os.Open(fp)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, binarySniffBytes)
	n, rerr := io.ReadFull(f, buf)
	if rerr != nil && !errors.Is(rerr, io.EOF) && !errors.Is(rerr, io.ErrUnexpectedEOF) {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}
