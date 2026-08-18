package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditExactReplace(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "f.txt")
	if err := os.WriteFile(f, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, err := runTool(t, "edit", env, map[string]any{
		"filePath": f, "oldString": "two", "newString": "TWO",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(f)
	if string(b) != "one\nTWO\nthree\n" {
		t.Fatalf("content = %q", b)
	}
	_ = out
}

func TestEditErrorsPinned(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "f.txt")
	if err := os.WriteFile(f, []byte("a\nb\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}

	_, err := runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "a", "newString": "a"})
	if err == nil || err.Error() != "no changes to apply: oldString and newString are identical" {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "", "newString": "a"})
	if err == nil || !strings.HasPrefix(err.Error(), "oldString cannot be empty") {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": filepath.Join(d, "nope.txt"), "oldString": "x", "newString": "y"})
	if err == nil || err.Error() != "file "+filepath.Join(d, "nope.txt")+" not found" {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": d, "oldString": "x", "newString": "y"})
	if err == nil || !strings.Contains(err.Error(), "path is a directory, not a file") {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "zzz", "newString": "y"})
	if err == nil || err.Error() != "could not find oldString in the file. It must match exactly, including whitespace, indentation, and line endings" {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "a", "newString": "b"})
	if err == nil || err.Error() != "found multiple matches for oldString. Provide more surrounding context to make the match unique" {
		t.Fatalf("err = %v", err)
	}
	_, err = runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "a", "newString": "b", "replaceAll": true})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(f)
	if string(b) != "b\nb\nb\n" {
		t.Fatalf("replaceAll content = %q", b)
	}
}

func TestEditPatternsAndExternal(t *testing.T) {
	// Plan deviation: the plan expected Patterns(raw) to return the
	// env.Dir-relative path ("sub/f.txt"), but the committed Task 11
	// interface takes raw args only — paths are emitted as given and the
	// engine (Task 17) resolves/relativizes against Env.Dir (same as
	// read). Assert as-given.
	d := t.TempDir()
	f := filepath.Join(d, "sub", "f.txt")
	raw, _ := json.Marshal(map[string]any{"filePath": f})
	res, always, err := Registry()["edit"].Patterns(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0] != f || len(always) != 1 || always[0] != "*" {
		t.Fatalf("patterns = %v %v", res, always)
	}
	ext, _ := Registry()["edit"].External(raw)
	if len(ext) != 1 || ext[0] != f {
		t.Fatalf("external = %v", ext)
	}
	if Registry()["edit"].Permission() != "edit" || Registry()["write"].Permission() != "edit" {
		t.Fatal("permission mapping")
	}
	// Visible(nil, Registry()): no wildcard-deny rules → all visible.
	if _, ok := Visible(nil, Registry())["write"]; !ok {
		t.Fatal("write visible without rules")
	}
}
