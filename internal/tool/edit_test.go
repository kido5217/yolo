package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	cases := []struct {
		name string
		rel  string // path suffix under the per-case dir; "" = f.txt
		old  string
		new  string
		want string
	}{
		{name: "identical strings", old: "a", new: "a",
			want: "no changes to apply: oldString and newString are identical"},
		{name: "empty oldString", old: "", new: "a",
			want: "oldString cannot be empty when editing an existing file. Provide the exact text to replace, or use write for an intentional full-file replacement"},
		{name: "missing file", rel: "nope.txt", old: "x", new: "y", want: ""}, // "file <fp> not found", fp filled in below
		{name: "path is a directory", rel: ".", old: "x", new: "y", want: ""}, // "path is a directory, not a file: <d>", d filled in below
		{name: "oldString not found", old: "zzz", new: "y",
			want: "could not find oldString in the file. It must match exactly, including whitespace, indentation, and line endings"},
		{name: "multiple matches", old: "a", new: "b",
			want: "found multiple matches for oldString. Provide more surrounding context to make the match unique"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			f := filepath.Join(d, "f.txt")
			if err := os.WriteFile(f, []byte("a\nb\na\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			fp := f
			want := tc.want
			if tc.rel != "" {
				fp = filepath.Join(d, tc.rel)
			}
			switch tc.name {
			case "missing file":
				want = "file " + fp + " not found"
			case "path is a directory":
				want = "path is a directory, not a file: " + fp
			}
			env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
			_, err := runTool(t, "edit", env, map[string]any{"filePath": fp, "oldString": tc.old, "newString": tc.new})
			if err == nil {
				t.Fatal("want error")
			}
			if err.Error() != want {
				t.Fatalf("err = %q, want %q", err, want)
			}
		})
	}
}

func TestEditreplaceAll(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "f.txt")
	if err := os.WriteFile(f, []byte("a\nb\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	_, err := runTool(t, "edit", env, map[string]any{"filePath": f, "oldString": "a", "newString": "b", "replaceAll": true})
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
