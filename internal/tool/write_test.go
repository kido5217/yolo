package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func runTool(t *testing.T, id string, env *Env, args map[string]any) (Output, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	return Registry()[id].Run(context.Background(), raw, env)
}

func TestWriteCreatesAndReports(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	out, err := runTool(t, "write", env, map[string]any{
		"filePath": filepath.Join(d, "new.txt"), "content": "a\nb\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "Wrote file successfully." {
		t.Fatalf("text = %q", out.Text)
	}
	metaAdded, _ := out.Meta["added"].(int)
	if metaAdded != 2 {
		t.Fatalf("added = %v", out.Meta)
	}
	b, _ := os.ReadFile(filepath.Join(d, "new.txt"))
	if string(b) != "a\nb\n" {
		t.Fatal("content mismatch")
	}
}

func TestWriteMissingDirCreated(t *testing.T) {
	d := t.TempDir()
	env := &Env{Dir: d, Limits: Limits{2000, 50 * 1024}}
	_, err := runTool(t, "write", env, map[string]any{
		"filePath": filepath.Join(d, "a", "b", "c.txt"), "content": "x",
	})
	if err != nil {
		t.Fatal(err)
	}
}
